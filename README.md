# Simple Statistic Daemon (SSD)

SSD is a high-performance Go daemon for collecting and aggregating real-time content statistics (views, clicks) with fingerprint-based user tracking. Runs as a standalone binary with no external service dependencies. Data is persisted to disk as Zstd-compressed JSON.

## Features

- **High Performance** — double-buffer pattern for lock-free writes, up to ~164,000 req/sec
- **Zero Dependencies** — standalone binary, no databases or message queues required
- **Trending Algorithm** — automatic time-decay: when views exceed 512, values are halved and the factor counter increments, enabling trending CTR calculation
- **Fingerprint Tracking** — per-user statistics grouped by fingerprint
- **Crash-Safe Persistence** — atomic file writes with Zstd compression
- **Graceful Shutdown** — handles SIGINT/SIGTERM, persists data before exit
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

### POST `/` — Submit Statistics

Record views and clicks for content items.

**Request:**
```json
{
  "v": ["105318", "58440"],
  "c": ["58440"],
  "f": "1035ed17aa899a3846b91b57021c2b4f"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `v` | `string[]` | IDs of viewed content |
| `c` | `string[]` | IDs of clicked content |
| `f` | `string` | User fingerprint |

**Response:** `201 Created`

### GET `/list` — Get Aggregated Statistics

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

### GET `/fingerprints` — Get Statistics by Fingerprint

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

### GET `/fingerprint?f={id}` — Get Statistics for a Fingerprint

Returns statistics for a specific user fingerprint.

**Response:** `200 OK`
```json
{
  "105318": { "Views": 1, "Clicks": 0, "Ftr": 0 }
}
```

## Configuration

### YAML Config

```yaml
pidFile: "/tmp/ssd.pid"
statistic:
  interval: 60
webServer:
  host: "0.0.0.0"
  port: 8090
persistence:
  filePath: "/data/ssd/data.bin"
  saveInterval: 120
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

### Environment Variables (Docker)

Environment variables override YAML config values. Configure via `.env` file or `docker compose` environment:

| Variable | Overrides | Default |
|----------|-----------|---------|
| `SSD_PORT` | Host port mapping | `8090` |
| `SSD_DATA_DIR` | Data directory on host | `./data` |
| `SSD_LOGS_DIR` | Logs directory on host | `./logs` |
| `SSD_LOG_LEVEL` | `logger.level` | `info` |
| `SSD_AGGREGATION_INTERVAL` | `statistic.interval` | `60` |
| `SSD_SAVE_INTERVAL` | `persistence.saveInterval` | `120` |

## Architecture

```
HTTP Request → Router (method check) → ApiController → StatisticService (double-buffer) → Models
                                                              ↓
                                          Scheduler (periodic aggregation + persistence)
                                                              ↓
                                          FileManager → Zstd Compressor → Disk
```

- **Double-Buffering** — the active buffer receives incoming stats while the inactive buffer is processed during aggregation, swapped atomically via mutex
- **Trending Decay** — when views exceed 512, values are halved and `Ftr` increments, naturally decaying old content
- **Atomic Persistence** — writes to a temp file, syncs to disk, then renames for crash safety
- **Dependency Injection** — Google Wire for automatic wiring

## Project Structure

```
├── configs/            YAML configs (dev, release, docker)
├── deployments/        Systemd service, logrotate config
├── internal/
│   ├── controllers/    HTTP handlers
│   ├── di/             Wire dependency injection
│   ├── models/         Data models (Statistic, PersonalStats, StatRecord)
│   ├── providers/      Config, Logger, Router providers
│   ├── services/       StatisticService (double-buffer core)
│   ├── statistic/      Scheduler, FileManager, Zstd compressor
│   └── structures/     Config schema, CLI flags, Route definitions
├── scripts/            Post-install script
├── Dockerfile          Multi-stage build
├── docker-compose.yml  Docker Compose setup
└── .env.sample         Environment config template
```

## Contributing

Contributions are welcome. Feel free to submit a pull request or open an issue.

## License

SSD is licensed under the Apache License 2.0. See the [LICENSE](./LICENSE) file for details.
