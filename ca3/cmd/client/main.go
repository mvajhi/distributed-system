package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/mvajhi/dist/ca3/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── Metrics ──────────────────────────────────────────────────────────────

type ScenarioMetrics struct {
	Scenario        string  `json:"scenario"`
	Consistency     string  `json:"consistency_model"`
	PutLatencyMs    float64 `json:"put_latency_ms"`
	GetLatencyMs    float64 `json:"get_latency_ms"`
	ConvergenceMs   float64 `json:"convergence_time_ms"`
	StaleReads      int     `json:"stale_reads_count"`
	Success         bool    `json:"success"`
	Notes           string  `json:"notes,omitempty"`
}

// ─── Helper functions ─────────────────────────────────────────────────────

func connect(addr string) (pb.KVStoreClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	return pb.NewKVStoreClient(conn), conn, nil
}

func connectAll(addrs []string) ([]pb.KVStoreClient, []*grpc.ClientConn, error) {
	var clients []pb.KVStoreClient
	var conns []*grpc.ClientConn
	for _, addr := range addrs {
		c, conn, err := connect(addr)
		if err != nil {
			// Close already opened connections.
			for _, cn := range conns {
				cn.Close()
			}
			return nil, nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
		}
		clients = append(clients, c)
		conns = append(conns, conn)
	}
	return clients, conns, nil
}

func doPut(client pb.KVStoreClient, key, value string, consistency pb.ConsistencyModel) (*pb.PutResponse, time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.Put(ctx, &pb.PutRequest{
		Key:         key,
		Value:       value,
		Consistency: consistency,
	})
	elapsed := time.Since(start)
	return resp, elapsed, err
}

func doGet(client pb.KVStoreClient, key string, consistency pb.ConsistencyModel) (*pb.GetResponse, time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.Get(ctx, &pb.GetRequest{
		Key:         key,
		Consistency: consistency,
	})
	elapsed := time.Since(start)
	return resp, elapsed, err
}

func waitForHealth(client pb.KVStoreClient, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		resp, err := client.HealthCheck(ctx, &pb.Empty{})
		cancel()
		if err == nil && resp.Alive {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("replica at %s not healthy after %v", addr, timeout)
}

func setDelay(client pb.KVStoreClient, delayMs int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.SetDelay(ctx, &pb.SetDelayRequest{DelayMs: delayMs})
	return err
}

func shutdownReplica(client pb.KVStoreClient, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Shutdown(ctx, &pb.ShutdownRequest{Reason: reason})
	return err
}

// ─── Scenarios ────────────────────────────────────────────────────────────

// Scenario 1: Temporary Inconsistency (Eventual Consistency)
func runScenario1(clients []pb.KVStoreClient, addrs []string) ScenarioMetrics {
	fmt.Println("\n=== Scenario 1: Temporary Inconsistency (Eventual) ===")

	key := "s1_test_key"
	value := "s1_value_v1"

	// PUT to replica-1 with eventual consistency.
	putResp, putLatency, err := doPut(clients[0], key, value, pb.ConsistencyModel_EVENTUAL)
	if err != nil {
		fmt.Printf("  PUT failed: %v\n", err)
		return ScenarioMetrics{Scenario: "1 (Temp Inconsistency)", Consistency: "Eventual", Success: false, Notes: err.Error()}
	}
	fmt.Printf("  PUT to %s: key=%s value=%s version=%d latency=%.2fms\n",
		addrs[0], key, value, putResp.Version, float64(putLatency.Microseconds())/1000.0)

	// Immediately GET from other replicas to count stale reads.
	staleReads := 0
	var getLatencyTotal time.Duration
	var getCount int

	for i := 1; i < len(clients); i++ {
		resp, getLatency, err := doGet(clients[i], key, pb.ConsistencyModel_EVENTUAL)
		getLatencyTotal += getLatency
		getCount++
		if err != nil {
			fmt.Printf("  GET from %s: error=%v\n", addrs[i], err)
			staleReads++
			continue
		}
		if !resp.Found || resp.Value != value {
			staleReads++
			fmt.Printf("  GET from %s: STALE (found=%v, value=%q)\n", addrs[i], resp.Found, resp.Value)
		} else {
			fmt.Printf("  GET from %s: FRESH (value=%q, version=%d)\n", addrs[i], resp.Value, resp.Version)
		}
	}

	// Poll until all replicas converge.
	convergenceStart := time.Now()
	converged := false
	for time.Since(convergenceStart) < 10*time.Second {
		allMatch := true
		for i := 1; i < len(clients); i++ {
			resp, getLatency, err := doGet(clients[i], key, pb.ConsistencyModel_EVENTUAL)
			getLatencyTotal += getLatency
			getCount++
			if err != nil || !resp.Found || resp.Value != value {
				allMatch = false
				if err == nil && (!resp.Found || resp.Value != value) {
					staleReads++
				}
				break
			}
		}
		if allMatch {
			converged = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	convergenceTime := time.Since(convergenceStart)
	avgGetLatency := float64(getLatencyTotal.Microseconds()) / float64(getCount) / 1000.0

	if converged {
		fmt.Printf("  CONVERGED in %.2fms, stale_reads=%d\n", float64(convergenceTime.Microseconds())/1000.0, staleReads)
	} else {
		fmt.Printf("  FAILED TO CONVERGE within 10s, stale_reads=%d\n", staleReads)
	}

	return ScenarioMetrics{
		Scenario:      "1 (Temp Inconsistency)",
		Consistency:   "Eventual",
		PutLatencyMs:  float64(putLatency.Microseconds()) / 1000.0,
		GetLatencyMs:  avgGetLatency,
		ConvergenceMs: float64(convergenceTime.Microseconds()) / 1000.0,
		StaleReads:    staleReads,
		Success:       converged,
	}
}

// Scenario 2: Replica Failure (Strong Consistency)
func runScenario2(clients []pb.KVStoreClient, addrs []string) ScenarioMetrics {
	fmt.Println("\n=== Scenario 2: Replica Failure (Strong) ===")

	key := "s2_test_key"
	notes := ""

	// Step 1: PUT with all 3 replicas alive.
	fmt.Println("  Step 1: PUT with all 3 replicas alive (strong)")
	putResp, putLatency, err := doPut(clients[0], key, "v1", pb.ConsistencyModel_STRONG)
	if err != nil {
		fmt.Printf("  PUT failed: %v\n", err)
		return ScenarioMetrics{Scenario: "2 (Replica Failure)", Consistency: "Strong", Success: false, Notes: err.Error()}
	}
	fmt.Printf("  PUT success: version=%d latency=%.2fms\n", putResp.Version, float64(putLatency.Microseconds())/1000.0)

	// Verify via GET.
	getResp, getLatency, err := doGet(clients[0], key, pb.ConsistencyModel_STRONG)
	if err != nil {
		fmt.Printf("  GET failed: %v\n", err)
	} else {
		fmt.Printf("  GET: value=%q version=%d latency=%.2fms\n", getResp.Value, getResp.Version, float64(getLatency.Microseconds())/1000.0)
	}

	// Step 2: Kill replica-3.
	fmt.Println("  Step 2: Shutting down replica-3")
	if err := shutdownReplica(clients[2], "scenario2_test"); err != nil {
		fmt.Printf("  Warning: shutdown signal to %s failed: %v\n", addrs[2], err)
	}
	time.Sleep(500 * time.Millisecond)

	// Step 3: PUT with 2 replicas alive (should succeed — quorum=2).
	fmt.Println("  Step 3: PUT with 2 replicas alive (strong)")
	putResp2, putLatency2, err := doPut(clients[0], key, "v2", pb.ConsistencyModel_STRONG)
	if err != nil {
		fmt.Printf("  PUT failed: %v\n", err)
		notes += "PUT with 2 replicas failed unexpectedly. "
	} else if !putResp2.Success {
		fmt.Printf("  PUT returned failure: %s\n", putResp2.Error)
		notes += "PUT with 2 replicas returned failure. "
	} else {
		fmt.Printf("  PUT success: version=%d latency=%.2fms\n", putResp2.Version, float64(putLatency2.Microseconds())/1000.0)
	}

	// Step 4: Kill replica-2.
	fmt.Println("  Step 4: Shutting down replica-2")
	if err := shutdownReplica(clients[1], "scenario2_test"); err != nil {
		fmt.Printf("  Warning: shutdown signal to %s failed: %v\n", addrs[1], err)
	}
	time.Sleep(500 * time.Millisecond)

	// Step 5: PUT with only 1 replica alive (should FAIL — no quorum).
	fmt.Println("  Step 5: PUT with 1 replica alive (strong) — expect failure")
	putResp3, putLatency3, err := doPut(clients[0], key, "v3", pb.ConsistencyModel_STRONG)
	if err != nil {
		fmt.Printf("  PUT error (expected): %v, latency=%.2fms\n", err, float64(putLatency3.Microseconds())/1000.0)
		notes += "PUT correctly failed with 1 replica. "
	} else if !putResp3.Success {
		fmt.Printf("  PUT correctly returned failure: %s, latency=%.2fms\n", putResp3.Error, float64(putLatency3.Microseconds())/1000.0)
		notes += "PUT correctly returned failure with 1 replica. "
	} else {
		fmt.Printf("  WARNING: PUT succeeded with only 1 replica! latency=%.2fms\n", float64(putLatency3.Microseconds())/1000.0)
		notes += "WARNING: PUT should have failed with only 1 replica. "
	}

	return ScenarioMetrics{
		Scenario:      "2 (Replica Failure)",
		Consistency:   "Strong",
		PutLatencyMs:  float64(putLatency.Microseconds()) / 1000.0,
		GetLatencyMs:  float64(getLatency.Microseconds()) / 1000.0,
		ConvergenceMs: 0,
		StaleReads:    0,
		Success:       true,
		Notes:         notes,
	}
}

// Scenario 3: Concurrent Conflict (Eventual Consistency)
func runScenario3(clients []pb.KVStoreClient, addrs []string) ScenarioMetrics {
	fmt.Println("\n=== Scenario 3: Concurrent Conflict (Eventual) ===")

	key := "s3_conflict_key"

	// Simultaneously PUT different values to replica-1 and replica-2.
	var wg sync.WaitGroup
	type putResult struct {
		idx     int
		value   string
		resp    *pb.PutResponse
		latency time.Duration
		err     error
	}
	results := make([]putResult, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		resp, lat, err := doPut(clients[0], key, "value_A", pb.ConsistencyModel_EVENTUAL)
		results[0] = putResult{0, "value_A", resp, lat, err}
	}()
	go func() {
		defer wg.Done()
		resp, lat, err := doPut(clients[1], key, "value_B", pb.ConsistencyModel_EVENTUAL)
		results[1] = putResult{1, "value_B", resp, lat, err}
	}()
	wg.Wait()

	avgPutLatency := 0.0
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("  PUT to %s failed: %v\n", addrs[r.idx], r.err)
		} else {
			fmt.Printf("  PUT to %s: value=%q version=%d latency=%.2fms\n",
				addrs[r.idx], r.value, r.resp.Version, float64(r.latency.Microseconds())/1000.0)
			avgPutLatency += float64(r.latency.Microseconds()) / 1000.0
		}
	}
	avgPutLatency /= 2.0

	// Wait for convergence and see which value wins.
	fmt.Println("  Waiting for convergence...")
	convergenceStart := time.Now()
	staleReads := 0
	converged := false
	var winnerValue string

	for time.Since(convergenceStart) < 10*time.Second {
		values := make(map[string]int)
		allSame := true
		var lastVal string
		for i, c := range clients {
			resp, _, err := doGet(c, key, pb.ConsistencyModel_EVENTUAL)
			if err != nil || !resp.Found {
				allSame = false
				staleReads++
				continue
			}
			values[resp.Value]++
			if lastVal == "" {
				lastVal = resp.Value
			} else if resp.Value != lastVal {
				allSame = false
				staleReads++
			}
			_ = i
		}
		if allSame && lastVal != "" {
			converged = true
			winnerValue = lastVal
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	convergenceTime := time.Since(convergenceStart)
	if converged {
		fmt.Printf("  CONVERGED in %.2fms, winner=%q, stale_reads=%d\n",
			float64(convergenceTime.Microseconds())/1000.0, winnerValue, staleReads)
	} else {
		fmt.Printf("  FAILED TO CONVERGE within 10s, stale_reads=%d\n", staleReads)
	}

	return ScenarioMetrics{
		Scenario:      "3 (Concurrent Conflict)",
		Consistency:   "Eventual",
		PutLatencyMs:  avgPutLatency,
		GetLatencyMs:  0,
		ConvergenceMs: float64(convergenceTime.Microseconds()) / 1000.0,
		StaleReads:    staleReads,
		Success:       converged,
		Notes:         fmt.Sprintf("winner=%s", winnerValue),
	}
}

// Scenario 4: Network Delay (Eventual Consistency)
func runScenario4(clients []pb.KVStoreClient, addrs []string) ScenarioMetrics {
	fmt.Println("\n=== Scenario 4: Network Delay 500ms (Eventual) ===")

	// Set 500ms delay on all replicas.
	for i, c := range clients {
		if err := setDelay(c, 500); err != nil {
			fmt.Printf("  Warning: failed to set delay on %s: %v\n", addrs[i], err)
		} else {
			fmt.Printf("  Set 500ms delay on %s\n", addrs[i])
		}
	}

	key := "s4_delayed_key"
	value := "s4_value_v1"

	// PUT to replica-1.
	putResp, putLatency, err := doPut(clients[0], key, value, pb.ConsistencyModel_EVENTUAL)
	if err != nil {
		fmt.Printf("  PUT failed: %v\n", err)
		return ScenarioMetrics{Scenario: "4 (Network Delay 500ms)", Consistency: "Eventual", Success: false, Notes: err.Error()}
	}
	fmt.Printf("  PUT to %s: version=%d latency=%.2fms\n",
		addrs[0], putResp.Version, float64(putLatency.Microseconds())/1000.0)

	// Poll for convergence, counting stale reads.
	convergenceStart := time.Now()
	staleReads := 0
	converged := false
	var avgGetLatency float64
	getCount := 0

	for time.Since(convergenceStart) < 15*time.Second {
		allMatch := true
		for i := 1; i < len(clients); i++ {
			resp, getLatency, err := doGet(clients[i], key, pb.ConsistencyModel_EVENTUAL)
			avgGetLatency += float64(getLatency.Microseconds()) / 1000.0
			getCount++
			if err != nil || !resp.Found || resp.Value != value {
				allMatch = false
				if err == nil {
					staleReads++
				}
				break
			}
		}
		if allMatch {
			converged = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	convergenceTime := time.Since(convergenceStart)
	if getCount > 0 {
		avgGetLatency /= float64(getCount)
	}

	if converged {
		fmt.Printf("  CONVERGED in %.2fms, stale_reads=%d\n",
			float64(convergenceTime.Microseconds())/1000.0, staleReads)
	} else {
		fmt.Printf("  FAILED TO CONVERGE within 15s, stale_reads=%d\n", staleReads)
	}

	// Reset delay.
	for i, c := range clients {
		if err := setDelay(c, 0); err != nil {
			fmt.Printf("  Warning: failed to reset delay on %s: %v\n", addrs[i], err)
		}
	}

	return ScenarioMetrics{
		Scenario:      "4 (Network Delay 500ms)",
		Consistency:   "Eventual",
		PutLatencyMs:  float64(putLatency.Microseconds()) / 1000.0,
		GetLatencyMs:  avgGetLatency,
		ConvergenceMs: float64(convergenceTime.Microseconds()) / 1000.0,
		StaleReads:    staleReads,
		Success:       converged,
	}
}

// ─── Output ───────────────────────────────────────────────────────────────

func printResultsTable(metrics []ScenarioMetrics) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                    SCENARIO RESULTS SUMMARY                                            ║")
	fmt.Println("╠════════════════════════════╦═════════════╦═══════════════╦═══════════════╦══════════════════╦════════════╣")
	fmt.Printf("║ %-26s ║ %-11s ║ %-13s ║ %-13s ║ %-16s ║ %-10s ║\n",
		"Scenario", "Consistency", "PUT Lat (ms)", "GET Lat (ms)", "Convergence (ms)", "Stale Reads")
	fmt.Println("╠════════════════════════════╬═════════════╬═══════════════╬═══════════════╬══════════════════╬════════════╣")
	for _, m := range metrics {
		convergence := fmt.Sprintf("%.1f", m.ConvergenceMs)
		if m.ConvergenceMs == 0 {
			convergence = "0 (Immediate)"
		}
		fmt.Printf("║ %-26s ║ %-11s ║ %13.1f ║ %13.1f ║ %16s ║ %10d ║\n",
			m.Scenario, m.Consistency, m.PutLatencyMs, m.GetLatencyMs, convergence, m.StaleReads)
	}
	fmt.Println("╚════════════════════════════╩═════════════╩═══════════════╩═══════════════╩══════════════════╩════════════╝")
}

func saveResults(metrics []ScenarioMetrics, dir string) {
	os.MkdirAll(dir, 0o755)
	for i, m := range metrics {
		data, _ := json.MarshalIndent(m, "", "  ")
		filename := fmt.Sprintf("%s/scenario%d.json", dir, i+1)
		os.WriteFile(filename, data, 0o644)
		fmt.Printf("  Saved %s\n", filename)
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────

func main() {
	// Subcommands.
	putCmd := flag.NewFlagSet("put", flag.ExitOnError)
	putTarget := putCmd.String("target", "localhost:8001", "Replica address")
	putConsistency := putCmd.String("consistency", "eventual", "Consistency model (eventual/strong)")

	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	getTarget := getCmd.String("target", "localhost:8001", "Replica address")
	getConsistency := getCmd.String("consistency", "eventual", "Consistency model (eventual/strong)")

	scenarioCmd := flag.NewFlagSet("scenario", flag.ExitOnError)
	scenarioReplicas := scenarioCmd.String("replicas", "localhost:8001,localhost:8002,localhost:8003", "Comma-separated replica addresses")
	scenarioName := scenarioCmd.String("name", "all", "Scenario to run: 1, 2, 3, 4, or all")
	resultsDir := scenarioCmd.String("results-dir", "results", "Directory to save results")

	if len(os.Args) < 2 {
		fmt.Println("Usage: client <command> [options]")
		fmt.Println("Commands: put, get, scenario")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "put":
		putCmd.Parse(os.Args[2:])
		args := putCmd.Args()
		if len(args) < 2 {
			fmt.Println("Usage: client put [options] <key> <value>")
			os.Exit(1)
		}
		client, conn, err := connect(*putTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		consistency := pb.ConsistencyModel_EVENTUAL
		if *putConsistency == "strong" {
			consistency = pb.ConsistencyModel_STRONG
		}

		resp, latency, err := doPut(client, args[0], args[1], consistency)
		if err != nil {
			fmt.Fprintf(os.Stderr, "PUT failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("PUT %s=%s | success=%v version=%d latency=%.2fms\n",
			args[0], args[1], resp.Success, resp.Version, float64(latency.Microseconds())/1000.0)
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
			os.Exit(1)
		}

	case "get":
		getCmd.Parse(os.Args[2:])
		args := getCmd.Args()
		if len(args) < 1 {
			fmt.Println("Usage: client get [options] <key>")
			os.Exit(1)
		}
		client, conn, err := connect(*getTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		consistency := pb.ConsistencyModel_EVENTUAL
		if *getConsistency == "strong" {
			consistency = pb.ConsistencyModel_STRONG
		}

		resp, latency, err := doGet(client, args[0], consistency)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GET failed: %v\n", err)
			os.Exit(1)
		}
		if !resp.Found {
			fmt.Printf("GET %s | NOT FOUND latency=%.2fms\n", args[0], float64(latency.Microseconds())/1000.0)
		} else {
			fmt.Printf("GET %s=%s | version=%d updated_by=%s latency=%.2fms\n",
				args[0], resp.Value, resp.Version, resp.UpdatedBy, float64(latency.Microseconds())/1000.0)
		}

	case "scenario":
		scenarioCmd.Parse(os.Args[2:])
		addrs := strings.Split(*scenarioReplicas, ",")
		if len(addrs) < 3 {
			fmt.Fprintln(os.Stderr, "Need at least 3 replica addresses")
			os.Exit(1)
		}

		clients, conns, err := connectAll(addrs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			for _, c := range conns {
				c.Close()
			}
		}()

		// Wait for all replicas to be healthy.
		fmt.Println("Checking replica health...")
		for i, c := range clients {
			if err := waitForHealth(c, addrs[i], 10*time.Second); err != nil {
				fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  %s: healthy\n", addrs[i])
		}

		var allMetrics []ScenarioMetrics

		switch *scenarioName {
		case "1":
			allMetrics = append(allMetrics, runScenario1(clients, addrs))
		case "2":
			allMetrics = append(allMetrics, runScenario2(clients, addrs))
		case "3":
			allMetrics = append(allMetrics, runScenario3(clients, addrs))
		case "4":
			allMetrics = append(allMetrics, runScenario4(clients, addrs))
		case "all":
			// Scenarios 1, 3, 4 don't kill replicas, run them first.
			allMetrics = append(allMetrics, runScenario1(clients, addrs))
			allMetrics = append(allMetrics, runScenario3(clients, addrs))
			allMetrics = append(allMetrics, runScenario4(clients, addrs))
			// Scenario 2 kills replicas, run it last.
			allMetrics = append(allMetrics, runScenario2(clients, addrs))
		default:
			fmt.Fprintf(os.Stderr, "Unknown scenario: %s (use 1, 2, 3, 4, or all)\n", *scenarioName)
			os.Exit(1)
		}

		printResultsTable(allMetrics)
		saveResults(allMetrics, *resultsDir)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
