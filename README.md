# Simple Statistic Daemon (SSD)

A high-performance Go daemon for collecting and aggregating real-time content statistics (views, clicks) with fingerprint-based user tracking. Single binary, zero external dependencies, ~150K writes/sec.

> **TL;DR** — Drop-in analytics backend: send views/clicks via JSON POST, query trending stats via GET. No database required.

## Why SSD?

| Problem | SSD's answer |
|---------|-------------|
| Need real-time view/click counting | Double-buffer pattern: **~154,000 POST/sec** |
| Database is overkill for simple counters | Standalone binary, data persisted as Zstd-compressed JSON |
| Read latency under load | In-place mutation + optional response cache: **P99 under 30ms** |
| Content goes stale but counters only grow | Trending algorithm with automatic time-decay |
| Worried about data loss on crash | Atomic writes (tmp + fsync + rename) |
| Multi-tenant / multi-section stats | Channel-based isolation (`?ch=news`, `?ch=blog`) |

## Features

- **High Performance** — double-buffer pattern with in-place mutation, ~154,000 POST req/sec, ~16,000 mixed RPS
- **Fast JSON** — `goccy/go-json` for 2-3x faster serialization vs stdlib `encoding/json`
- **Response Cache** — optional freecache-based caching with zero-alloc key lookup (`unsafe.Slice`), TTL = aggregation interval + 1s
- **Zero External Dependencies** — standalone binary, no databases or message queues
- **Trending Algorithm** — automatic time-decay: views > 512 triggers halving with factor counter for trending CTR
- **Fingerprint Tracking** — per-user statistics grouped by browser fingerprint
- **Channel Isolation** — separate stat namespaces via `ch` parameter (up to 1,000 channels), double-check RLock/Lock pattern
- **Crash-Safe Persistence** — atomic file writes with Zstd compression
- **Graceful Shutdown** — SIGINT/SIGTERM handling with data persistence before exit
- **Prometheus Metrics** — optional `/metrics` endpoint with request counters, latency histograms, cache hit/miss, persistence duration, buffer/channel gauges
- **Health Check** — `GET /health` for Kubernetes readiness/liveness probes (uptime, buffer size, channel count)
- **HTTP Hardened** — server-side ReadTimeout, WriteTimeout, IdleTimeout
- **Fully Tested** — unit tests with race detector, 100% coverage on models and services
- **Docker Ready** — multi-stage Dockerfile included

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/DimitriyKuschel/ssd
cd ssd
cp .env.sample .env    # adjust settings if needed
docker compose up -d
```

The API is available at `http://localhost:8090`.

### Build from Source

```bash
git clone https://github.com/DimitriyKuschel/ssd
cd ssd
go build -o ssd ./
./ssd -config configs/config-dev.yml
```

### Release Build (GoReleaser)

```bash
go get -u
go vet
goreleaser --rm-dist --skip-publish --skip-validate
```

Run the binary for your platform from the `dist` directory.

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to YAML config file | `/etc/ssd/ssd.yml` |
| `-debug` | Enable debug mode (console logging) | `false` |
| `-version` | Print version and exit | |
| `-help` | Show available flags | |
| `-test` | Test mode | `false` |

## API

All GET endpoints accept an optional `?ch=<channel>` query parameter for channel isolation. If omitted, the `"default"` channel is used.

### POST `/` — Submit Statistics

Record views and clicks for content items.

**Request:**
```json
{
  "v": ["105318", "58440"],
  "c": ["58440"],
  "f": "1035ed17aa899a3846b91b57021c2b4f",
  "ch": "news"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `v` | `string[]` | no | IDs of viewed content |
| `c` | `string[]` | no | IDs of clicked content |
| `f` | `string` | no | User fingerprint |
| `ch` | `string` | no | Channel name (default: `"default"`) |

**Response:** `201 Created`

### GET `/list` — Aggregated Statistics

Returns trending statistics for all tracked content.

**Response:** `200 OK`
```json
{
  "105318": { "Views": 1, "Clicks": 0, "Ftr": 0 },
  "58440":  { "Views": 1, "Clicks": 1, "Ftr": 0 }
}
```

| Field | Description |
|-------|-------------|
| `Views` | View count (halved when > 512) |
| `Clicks` | Click count (halved proportionally) |
| `Ftr` | Factor — number of times values were halved |

To reconstruct full values: `Views * 2^Ftr`, `Clicks * 2^Ftr`.

### GET `/fingerprints` — Statistics by Fingerprint

Returns all statistics grouped by user fingerprint.

**Response:** `200 OK`
```json
{
  "1035ed17aa899a3846b91b57021c2b4f": {
    "data": {
      "105318": { "Views": 1, "Clicks": 0, "Ftr": 0 }
    }
  }
}
```

### GET `/fingerprint?f={id}` — Single Fingerprint Statistics

Returns statistics for a specific user fingerprint.

**Response:** `200 OK`
```json
{
  "105318": { "Views": 1, "Clicks": 0, "Ftr": 0 }
}
```

### GET `/channels` — List Channels

Returns all active channel names.

**Response:** `200 OK`
```json
["default", "news", "blog"]
```

### GET `/health` — Health Check

Returns service health status. Useful for Kubernetes readiness/liveness probes.

**Response:** `200 OK`
```json
{
  "status": "ok",
  "uptime": "1h30m45s",
  "uptime_seconds": 5445.0,
  "buffer_size": 128,
  "channels": 3
}
```

### GET `/metrics` — Prometheus Metrics

Returns metrics in Prometheus text format. Only available when `metrics.enabled: true`.

**Exported metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ssd_requests_total` | Counter | endpoint, status | HTTP request count |
| `ssd_request_duration_seconds` | Histogram | endpoint | Request latency |
| `ssd_cache_hits_total` | Counter | — | Cache hit count |
| `ssd_cache_misses_total` | Counter | — | Cache miss count |
| `ssd_persistence_duration_seconds` | Histogram | — | Persistence operation duration |
| `ssd_buffer_size` | Gauge | — | Items in the active buffer |
| `ssd_channels_total` | Gauge | — | Number of channels |
| `ssd_records_total` | Gauge | channel | Stat records per channel |

## Configuration

### YAML Config

```yaml
pidFile: "/tmp/ssd.pid"
statistic:
  interval: 60s
webServer:
  host: "0.0.0.0"
  port: 8090
persistence:
  filePath: "/data/ssd/data.bin"
  saveInterval: 120s
cache:
  enabled: true
  size: 32
metrics:
  enabled: true
logger:
  level: "info"
  mode: 0640
  dir: "/var/log/ssd"
```

### Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `pidFile` | PID file path | `/tmp/ssd.pid` |
| `statistic.interval` | Stats aggregation interval (seconds) | `60` |
| `webServer.host` | Listen address | `127.0.0.1` |
| `webServer.port` | Listen port | `8090` |
| `persistence.filePath` | Compressed data file path | `/etc/ssd/data.bin` |
| `persistence.saveInterval` | Data save interval (seconds) | `600` |
| `logger.level` | Log level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` | `info` |
| `logger.mode` | Log file permissions | `0640` |
| `logger.dir` | Log files directory | `/var/log/ssd` |
| `cache.enabled` | Enable response cache | `false` |
| `cache.size` | Cache size in MB | `32` |
| `metrics.enabled` | Enable Prometheus `/metrics` endpoint | `false` |

### Environment Variables (Docker)

Environment variables override YAML config values. Configure via `.env` file or `docker compose` environment:

| Variable | Overrides | Default |
|----------|-----------|---------|
| `SSD_PORT` | Host port mapping | `8090` |
| `SSD_DATA_DIR` | Data directory on host | `./data` |
| `SSD_LOGS_DIR` | Logs directory on host | `./logs` |
| `SSD_LOG_LEVEL` | `logger.level` | `info` |
| `SSD_AGGREGATION_INTERVAL` | `statistic.interval` | `60s` |
| `SSD_SAVE_INTERVAL` | `persistence.saveInterval` | `120s` |
| `SSD_CACHE_ENABLED` | `cache.enabled` | `true` |
| `SSD_CACHE_SIZE` | `cache.size` | `32` |
| `SSD_METRICS_ENABLED` | `metrics.enabled` | `true` |

## Architecture

```
HTTP Request → MetricsMiddleware → Router (method check) → ApiController → StatisticService → Models
     ↑                                                                           ↓
/health  /metrics                                        Scheduler (aggregation + persistence + metrics)
                                                                                 ↓
                                                         FileManager → Zstd Compressor → Disk
```

- **Double-Buffering** — the active buffer receives incoming stats (pre-allocated based on previous size) while the inactive buffer is processed during aggregation, swapped atomically via mutex
- **In-Place Mutation** — StatRecord fields are modified directly instead of allocating new objects, eliminating ~150K allocs/sec on the write path
- **Trending Decay** — when views exceed 512, values are halved via bit-shift `(n+1)>>1` and `Ftr` increments, naturally decaying old content
- **Atomic Snapshot** — `GetSnapshot()` collects all channel data under a single RLock for consistent persistence
- **Atomic Persistence** — writes to a temp file, syncs to disk, then renames for crash safety
- **Two-Mux Routing** — outer mux handles `/health` and `/metrics` (infrastructure); inner mux handles API routes wrapped with metrics middleware
- **Metrics** — Prometheus pull model via `/metrics`; noop provider injected when disabled (zero overhead)
- **Dependency Injection** — Google Wire for automatic wiring

## Testing

The project has comprehensive unit tests with race condition detection.

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### Coverage

| Package | Coverage | Tests |
|---------|----------|-------|
| `models/` | 100% | 30 |
| `services/` | 80% | 18 |
| `controllers/` | 90% | 31 |
| `statistic/` | 71% | 25 |
| `providers/` | 68% | 36 |
| `internal` (routes) | 17% | 2 |
| **Total** | | **142** |

All tests run with `-race` enabled to verify thread safety of concurrent data structures (double-buffer, per-channel maps, `sync.RWMutex`-protected models).

## Project Structure

```
├── configs/            YAML configs (dev, release, docker)
├── deployments/        Systemd service, logrotate config
├── internal/
│   ├── controllers/    HTTP handlers (+ tests)
│   ├── di/             Wire dependency injection
│   ├── models/         Data models with thread-safe maps (+ tests)
│   ├── providers/      Config, Logger, Router, Cache, Metrics providers (+ tests)
│   ├── services/       StatisticService — double-buffer core (+ tests)
│   ├── statistic/      Scheduler, FileManager, Zstd compressor (+ tests)
│   ├── structures/     Config schema, CLI flags, Route definitions
│   └── testutil/       Shared mock implementations for tests
├── scripts/            Post-install script
├── tests/loadtest/     Load test tool and configs
├── Dockerfile          Multi-stage build
├── docker-compose.yml  Docker Compose setup
└── .env.sample         Environment config template
```

## Benchmarks

Load test: 50 concurrent workers, 10 seconds per phase, Apple Silicon. Source: `tests/loadtest/`.

```bash
go build -o /tmp/ssd-loadtest/ssd ./
/tmp/ssd-loadtest/ssd -config tests/loadtest/config-cache-on.yml &
go run tests/loadtest/main.go
```

### POST throughput (seeding phase)

~154,000 RPS (P99 = 1.3ms). Cache has no effect on write path.

### GET latency — Mixed load (70% POST, 30% GET)

| Endpoint | Cache OFF P99 | Cache ON P99 |
|---|---|---|
| GET /list | 16.1ms | 14.8ms |
| GET /fingerprints | 45.5ms | 41.6ms |
| GET /fingerprint | 16.4ms | 13.8ms |
| GET /channels | 15.5ms | 14.0ms |
| **Total RPS** | **15,646** | **15,583** |

### GET latency — Read-heavy load (10% POST, 90% GET)

| Endpoint | Cache OFF P99 | Cache ON P99 |
|---|---|---|
| GET /list | 30.5ms | 28.1ms |
| GET /fingerprints | 60.2ms | 66.1ms |
| GET /fingerprint | 29.5ms | 26.5ms |
| GET /channels | 27.0ms | 25.7ms |
| **Total RPS** | **5,970** | **5,845** |

### v1.2.0 performance improvements vs v1.1.x

In-place mutation, `goccy/go-json`, double-check locking, buffer pre-allocation, and atomic snapshots reduced latencies by 25-66% and increased throughput by 36-62%:

| Metric (Phase 3, cache OFF) | v1.1.x | v1.2.0 | Change |
|---|---|---|---|
| GET /list P99 | 59.3ms | 30.5ms | **-49%** |
| GET /fingerprints P99 | 147.7ms | 60.2ms | **-59%** |
| GET /fingerprint P99 | 87.1ms | 29.5ms | **-66%** |
| GET /channels P99 | 42.7ms | 27.0ms | **-37%** |
| Total RPS | 3,697 | 5,970 | **+62%** |

The core computation is now fast enough that the response cache provides only marginal additional benefit (~5-10% P99 reduction). Cache remains useful for deployments with very large datasets where deep-copy and JSON serialization dominate.

## Development

### Prerequisites

- Go 1.25+
- [Google Wire](https://github.com/google/wire) (for DI code generation, optional)

### Common Commands

```bash
go build -o ssd ./                  # Build
go test -race ./...                 # Test with race detector
go vet ./...                        # Lint
./ssd -config configs/config-dev.yml -debug   # Run locally
```

### Dependency Injection

After modifying `internal/di/injectors.go`, regenerate:

```bash
cd internal/di && wire
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Ensure tests pass: `go test -race ./...`
4. Submit a pull request

## License

SSD is licensed under the Apache License 2.0. See the [LICENSE](./LICENSE) file for details.
