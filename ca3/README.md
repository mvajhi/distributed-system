# Replicated Key-Value Store

A distributed key-value store in Go with gRPC, supporting **3 independent replica processes**, two consistency models, versioning, and conflict resolution.

## Features

- **Eventual Consistency**: Write locally, replicate asynchronously
- **Strong Consistency (Quorum-based)**: Write succeeds only when majority (2/3) of replicas acknowledge
- **Versioning & LWW Conflict Resolution**: Version numbers + timestamp-based Last-Write-Wins
- **Network Delay Injection**: Configurable delay for testing
- **Structured Logging**: JSON logs with timestamps, replica IDs, event types, and versions
- **Automated Scenario Runner**: 4 test scenarios with metrics collection

## Quick Start

### Prerequisites

- Go 1.21+
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` plugins

### Build & Run All Scenarios

```bash
./scripts/run_scenarios.sh
```

This will:
1. Build the `replica` and `client` binaries
2. Start 3 replica processes (ports 8001, 8002, 8003)
3. Run all 4 test scenarios
4. Print a metrics summary table
5. Save results to `results/`

### Manual Usage

**Start replicas** (in separate terminals):
```bash
go run ./cmd/replica/ --config configs/replica1.json
go run ./cmd/replica/ --config configs/replica2.json
go run ./cmd/replica/ --config configs/replica3.json
```

**Client operations:**
```bash
# PUT (eventual)
go run ./cmd/client/ put --target localhost:8001 --consistency eventual mykey myvalue

# PUT (strong)
go run ./cmd/client/ put --target localhost:8001 --consistency strong mykey myvalue

# GET
go run ./cmd/client/ get --target localhost:8002 mykey

# Run specific scenario
go run ./cmd/client/ scenario --name 1

# Run all scenarios
go run ./cmd/client/ scenario --name all
```

## Architecture

```
┌─────────┐     gRPC      ┌───────────────┐     gRPC      ┌───────────────┐
│  Client  │──────────────▶│   Replica 1   │◄─────────────▶│   Replica 2   │
│   CLI    │               │  (port 8001)  │               │  (port 8002)  │
└─────────┘               └───────────────┘               └───────────────┘
                                  ▲                               ▲
                                  │          gRPC                 │
                                  └──────────────┐────────────────┘
                                                 │
                                          ┌──────┴────────┐
                                          │   Replica 3   │
                                          │  (port 8003)  │
                                          └───────────────┘
```

Each replica is an **independent OS process** with its own gRPC server and in-memory storage.

## Project Structure

```
ca3/
├── proto/kvstore.proto          # gRPC service definition
├── proto/pb/                    # Generated Go code
├── cmd/
│   ├── replica/main.go          # Replica entry-point
│   └── client/main.go           # Client CLI
├── internal/
│   ├── config/config.go         # JSON config loader
│   ├── logger/logger.go         # Structured slog logger
│   ├── storage/memory.go        # Thread-safe KV store + conflict resolution
│   ├── replication/
│   │   ├── replicator.go        # Replicator interface
│   │   ├── eventual.go          # Async replication
│   │   └── strong.go            # Quorum-based replication
│   └── server/grpc_server.go    # gRPC service implementation
├── configs/                     # Per-replica JSON configs
├── scripts/run_scenarios.sh     # Automated test runner
└── results/                     # Scenario output
```

## Test Scenarios

| # | Scenario | Model | What It Tests |
|---|----------|-------|---------------|
| 1 | Temporary Inconsistency | Eventual | Stale reads during async replication |
| 2 | Replica Failure | Strong | Quorum behavior with 1 and 2 failures |
| 3 | Concurrent Conflict | Eventual | LWW conflict resolution convergence |
| 4 | Network Delay (500ms) | Eventual | Convergence under network latency |

## Testing

```bash
# Unit tests with race detection
go test -race -v ./internal/...

# Integration tests (starts replicas automatically)
./scripts/run_scenarios.sh
```
