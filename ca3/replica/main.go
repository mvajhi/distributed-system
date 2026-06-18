package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	ID      string   `json:"id"`
	Address string   `json:"address"`
	Peers   []string `json:"peers"`
}

type DataItem struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
}

type PutRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Consistency string `json:"consistency"`
}

var (
	config     Config
	dataStore  = make(map[string]DataItem)
	storeMutex sync.RWMutex
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func loadConfig(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening config file: %v\n", err)
	}
	defer file.Close()
	json.NewDecoder(file).Decode(&config)
}

func sendToPeer(peerURL string, item DataItem) {
	// شبیه‌سازی تاخیر شبکه برای سناریوها
	delayStr := os.Getenv("NETWORK_DELAY_MS")
	if delayStr != "" {
		if ms, err := strconv.Atoi(delayStr); err == nil {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	}

	jsonData, _ := json.Marshal(item)
	req, _ := http.NewRequest("POST", peerURL+"/internal/replicate", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[%s] Network Error: Failed to reach %s\n", config.ID, peerURL)
		return
	}
	defer resp.Body.Close()
}

func replicateAsync(item DataItem) {
	for _, peer := range config.Peers {
		go sendToPeer(peer, item)
	}
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	var req PutRequest
	json.NewDecoder(r.Body).Decode(&req)

	storeMutex.Lock()
	existing, exists := dataStore[req.Key]
	newVersion := 1
	if exists {
		newVersion = existing.Version + 1
	}

	newItem := DataItem{
		Key:       req.Key,
		Value:     req.Value,
		Version:   newVersion,
		UpdatedBy: config.ID,
	}
	storeMutex.Unlock()

	if req.Consistency == "eventual" {
		storeMutex.Lock()
		dataStore[req.Key] = newItem
		storeMutex.Unlock()

		go replicateAsync(newItem)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Write successful on %s (Eventual)\n", config.ID)
		return
	}

	if req.Consistency == "strong" {
		http.Error(w, "Strong consistency not implemented by Person 1", http.StatusNotImplemented)
		return
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/get/")
	storeMutex.RLock()
	item, exists := dataStore[key]
	storeMutex.RUnlock()

	if !exists {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func handleInternalReplicate(w http.ResponseWriter, r *http.Request) {
	var incoming DataItem
	json.NewDecoder(r.Body).Decode(&incoming)

	storeMutex.Lock()
	defer storeMutex.Unlock()

	existing, exists := dataStore[incoming.Key]
	if !exists {
		dataStore[incoming.Key] = incoming
	} else {
		if incoming.Version >= existing.Version {
			dataStore[incoming.Key] = incoming
		}
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <config_file.json>\n", os.Args[0])
	}
	loadConfig(os.Args[1])

	http.HandleFunc("/put", handlePut)
	http.HandleFunc("/get/", handleGet)
	http.HandleFunc("/internal/replicate", handleInternalReplicate)

	log.Printf("Starting %s on %s...\n", config.ID, config.Address)
	log.Fatal(http.ListenAndServe(config.Address, nil))
}
