#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────
# run_scenarios.sh — Build, launch 3 replicas, run all scenarios, collect results.
# ──────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"
RESULTS_DIR="$PROJECT_DIR/results"
CONFIGS_DIR="$PROJECT_DIR/configs"

REPLICA_PIDS=()

cleanup() {
    echo ""
    echo "==> Cleaning up..."
    for pid in "${REPLICA_PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            echo "    Stopping replica (PID $pid)"
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
    echo "==> Done."
}

trap cleanup EXIT

# ── Step 1: Build ─────────────────────────────────────────────────────────

echo "==> Building..."
mkdir -p "$BIN_DIR" "$RESULTS_DIR"

(cd "$PROJECT_DIR" && go build -o "$BIN_DIR/replica" ./cmd/replica/)
(cd "$PROJECT_DIR" && go build -o "$BIN_DIR/client"  ./cmd/client/)

echo "    Built replica and client binaries."

# ── Step 2: Start replicas ────────────────────────────────────────────────

echo "==> Starting replicas..."

for i in 1 2 3; do
    "$BIN_DIR/replica" --config "$CONFIGS_DIR/replica${i}.json" &
    REPLICA_PIDS+=($!)
    echo "    Started replica-$i (PID ${REPLICA_PIDS[$((i-1))]})"
done

# Give replicas time to start.
sleep 2

# ── Step 3: Run scenarios ────────────────────────────────────────────────

SCENARIO="${1:-all}"
echo "==> Running scenario: $SCENARIO"

"$BIN_DIR/client" scenario \
    --replicas "localhost:8001,localhost:8002,localhost:8003" \
    --name "$SCENARIO" \
    --results-dir "$RESULTS_DIR"

echo ""
echo "==> Results saved to $RESULTS_DIR/"
