// part2/main.go
// Distributed Computing HW1 - Part 2: Concurrency, Scheduling, Context Switching
// Author: SayyedAliTahami
// Description: Measures effect of #goroutines, GOMAXPROCS, and workload type on performance.

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/trace"
	"sync"
	"time"
)

const (
	totalOps       = 10000000
	taskIterations = 50
)

var (
	dummyResult int
)


type WorkloadFunction func(mu *sync.Mutex)

func cpuWorkload(mu *sync.Mutex) {
	acc := 0
	for i := 0; i < taskIterations; i++ {
		acc += i * i
	}
	dummyResult += acc
}

func mixedWorkload(mu *sync.Mutex) {
	acc := 0
	for i := 0; i < taskIterations; i++ {
		acc += i * i
	}
	dummyResult += acc

	mu.Lock()
	dummyResult++
	mu.Unlock()
}


type BenchResult struct {
	GOMAXPROCS   int
	NGoroutines  int
	WorkloadType string
	TotalTimeMs  float64
	Throughput   float64
	AvgLatencyUs float64
}

func runBenchmark(gomax int, nGo int, workloadType string) BenchResult {
	runtime.GOMAXPROCS(gomax)

	opsPerGo := totalOps / nGo
	remainder := totalOps % nGo

	var workFunc WorkloadFunction
	switch workloadType {
	case "cpu":
		workFunc = cpuWorkload
	case "mixed":
		workFunc = mixedWorkload
	default:
		log.Fatalf("unknown workload: %s", workloadType)
	}

	var mu sync.Mutex

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < nGo; i++ {
		wg.Add(1)
		myOps := opsPerGo
		if i < remainder {
			myOps++
		}
		go func(ops int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				workFunc(&mu)
			}
		}(myOps)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalTimeSec := elapsed.Seconds()
	throughput := float64(totalOps) / totalTimeSec
	avgLatencyUs := (elapsed.Seconds() / float64(totalOps)) * 1e6

	return BenchResult{
		GOMAXPROCS:   gomax,
		NGoroutines:  nGo,
		WorkloadType: workloadType,
		TotalTimeMs:  elapsed.Seconds() * 1000,
		Throughput:   throughput,
		AvgLatencyUs: avgLatencyUs,
	}
}


func generateTrace(gomax int, nGo int, workloadType string) {
	fmt.Printf("\n  Generating trace for GOMAXPROCS=%d, goroutines=%d, workload=%s ...\n", gomax, nGo, workloadType)
	f, err := os.Create("trace.out")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	trace.Start(f)
	defer trace.Stop()

	runtime.GOMAXPROCS(gomax)
	opsPerGo := totalOps / nGo
	remainder := totalOps % nGo

	var workFunc WorkloadFunction
	if workloadType == "cpu" {
		workFunc = cpuWorkload
	} else {
		workFunc = mixedWorkload
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < nGo; i++ {
		wg.Add(1)
		myOps := opsPerGo
		if i < remainder {
			myOps++
		}
		go func(ops int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				workFunc(&mu)
			}
		}(myOps)
	}
	wg.Wait()
	fmt.Println("   Trace written to trace.out")
	fmt.Println("   View with: go tool trace trace.out")
}


func main() {
	gomaxValues := []int{1, 2, runtime.NumCPU()}
	goroutineValues := []int{1, 2, 4, 8, 16, 32, 64}
	workloads := []string{"cpu", "mixed"}

	var allResults []BenchResult

	fmt.Println("  Starting benchmarks for Part 2 ...")
	fmt.Println("======================================")

	for _, gomax := range gomaxValues {
		for _, nGo := range goroutineValues {
			for _, wl := range workloads {
				fmt.Printf("Running: GOMAXPROCS=%d, goroutines=%d, workload=%s ... ", gomax, nGo, wl)
				res := runBenchmark(gomax, nGo, wl)
				allResults = append(allResults, res)
				fmt.Printf("done (%.2f ms, %.0f ops/sec)\n", res.TotalTimeMs, res.Throughput)
			}
		}
	}

	csvFile, err := os.Create("results/results.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	writer.Write([]string{
		"GOMAXPROCS", "Goroutines", "Workload",
		"TotalTime_ms", "Throughput_ops_sec", "AvgLatency_us",
	})

	for _, r := range allResults {
		row := []string{
			fmt.Sprintf("%d", r.GOMAXPROCS),
			fmt.Sprintf("%d", r.NGoroutines),
			r.WorkloadType,
			fmt.Sprintf("%.2f", r.TotalTimeMs),
			fmt.Sprintf("%.2f", r.Throughput),
			fmt.Sprintf("%.2f", r.AvgLatencyUs),
		}
		writer.Write(row)
	}

	fmt.Println("\n  Results saved to results.csv")

	generateTrace(2, 16, "mixed")

	// چاپ یک نمونه جدول در کنسول
	fmt.Println("\n Sample Table (first 10 rows):")
	fmt.Println("GOMAXPROCS\tGoroutines\tWorkload\tTime(ms)\tThroughput\tAvgLatency(us)")
	for i, r := range allResults {
		if i >= 10 {
			break
		}
		fmt.Printf("%d\t\t%d\t\t%s\t\t%.2f\t\t%.0f\t\t%.2f\n",
			r.GOMAXPROCS, r.NGoroutines, r.WorkloadType,
			r.TotalTimeMs, r.Throughput, r.AvgLatencyUs)
	}
}