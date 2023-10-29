# Simple Statistic Daemon (SSD)
SSD is a high-performance utility designed to capture and aggregate trending statistics from your content effortlessly.

## Features
- **High Performance**: Crafted with Go for maximum speed. Our benchmarks show up to ~164,000 requests/sec.
- **User-friendly**: No external dependencies; SSD operates as a standalone binary. It's as simple as download and execute.

## Installation
1. Download and install **goreleaser** from [here](https://goreleaser.com/install/).
2. Clone the repository:
    ```bash
    git clone <repository_url>
    ```
3. Enter the project directory:
    ```bash
    cd ssd
    ```
4. Build the project:
    ```bash
    go get -u && go vet && git tag -f v1.0.0 && goreleaser --rm-dist --skip-publish --skip-validate
    ```
5. Run the binary tailored for your platform from the `dist` directory.

## Usage
Execute the application to use the default configuration. If needed, customize the configuration file to fit your specific use case.

## Documentation
### Endpoints

| Action        | Description  | URL     | Method | Payload/Parameters | Response           | Notes |
|---------------|--------------|---------|--------|--------------------|--------------------|-------|
| Add statistic | Record stats to the in-memory DB. When views for an item exceed 512, the system divides views and clicks by 2, incrementing the division counter, Ftr, by 1. This allows for trending CTR calculation or full views and clicks reproduction. | `/` | POST   | `{"v": ["105318","58440"],"c": ["58440"],"f": "1035ed17aa899a3846b91b57021c2b4f"}` | Status: 201 Created | `v`: IDs of viewed content<br/>`c`: IDs of clicked content<br/>`f`: User fingerprint |
| Get statistic | Retrieve stats from the in-memory DB. | `/list` | GET | - | Status: 200 | JSON response with content ID as key. Includes trending clicks, views, and applied divisions. |

### Configuration
```yaml
pidFile: "/tmp/ssd.pid"
statistic:
  interval: 60
webServer:
  host: "127.0.0.1"
  port: 8090
persistence:
  filePath: "/etc/ssd/data.bin"
  saveInterval: 600
logger:
  level: "info"
  mode: 0640
  dir: "/var/log/ssd"
```
### Configuration Parameters

| Parameter               | Description                           | Default           |
|-------------------------|---------------------------------------|-------------------|
| `pidFile`               | PID file path                         | `/tmp/ssd.pid`    |
| `statistic.interval`    | Stats aggregation interval (sec)      | `60`              |
| `webServer.host`        | Web server host                       | `127.0.0.1`       |
| `webServer.port`        | Web server port                       | `8090`            |
| `persistence.filePath`  | Data storage path                     | `/etc/ssd/data.bin`|
| `persistence.saveInterval`| Data saving interval (sec)           | `600`             |
| `logger.level`          | Logging level                         | `info`            |
| `logger.mode`           | Log file mode                         | `0640`            |
| `logger.dir`            | Log files directory                   | `/var/log/ssd`    |

### Contributing

We welcome contributions! If you come across any issues or have suggestions, feel free to submit a pull request or open an issue.

### License

SSD is licensed under the Apache License 2.0. Please see the [LICENSE](./LICENSE) file for more information.

