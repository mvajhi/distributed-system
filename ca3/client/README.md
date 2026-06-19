# Client Scenario Runner

This folder contains the client implementation to orchestrate and execute testing scenarios on the replica cluster.

## Running

```bash
go run main.go --scenario=1 --replicas=replica1=http://localhost:8001,replica2=http://localhost:8002,replica3=http://localhost:8003
```

- `--scenario`: The scenario ID to run (1-4).
- `--replicas`: Comma-separated list of replica URL mappings.
