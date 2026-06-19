package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	scenario    int
	replicasRaw string
	replicas    map[string]string // ID -> URL
)

// Item matches the JSON structure returned by replicas
type Item struct {
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func printStep(step string, desc string) {
	fmt.Printf("%s%s[%s]%s %s\n", colorBold, colorBlue, step, colorReset, desc)
}

func printDetail(label string, val interface{}) {
	fmt.Printf("  %s%s:%s %v\n", colorCyan, label, colorReset, val)
}

func printSuccess(msg string) {
	fmt.Printf("%s%s[SUCCESS] %s%s\n", colorBold, colorGreen, msg, colorReset)
}

func printFailure(msg string) {
	fmt.Printf("%s%s[FAILURE] %s%s\n", colorBold, colorRed, msg, colorReset)
}

func printWarning(msg string) {
	fmt.Printf("%s%s[WARN] %s%s\n", colorBold, colorYellow, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("%s%s[INFO] %s%s\n", colorBold, colorCyan, msg, colorReset)
}

func main() {
	flag.IntVar(&scenario, "scenario", 1, "Scenario to run (1-4)")
	flag.StringVar(&replicasRaw, "replicas", "", "Comma-separated list of replicas (id=url)")
	flag.Parse()

	if replicasRaw == "" {
		fmt.Println("Error: --replicas flag is required")
		os.Exit(1)
	}

	replicas = parseReplicas(replicasRaw)

	switch scenario {
	case 1:
		runScenario1()
	case 2:
		runScenario2()
	case 3:
		runScenario3()
	case 4:
		runScenario4()
	default:
		fmt.Printf("Unknown scenario: %d\n", scenario)
		os.Exit(1)
	}
}

func parseReplicas(raw string) map[string]string {
	res := make(map[string]string)
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			res[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return res
}

func writeKey(url, key, val string) (*http.Response, error) {
	body, _ := json.Marshal(map[string]string{"value": val})
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/kv/%s", url, key), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 35 * time.Second}
	return client.Do(req)
}

func readKey(url, key string) (Item, int, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/kv/%s", url, key))
	if err != nil {
		return Item{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Item{}, 404, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Item{}, resp.StatusCode, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var item Item
	err = json.NewDecoder(resp.Body).Decode(&item)
	return item, 200, err
}

func getStatus(url string) (map[string]interface{}, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/status", url))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	return res, err
}

func runScenario1() {
	fmt.Printf("\n%s==================================================%s\n", colorPurple, colorReset)
	fmt.Printf("%sRunning Scenario 1: Observe Temporary Inconsistency%s\n", colorBold, colorReset)
	fmt.Printf("%s==================================================%s\n", colorPurple, colorReset)

	r1URL, ok1 := replicas["replica1"]
	r2URL, ok2 := replicas["replica2"]
	if !ok1 || !ok2 {
		printFailure("Scenario 1 requires replica1 and replica2 URLs")
		os.Exit(1)
	}

	key := "x"
	val := "100"

	printStep("STEP 1", fmt.Sprintf("Writing key '%s' = '%s' to coordinator Replica 1 (R1)...", key, val))
	resp, err := writeKey(r1URL, key, val)
	if err != nil {
		printFailure(fmt.Sprintf("Failed to write to R1: %v", err))
		return
	}
	resp.Body.Close()
	printSuccess("Write operation confirmed by R1.")

	printStep("STEP 2", fmt.Sprintf("Immediately reading key '%s' from Replica 2 (R2)...", key))
	itemImmediate, codeImmediate, err := readKey(r2URL, key)
	
	printStep("STEP 3", "Recording immediate read outcome:")
	if err != nil {
		printFailure(fmt.Sprintf("Immediate read error: %v", err))
	} else if codeImmediate == 404 {
		printWarning("Immediate read result: Key NOT found (404 Stale / Inconsistent)")
	} else {
		printDetail("Value read from R2", itemImmediate.Value)
		printDetail("Version", itemImmediate.Version)
		printDetail("Updated By", itemImmediate.UpdatedBy)
	}

	printStep("STEP 4", "Waiting 3.5 seconds for eventual replication propagation...")
	time.Sleep(3500 * time.Millisecond)

	printStep("STEP 5", fmt.Sprintf("Reading key '%s' from Replica 2 (R2) again...", key))
	itemLater, codeLater, err := readKey(r2URL, key)
	if err != nil {
		printFailure(fmt.Sprintf("Later read error: %v", err))
	} else if codeLater == 404 {
		printFailure("Later read result: Key still NOT found (Convergence Failed)")
	} else {
		printSuccess("Later read succeeded (Consistent state reached).")
		printDetail("Value read from R2", itemLater.Value)
		printDetail("Version", itemLater.Version)
		printDetail("Updated By", itemLater.UpdatedBy)
	}
}

func runScenario2() {
	fmt.Printf("\n%s==================================================%s\n", colorPurple, colorReset)
	fmt.Printf("%sRunning Scenario 2: Replica Failure Handling%s\n", colorBold, colorReset)
	fmt.Printf("%s==================================================%s\n", colorPurple, colorReset)

	r1URL, ok1 := replicas["replica1"]
	if !ok1 {
		printFailure("Replica 1 URL is required")
		os.Exit(1)
	}

	key := fmt.Sprintf("s2-%d", time.Now().Unix())
	val := "val-s2"

	status, err := getStatus(r1URL)
	mode := "unknown"
	if err == nil {
		if m, ok := status["mode"].(string); ok {
			mode = m
		}
	}

	printStep("STEP 1", fmt.Sprintf("Checking active replica consistency mode: %s", mode))
	printStep("STEP 2", fmt.Sprintf("Attempting PUT key '%s' = '%s' to coordinator R1...", key, val))
	
	startPut := time.Now()
	resp, err := writeKey(r1URL, key, val)
	duration := time.Since(startPut)

	if err != nil {
		printFailure(fmt.Sprintf("PUT operation failed: %v (Duration: %v)", err, duration))
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		printSuccess(fmt.Sprintf("PUT succeeded. Status: %s (Duration: %v)", resp.Status, duration))
		printDetail("Response Body", string(bodyBytes))
	} else {
		printFailure(fmt.Sprintf("PUT failed with status: %d (Duration: %v)", resp.StatusCode, duration))
		printDetail("Response Body", string(bodyBytes))
	}

	printStep("STEP 3", "Consistency Model Comparison:")
	if mode == "eventual" {
		printInfo("Under eventual consistency, the write is registered locally and returns success immediately.")
		printInfo("Replication is sent in the background. If replicas are down, the client write is still successful.")
	} else if mode == "strong" {
		printInfo("Under strong consistency, the write must be replicated to a quorum (2 out of 3 replicas).")
		printInfo("If only 1 replica has failed (meaning 2 are still active), the write still succeeds because quorum is reached.")
		printInfo("If 2 replicas have failed (meaning only 1 is active), the write fails (503 Service Unavailable) because quorum cannot be reached.")
	}
}

func runScenario3() {
	fmt.Printf("\n%s==================================================%s\n", colorPurple, colorReset)
	fmt.Printf("%sRunning Scenario 3: Concurrent Conflict Resolution%s\n", colorBold, colorReset)
	fmt.Printf("%s==================================================%s\n", colorPurple, colorReset)

	r1URL := replicas["replica1"]
	r2URL := replicas["replica2"]
	if r1URL == "" || r2URL == "" {
		printFailure("Scenario 3 requires replica1 and replica2 URLs")
		os.Exit(1)
	}

	key := "conflict-key"

	printStep("STEP 1", fmt.Sprintf("Sending concurrent writes to R1 ('val_r1') and R2 ('val_r2') for key '%s'...", key))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		resp, err := writeKey(r1URL, key, "val_r1")
		if err == nil {
			resp.Body.Close()
			printInfo(fmt.Sprintf("Write to Replica 1 returned status: %s", resp.Status))
		} else {
			printFailure(fmt.Sprintf("Write to Replica 1 failed: %v", err))
		}
	}()

	go func() {
		defer wg.Done()
		resp, err := writeKey(r2URL, key, "val_r2")
		if err == nil {
			resp.Body.Close()
			printInfo(fmt.Sprintf("Write to Replica 2 returned status: %s", resp.Status))
		} else {
			printFailure(fmt.Sprintf("Write to Replica 2 failed: %v", err))
		}
	}()

	wg.Wait()

	printStep("STEP 2", "Waiting 4.5 seconds for concurrent replication propagation and LWW resolution...")
	time.Sleep(4500 * time.Millisecond)

	printStep("STEP 3", "Verifying final state and read values from all replicas:")
	for name, url := range replicas {
		item, code, err := readKey(url, key)
		if err != nil {
			printFailure(fmt.Sprintf("%s: Read error: %v", name, err))
		} else if code != 200 {
			printFailure(fmt.Sprintf("%s: Not found / bad status (%d)", name, code))
		} else {
			printDetail(fmt.Sprintf("%s value", name), fmt.Sprintf("'%s' (Version: %d, UpdatedBy: %s)", item.Value, item.Version, item.UpdatedBy))
		}
	}

	printStep("STEP 4", "Conflict Resolution Policy Explanation:")
	printInfo("The key-value store implements Last-Write-Wins (LWW) with version checks.")
	printInfo("When concurrent writes occur (both starting at Version 1), they have equal versions.")
	printInfo("To resolve equal-version conflicts deterministically, we compare replica IDs alphabetically.")
	printInfo("Since 'replica2' > 'replica1' alphabetically, the value written to replica2 ('val_r2') wins.")
	printInfo("As a result, all replicas converge to 'val_r2' once replication messages are fully exchanged.")
}

func runScenario4() {
	r1URL := replicas["replica1"]
	if r1URL == "" {
		fmt.Println("Error: replica1 URL is required")
		os.Exit(1)
	}

	status, err := getStatus(r1URL)
	if err != nil {
		fmt.Printf("Error getting replica status: %v\n", err)
		os.Exit(1)
	}
	mode := status["mode"].(string)
	delay := int(status["delay"].(float64))

	key := fmt.Sprintf("s4-%d", time.Now().Unix())
	val := "val-s4"

	// Measure PUT latency
	startPut := time.Now()
	resp, err := writeKey(r1URL, key, val)
	if err != nil {
		fmt.Printf("Error: PUT failed: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()
	putLatency := time.Since(startPut)

	// Measure GET latency (from coordinator R1)
	startGet := time.Now()
	_, _, err = readKey(r1URL, key)
	if err != nil {
		fmt.Printf("Error: GET failed: %v\n", err)
		os.Exit(1)
	}
	getLatency := time.Since(startGet)

	// Measure Convergence Time
	startPoll := time.Now()
	converged := false
	var convergenceTime time.Duration

	for time.Since(startPoll) < 15*time.Second {
		allConverged := true
		for _, url := range replicas {
			item, code, err := readKey(url, key)
			if err != nil || code != 200 || item.Value != val {
				allConverged = false
				break
			}
		}
		if allConverged {
			convergenceTime = time.Since(startPut)
			converged = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	convVal := "timeout"
	if converged {
		convVal = fmt.Sprintf("%.2fms", float64(convergenceTime.Milliseconds()))
	}
	fmt.Printf("RESULT: %s %dms %.2fms %.2fms %s\n",
		mode,
		delay,
		float64(putLatency.Milliseconds()),
		float64(getLatency.Milliseconds()),
		convVal,
	)
}
