# Simple Statistic Daemon (SSD)

A high-performance Go daemon for collecting and aggregating real-time content statistics (views, clicks) with fingerprint-based user tracking. Single binary, zero external dependencies, ~140K writes/sec.

> **TL;DR** — Drop-in analytics backend: send views/clicks via JSON POST, query trending stats via GET. No database required.

## Why SSD?

| Problem | SSD's answer |
|---------|-------------|
| Need real-time view/click counting | Double-buffer pattern: **~140,000 POST/sec** |
| Database is overkill for simple counters | Standalone binary, data persisted as Zstd-compressed binary |
| Read latency under load | In-place mutation + optional response cache: **P99 under 30ms** |
| Content goes stale but counters only grow | Trending algorithm with automatic time-decay |
| Worried about data loss on crash | Atomic writes (tmp + fsync + rename) |
| Multi-tenant / multi-section stats | Channel-based isolation (`?ch=news`, `?ch=blog`) |
| Memory grows unbounded with fingerprints | Roaring Bitmaps (~96% savings), TTL eviction + cold storage |
| Need bounded memory usage | Configurable limits per channel, per fingerprint, with eviction |

## Features

- **High Performance** — double-buffer pattern with in-place mutation, ~140,000 POST req/sec, ~16,000 mixed RPS
- **Memory Efficient** — Roaring Bitmaps for fingerprint tracking (~96% memory reduction), value-type StatRecords (~63% savings), sparse counters
- **Bounded Memory** — configurable limits: `maxRecords` per channel, `maxRecordsPerFingerprint` per user, `maxChannels` globally; least-viewed entries evicted when limits are reached
- **Fingerprint TTL** — inactive fingerprints are evicted to cold storage on disk after configurable TTL, automatically restored on next interaction
- **Cold Storage** — write-behind disk tier for evicted fingerprints with lazy load, lazy delete, and configurable cold TTL for permanent cleanup
- **Fast JSON** — `goccy/go-json` for 2-3x faster serialization vs stdlib `encoding/json`
- **Binary Persistence** — V5 binary format with Roaring Bitmap serialization for fast save/restore; automatic migration from older JSON formats (V1-V4)
- **Response Cache** — optional freecache-based caching with zero-alloc key lookup (`unsafe.Slice`), TTL = aggregation interval + 1s
- **Zero External Dependencies** — standalone binary, no databases or message queues
- **Trending Algorithm** — automatic time-decay: views > 512 triggers halving with factor counter for trending CTR
- **Fingerprint Tracking** — per-user statistics grouped by browser fingerprint
- **Channel Isolation** — separate stat namespaces via `ch` parameter (configurable max channels), double-check RLock/Lock pattern
- **Crash-Safe Persistence** — atomic file writes (tmp + fsync + rename) with Zstd compression
- **Graceful Shutdown** — SIGINT/SIGTERM handling with data persistence and cold storage flush before exit
- **Prometheus Metrics** — optional `/metrics` endpoint with request counters, latency histograms, cache hit/miss, persistence duration, buffer/channel gauges
- **Health Check** — `GET /health` for Kubernetes readiness/liveness probes (uptime, buffer size, channel count)
- **HTTP Hardened** — server-side ReadTimeout, WriteTimeout, IdleTimeout
- **Fully Tested** — 253 unit tests with race detector
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
  maxChannels: 1000               # max channels (-1 = unlimited)
  maxRecords: -1                  # max trending records per channel (-1 = unlimited)
  evictionPercent: 10             # % of records to evict when limit is reached
  maxRecordsPerFingerprint: -1    # max content IDs per fingerprint (-1 = unlimited)
  fingerprintTTL: 24h             # inactivity TTL for fingerprints (0 = no eviction)
  coldStorageDir: ""              # cold storage dir (empty = auto: {persistence.dir}/fingerprints/)
  coldTTL: 720h                   # TTL for cold entries before permanent deletion (0 = keep forever)
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
| `statistic.interval` | Stats aggregation interval | `60s` |
| `statistic.maxChannels` | Max channels (-1 = unlimited) | `1000` |
| `statistic.maxRecords` | Max trending records per channel (-1 = unlimited) | `-1` |
| `statistic.evictionPercent` | % of records to evict when cap is reached | `10` |
| `statistic.maxRecordsPerFingerprint` | Max content IDs per fingerprint (-1 = unlimited) | `-1` |
| `statistic.fingerprintTTL` | Inactivity TTL for fingerprints; 0 = no eviction | `0` |
| `statistic.coldStorageDir` | Dir for `.cold.zst` files; empty = auto | `""` |
| `statistic.coldTTL` | TTL for cold entries before permanent deletion; 0 = forever | `0` |
| `webServer.host` | Listen address | `127.0.0.1` |
| `webServer.port` | Listen port | `8090` |
| `persistence.filePath` | Compressed data file path | `/etc/ssd/data.bin` |
| `persistence.saveInterval` | Data save interval | `600s` |
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
| `SSD_MAX_CHANNELS` | `statistic.maxChannels` | `1000` |
| `SSD_MAX_RECORDS` | `statistic.maxRecords` | `-1` |
| `SSD_EVICTION_PERCENT` | `statistic.evictionPercent` | `10` |
| `SSD_MAX_RECORDS_PER_FP` | `statistic.maxRecordsPerFingerprint` | `-1` |
| `SSD_FINGERPRINT_TTL` | `statistic.fingerprintTTL` | `0` |
| `SSD_COLD_STORAGE_DIR` | `statistic.coldStorageDir` | `""` |
| `SSD_COLD_TTL` | `statistic.coldTTL` | `0` |
| `SSD_SAVE_INTERVAL` | `persistence.saveInterval` | `120s` |
| `SSD_CACHE_ENABLED` | `cache.enabled` | `true` |
| `SSD_CACHE_SIZE` | `cache.size` | `32` |
| `SSD_METRICS_ENABLED` | `metrics.enabled` | `true` |

## Architecture

```
HTTP Request → MetricsMiddleware → Router (method check) → ApiController → StatisticService → Models
     ↑                                                                           ↓
/health  /metrics                                        Scheduler (aggregation + eviction + persistence)
                                                                                 ↓
                                                         FileManager → Zstd Compressor → Disk
                                                         ColdStorage → {channel}.cold.zst files
```

- **Double-Buffering** — the active buffer receives incoming stats (pre-allocated based on previous size) while the inactive buffer is processed during aggregation, swapped atomically via mutex
- **StatStore** — `map[uint32]StatRecord` (values inline, not pointers) with configurable `maxRecords` and eviction by least-viewed score
- **FingerprintRecord** — Roaring Bitmaps (`viewed`/`clicked`) + sparse `counts` map; first interaction = bitmap bit only, repeated = promoted to counts. ~96% memory reduction vs full StatRecord per ID
- **PersonalStatStore** — manages per-channel fingerprints with RLock fast path, TTL eviction, per-fingerprint record limits, and cold storage restore on miss
- **Cold Storage** — write-behind disk overflow: evicted fingerprints buffered in-memory, flushed to `{channel}.cold.zst` atomically; lazy load/delete minimizes I/O
- **Trending Decay** — when views exceed 512, values are halved via bit-shift `(n+1)>>1` and `Ftr` increments, naturally decaying old content
- **Atomic Snapshot** — `GetSnapshot()` collects all channel data under a single RLock for consistent persistence
- **Binary Persistence** — V5 format with Roaring Bitmap serialization; automatic migration from V1-V4 JSON formats
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
| `models/` | 85% | 109 |
| `services/` | 80% | 26 |
| `controllers/` | 90% | 23 |
| `statistic/` | 79% | 57 |
| `providers/` | 65% | 36 |
| `internal` (routes) | 16% | 2 |
| **Total** | | **253** |

All tests run with `-race` enabled to verify thread safety of concurrent data structures (double-buffer, per-channel maps, `sync.RWMutex`-protected models, Roaring Bitmap operations).

## Project Structure

```
├── configs/            YAML configs (dev, release, docker)
├── deployments/        Systemd service, logrotate config
├── internal/
│   ├── controllers/    HTTP handlers (+ tests)
│   ├── di/             Wire dependency injection
│   ├── models/         StatStore, PersonalStatStore, FingerprintRecord, binary serialization (+ tests)
│   ├── providers/      Config, Logger, Router, Cache, Metrics providers (+ tests)
│   ├── services/       StatisticService — double-buffer core (+ tests)
│   ├── statistic/      Scheduler, FileManager, Compressor, ColdStorage (+ tests)
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

~140,000 RPS (P99 = 1.3ms). Cache has no effect on write path.

### GET latency — Mixed load (70% POST, 30% GET)

| Endpoint | Cache OFF P99 | Cache ON P99 |
|---|---|---|
| GET /list | 16.3ms | 14.9ms |
| GET /fingerprints | 53.2ms | 40.6ms |
| GET /fingerprint | 17.4ms | 14.8ms |
| GET /channels | 14.8ms | 14.1ms |
| **Total RPS** | **16,038** | **16,251** |

### GET latency — Read-heavy load (10% POST, 90% GET)

| Endpoint | Cache OFF P99 | Cache ON P99 |
|---|---|---|
| GET /list | 32.5ms | 28.6ms |
| GET /fingerprints | 68.5ms | 67.8ms |
| GET /fingerprint | 32.6ms | 27.9ms |
| GET /channels | 27.0ms | 26.1ms |
| **Total RPS** | **5,966** | **5,822** |

### What's new in v1.3.0 vs v1.2.x

**Memory-efficient data structures:**
- `StatStore` replaces `Statistic` — `map[uint32]StatRecord` with values stored inline (not pointers), ~63% memory savings per record
- `FingerprintRecord` with Roaring Bitmaps — first view/click sets only a bitmap bit, repeated interactions promote to sparse `counts` map, ~96% memory reduction per fingerprint
- `PersonalStatStore` replaces `PersonalStats` — manages fingerprints with TTL eviction and cold storage integration

**Bounded memory with eviction:**
- `statistic.maxRecords` — cap on trending records per channel, evicts least-viewed entries
- `statistic.maxRecordsPerFingerprint` — cap on content IDs per fingerprint
- `statistic.maxChannels` — now configurable (was hardcoded at 1000)
- `statistic.evictionPercent` — configurable batch size for eviction (default 10%)

**Fingerprint TTL + cold storage:**
- `statistic.fingerprintTTL` — inactive fingerprints are evicted from memory after TTL
- Evicted fingerprints are saved to cold storage (`{channel}.cold.zst` files) and automatically restored on next interaction
- `statistic.coldTTL` — permanent cleanup of old cold entries
- Write-behind design: `Evict()` buffers in memory, `Flush()` writes to disk atomically

**V5 binary persistence:**
- New binary format with Roaring Bitmap serialization for faster save/restore
- Automatic migration from V1/V2/V3/V4 JSON formats on startup
- `fsync` before rename for crash safety

**Environment variables:**
- All new config fields are now overridable via env vars: `SSD_MAX_CHANNELS`, `SSD_MAX_RECORDS`, `SSD_EVICTION_PERCENT`, `SSD_MAX_RECORDS_PER_FP`, `SSD_FINGERPRINT_TTL`, `SSD_COLD_STORAGE_DIR`, `SSD_COLD_TTL`

**New dependency:** `github.com/RoaringBitmap/roaring` for compact bitmap storage

**Performance comparison (Phase 3, cache OFF):**

| Metric | v1.2.x | v1.3.0 | Change |
|---|---|---|---|
| POST seeding RPS | 154,000 | 140,013 | **-9%** |
| GET /list P99 | 30.5ms | 32.5ms | +7% |
| GET /fingerprints P99 | 60.2ms | 68.5ms | +14% |
| GET /fingerprint P99 | 29.5ms | 32.6ms | +11% |
| GET /channels P99 | 27.0ms | 27.0ms | 0% |
| Total RPS (Phase 3) | 5,970 | 5,966 | ~0% |

The small P99 regression (~7-14%) is expected: `StatStore` uses read-modify-write (two map accesses instead of pointer mutation), and `FingerprintRecord.GetData()` reconstructs `map[int]*StatRecord` from bitmaps + sparse counts on each read. With response cache enabled, the regression is fully absorbed (cache ON P99 is within 1-2% of v1.2.x). The trade-off is ~63% less memory per stat record and ~96% less memory per fingerprint, plus bounded memory via eviction.

**27 files changed, +3,804 / -81 lines. Test count: 142 → 253.**

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
