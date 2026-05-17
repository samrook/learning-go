# Bounded Worker Pool with Live Telemetry

Small go service to process high-throughput workloads with **strictly bounded goroutines**, using a buffered channel as backpressure and `sync/atomic` for lock-free telemetry.

## What it includes

- A fixed-size worker pool (`pool` package) with `Submit` and `TrySubmit`.
- A `/metrics` endpoint (plain text) exposing live pool stats.
- Tests that validate bounded concurrency, backpressure, and metrics output.

## Run tests

```bash
cd learning-go/bounded-worker-pool
go test ./...
```

## Run the demo server

```bash
cd learning-go/bounded-worker-pool
go run ./cmd/workerpoold -workers 4 -queue 32 -addr :8080
```

Then visit:

- `http://localhost:8080/metrics`
- `http://localhost:8080/enqueue?n=10000&ms=25`

