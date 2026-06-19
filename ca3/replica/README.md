# Replica Server

This folder contains the Go implementation of a single replica server for our versioned replicated key-value database.

## Features
- In-memory database with Last-Write-Wins (LWW) conflict resolution.
- Standard HTTP endpoints for client read/write and replica replication.
- Support for concurrent quorum-based writes (`strong`) or asynchronous background writes (`eventual`).
- Configurable artificial network delays.
- Configurable via flags or JSON config files.

## Running

### Via Flags
```bash
go run main.go store.go --id=replica1 --port=8001 --mode=strong --delay=500 --replicas=replica1=http://localhost:8001,replica2=http://localhost:8002,replica3=http://localhost:8003
```

### Via Config File
```bash
go run main.go store.go --config=../configs/replica1.json
```
Note: You can override config file values by supplying additional flags on the command line.
