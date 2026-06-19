package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	replicaID  string
	port       int
	mode       string
	delay      int
	peersRaw   string
	configPath string
	peers      map[string]string // map[replicaID]url
	kvStore    *Store
)

func logEvent(action, key string, version int, details string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Printf("[%s] [%s] %s key=%s version=%d %s\n", timestamp, replicaID, action, key, version, details)
}

type ConfigFile struct {
	ID       string            `json:"id"`
	Port     int               `json:"port"`
	Mode     string            `json:"mode"`
	Delay    int               `json:"delay"`
	Replicas map[string]string `json:"replicas"`
}

func main() {
	flag.StringVar(&replicaID, "id", "", "Replica ID (e.g. replica1)")
	flag.IntVar(&port, "port", 8001, "Port to listen on")
	flag.StringVar(&mode, "mode", "eventual", "Consistency mode (eventual or strong)")
	flag.IntVar(&delay, "delay", 0, "Artificial delay in milliseconds for outgoing requests")
	flag.StringVar(&peersRaw, "replicas", "", "Comma-separated list of peer replicas (id=url)")
	flag.StringVar(&configPath, "config", "", "Path to JSON configuration file")
	flag.Parse()

	// Detect which flags were explicitly set via command line
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	peers = make(map[string]string)

	if configPath != "" {
		cfgFile, err := os.Open(configPath)
		if err != nil {
			fmt.Printf("Error opening config file: %v\n", err)
			os.Exit(1)
		}
		defer cfgFile.Close()

		var cfg ConfigFile
		if err := json.NewDecoder(cfgFile).Decode(&cfg); err != nil {
			fmt.Printf("Error decoding config file: %v\n", err)
			os.Exit(1)
		}

		// Apply config file values if they weren't explicitly set in command line
		if !setFlags["id"] {
			replicaID = cfg.ID
		}
		if !setFlags["port"] {
			port = cfg.Port
		}
		if !setFlags["mode"] {
			mode = cfg.Mode
		}
		if !setFlags["delay"] {
			delay = cfg.Delay
		}
		if !setFlags["replicas"] {
			for id, url := range cfg.Replicas {
				if id != replicaID {
					peers[id] = url
				}
			}
		}
	}

	// Parse command-line replicas if explicitly provided
	if setFlags["replicas"] {
		peers = parsePeers(peersRaw, replicaID)
	}

	if replicaID == "" {
		fmt.Println("Error: replica --id is required (either via flag or config file)")
		os.Exit(1)
	}

	kvStore = NewStore()

	mux := http.NewServeMux()

	// GET /kv/{key}
	mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		item, exists := kvStore.Get(key)
		if !exists {
			logEvent("GET_CLIENT", key, 0, "status=404_not_found")
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}

		logEvent("GET_CLIENT", key, item.Version, fmt.Sprintf("status=200_ok val=%s updated_by=%s", item.Value, item.UpdatedBy))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	})

	// PUT /kv/{key}
	mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		var req struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		logEvent("PUT_CLIENT", key, 0, fmt.Sprintf("received value=%s", req.Value))

		// Atomically update local store
		item := kvStore.IncrementAndPut(key, req.Value, replicaID)
		logEvent("PUT_LOCAL", key, item.Version, fmt.Sprintf("updated val=%s", item.Value))

		if len(peers) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(item)
			return
		}

		if mode == "strong" {
			// Strong consistency: wait for quorum (coordinator + 1 other for N=3)
			successChan := make(chan bool, len(peers))
			for peerID, peerURL := range peers {
				go func(id, url string) {
					success := replicateToPeer(key, item, url, id)
					successChan <- success
				}(peerID, peerURL)
			}

			totalReplicas := len(peers) + 1
			quorum := totalReplicas/2 + 1
			neededPeers := quorum - 1

			successCount := 0
			failCount := 0
			quorumReached := false

			for i := 0; i < len(peers); i++ {
				success := <-successChan
				if success {
					successCount++
					if successCount >= neededPeers {
						quorumReached = true
					}
				} else {
					failCount++
				}
			}

			if quorumReached {
				logEvent("PUT_CLIENT", key, item.Version, fmt.Sprintf("success strong_quorum_reached successes=%d", successCount))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(item)
			} else {
				logEvent("PUT_CLIENT", key, item.Version, fmt.Sprintf("error strong_quorum_failed successes=%d fails=%d", successCount, failCount))
				http.Error(w, "Quorum not reached", http.StatusServiceUnavailable)
			}

		} else {
			// Eventual consistency: replicate asynchronously
			for peerID, peerURL := range peers {
				go func(id, url string) {
					replicateToPeer(key, item, url, id)
				}(peerID, peerURL)
			}
			logEvent("PUT_CLIENT", key, item.Version, "success eventual_mode_immediate_response")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(item)
		}
	})

	// POST /replicate/{key}
	mux.HandleFunc("POST /replicate/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		var incoming Item
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		logEvent("REPLICATE_RECV", key, incoming.Version, fmt.Sprintf("from_replica=%s incoming_val=%s", incoming.UpdatedBy, incoming.Value))

		updated, current := kvStore.Put(key, incoming)
		if updated {
			logEvent("REPLICATE_ACCEPT", key, incoming.Version, fmt.Sprintf("val=%s updated_by=%s", incoming.Value, incoming.UpdatedBy))
			w.WriteHeader(http.StatusOK)
		} else {
			logEvent("REPLICATE_REJECT", key, incoming.Version, fmt.Sprintf("current_version=%d current_updated_by=%s", current.Version, current.UpdatedBy))
			w.WriteHeader(http.StatusConflict)
		}
	})

	// Status endpoint for testing
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    replicaID,
			"mode":  mode,
			"delay": delay,
		})
	})

	addr := fmt.Sprintf(":%d", port)
	logEvent("START", "", 0, fmt.Sprintf("Listening on port %d, mode=%s, delay=%dms", port, mode, delay))
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func parsePeers(peersRaw, selfID string) map[string]string {
	peers := make(map[string]string)
	if peersRaw == "" {
		return peers
	}
	parts := strings.Split(peersRaw, ",")
	for _, p := range parts {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			id := strings.TrimSpace(kv[0])
			url := strings.TrimSpace(kv[1])
			if id != selfID {
				peers[id] = url
			}
		}
	}
	return peers
}

func replicateToPeer(key string, item Item, peerURL string, peerID string) bool {
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	url := fmt.Sprintf("%s/replicate/%s", peerURL, key)
	bodyBytes, err := json.Marshal(item)
	if err != nil {
		logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s error=marshal_failed:_%v", peerID, err))
		return false
	}

	logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s url=%s", peerID, url))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s error=request_creation_failed:_%v", peerID, err))
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s error=send_failed:_%v", peerID, err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s success status=200", peerID))
		return true
	} else if resp.StatusCode == http.StatusConflict {
		logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s status=409_conflict_rejected", peerID))
		return true
	}

	logEvent("REPLICATE_SEND", key, item.Version, fmt.Sprintf("to=%s error=failed_with_status_%d", peerID, resp.StatusCode))
	return false
}
