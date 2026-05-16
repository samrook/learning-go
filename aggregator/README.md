# Resilience-First API Aggregator (Go)

Small Go service that demonstrates a common “API aggregator” pattern: fetch multiple downstream resources concurrently, apply per-request timeouts with `context`, and return partial data when one downstream is slow.

## What it does

- Exposes `GET /aggregate`
- Calls three simulated downstream APIs concurrently:
  - `GET /api/artist-bio`
  - `GET /api/current-song`
  - `GET /api/album-art`
- Uses a shorter timeout for Album Art (default `500ms`). If it’s slow, the request is canceled and the response still returns the other data.

## Run

```bash
go test ./...
go run ./cmd/aggregator
```

In another terminal:

```bash
curl -s http://127.0.0.1:8080/aggregate | jq .
```

By default, `album_art` is intentionally slow (700ms), so you’ll see `artist_bio` + `current_song` with an `errors` entry for album art timing out.

## Configuration

- `-addr` (default `:8080`): HTTP listen address
- `-base-url` (default empty): base URL for downstream APIs. If not set, the service points to itself.

Example:

```bash
go run ./cmd/aggregator -addr :8081 -base-url http://127.0.0.1:8081
```
