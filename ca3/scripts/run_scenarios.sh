#!/bin/bash

# Exit on error
set -e

# Compile binaries using new layout
echo "Building server and client..."
mkdir -p bin
go build -o bin/server replica/main.go replica/store.go
go build -o bin/client client/main.go

REPLICAS="replica1=http://localhost:8001,replica2=http://localhost:8002,replica3=http://localhost:8003"
mkdir -p logs
mkdir -p results

PID1=""
PID2=""
PID3=""

start_replicas() {
    local mode=$1
    local delay=$2
    echo "Starting replica cluster (mode: $mode, delay: ${delay}ms)..."
    ./bin/server --id=replica1 --port=8001 --mode=$mode --delay=$delay --replicas=$REPLICAS > logs/replica1.log 2>&1 &
    PID1=$!
    ./bin/server --id=replica2 --port=8002 --mode=$mode --delay=$delay --replicas=$REPLICAS > logs/replica2.log 2>&1 &
    PID2=$!
    ./bin/server --id=replica3 --port=8003 --mode=$mode --delay=$delay --replicas=$REPLICAS > logs/replica3.log 2>&1 &
    PID3=$!
    sleep 1.5 # Wait for startup
}

stop_replicas() {
    echo "Stopping replica cluster..."
    kill $PID1 $PID2 $PID3 2>/dev/null || true
    wait $PID1 $PID2 $PID3 2>/dev/null || true
    PID1=""
    PID2=""
    PID3=""
}

# Cleanup on exit
trap stop_replicas EXIT

echo "=================================================="
echo "RUNNING SCENARIO 1 (Temporary Inconsistency)"
echo "=================================================="
start_replicas eventual 2000
./bin/client --scenario=1 --replicas=$REPLICAS > results/scenario1.txt 2>&1
cat results/scenario1.txt
stop_replicas

echo ""
echo "=================================================="
echo "RUNNING SCENARIO 2 (Replica Failures)"
echo "=================================================="

# Clear and initialize scenario2.txt
echo "==================================================" > results/scenario2.txt
echo "Scenario 2: Replica Failure Handling" >> results/scenario2.txt
echo "==================================================" >> results/scenario2.txt
echo "" >> results/scenario2.txt

echo "--- Sub-scenario 2a: Eventual Consistency, 1 Replica Down (R3) ---"
echo "--- Sub-scenario 2a: Eventual Consistency, 1 Replica Down (R3) ---" >> results/scenario2.txt
start_replicas eventual 0
echo "Stopping R3..."
kill $PID3 || true
wait $PID3 2>/dev/null || true
PID3=""
./bin/client --scenario=2 --replicas=$REPLICAS >> results/scenario2.txt 2>&1
stop_replicas
echo "" >> results/scenario2.txt

echo "--- Sub-scenario 2b: Strong Consistency, 1 Replica Down (R3) ---"
echo "--- Sub-scenario 2b: Strong Consistency, 1 Replica Down (R3) ---" >> results/scenario2.txt
start_replicas strong 0
echo "Stopping R3..."
kill $PID3 || true
wait $PID3 2>/dev/null || true
PID3=""
./bin/client --scenario=2 --replicas=$REPLICAS >> results/scenario2.txt 2>&1
stop_replicas
echo "" >> results/scenario2.txt

echo "--- Sub-scenario 2c: Eventual Consistency, 2 Replicas Down (R2, R3) ---"
echo "--- Sub-scenario 2c: Eventual Consistency, 2 Replicas Down (R2, R3) ---" >> results/scenario2.txt
start_replicas eventual 0
echo "Stopping R2 and R3..."
kill $PID2 $PID3 2>/dev/null || true
wait $PID2 $PID3 2>/dev/null || true
PID2=""
PID3=""
./bin/client --scenario=2 --replicas=$REPLICAS >> results/scenario2.txt 2>&1
stop_replicas
echo "" >> results/scenario2.txt

echo "--- Sub-scenario 2d: Strong Consistency, 2 Replicas Down (R2, R3) ---"
echo "--- Sub-scenario 2d: Strong Consistency, 2 Replicas Down (R2, R3) ---" >> results/scenario2.txt
start_replicas strong 0
echo "Stopping R2 and R3..."
kill $PID2 $PID3 2>/dev/null || true
wait $PID2 $PID3 2>/dev/null || true
PID2=""
PID3=""
set +e
./bin/client --scenario=2 --replicas=$REPLICAS >> results/scenario2.txt 2>&1
set -e
stop_replicas

cat results/scenario2.txt

echo ""
echo "=================================================="
echo "RUNNING SCENARIO 3 (Concurrent Conflicts)"
echo "=================================================="
start_replicas eventual 2000
./bin/client --scenario=3 --replicas=$REPLICAS > results/scenario3.txt 2>&1
cat results/scenario3.txt
stop_replicas

echo ""
echo "=================================================="
echo "RUNNING SCENARIO 4 (Network Delay Impact)"
echo "=================================================="

RESULTS=()

run_s4_case() {
    local mode=$1
    local delay=$2
    start_replicas $mode $delay
    
    local out
    out=$(./bin/client --scenario=4 --replicas=$REPLICAS | grep "RESULT:")
    echo "Got metrics: $out"
    RESULTS+=("$out")
    
    stop_replicas
    sleep 0.5
}

run_s4_case eventual 0
run_s4_case eventual 500
run_s4_case eventual 2000
run_s4_case strong 0
run_s4_case strong 500
run_s4_case strong 2000

# Write formatted table to scenario4.txt
{
    echo "# Scenario 4 Network Delay Impact Matrix"
    echo ""
    printf "| %-10s | %-7s | %-15s | %-15s | %-17s |\n" "Mode" "Delay" "PUT Latency" "GET Latency" "Convergence Time"
    printf "|------------|---------|-----------------|-----------------|------------------|\n"
    for res in "${RESULTS[@]}"; do
        data=${res#RESULT: }
        read -r m d put get conv <<< "$data"
        printf "| %-10s | %-7s | %-15s | %-15s | %-17s |\n" "$m" "$d" "$put" "$get" "$conv"
    done
    echo ""
    echo "---"
} > results/scenario4.txt

# Write JSON format to scenario4.json
{
    echo "["
    first=true
    for res in "${RESULTS[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        data=${res#RESULT: }
        read -r m d put get conv <<< "$data"
        d_val=${d%ms}
        put_val=${put%ms}
        get_val=${get%ms}
        conv_val=${conv%ms}
        if [ "$conv_val" = "timeout" ]; then
            conv_val="null"
        fi
        printf "  {\"mode\": \"%s\", \"delay_ms\": %s, \"put_latency_ms\": %s, \"get_latency_ms\": %s, \"convergence_time_ms\": %s}" "$m" "$d_val" "$put_val" "$get_val" "$conv_val"
    done
    echo ""
    echo "]"
} > results/scenario4.json

echo ""
cat results/scenario4.txt
echo ""
echo "Scenario 4 JSON results:"
cat results/scenario4.json
