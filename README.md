<p align="center">
  <br>
  <code>
  ██╗    ██╗ █████╗ ██╗    ██╗ █████╗ ██╗
  ██║    ██║██╔══██╗██║    ██║██╔══██╗██║
  ██║ █╗ ██║███████║██║ █╗ ██║███████║██║
  ╚██████╔╝██║  ██║╚██████╔╝██║  ██║██║
   ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝
  </code>
  <br><br>
  <strong>ส่งไว รับไว ไม่ง้อ TCP</strong>
  <br>
  Blazing-fast file transfer powered by QUIC
  <br><br>
  <a href="https://github.com/killernay/waiwai/actions/workflows/ci.yml"><img src="https://github.com/killernay/waiwai/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/killernay/waiwai/releases"><img src="https://img.shields.io/github/v/release/killernay/waiwai?include_prereleases&style=flat-square" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/killernay/waiwai"><img src="https://goreportcard.com/badge/github.com/killernay/waiwai?style=flat-square" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/killernay/waiwai"><img src="https://pkg.go.dev/badge/github.com/killernay/waiwai.svg" alt="Go Reference"></a>
</p>

---

## Why waiwai?

TCP-based tools like `scp` and `rsync` are bottlenecked by a single stream and suffer badly on high-latency or lossy links. **waiwai** uses [QUIC](https://www.rfc-editor.org/rfc/rfc9000.html) to multiplex 8–32 parallel streams over UDP, saturating the pipe even on transcontinental WAN links.

| Scenario | scp | waiwai (16 streams) | Speedup |
|---|---|---|---|
| LAN (< 1 ms RTT) | ~900 MB/s | ~900 MB/s | 1x |
| WAN 150 ms RTT | ~20 MB/s | ~150 MB/s | **7.5x** |
| WAN 200 ms + 1 % loss | ~5 MB/s | ~80 MB/s | **16x** |

## Features

| | Feature | Description |
|---|---|---|
| **Multi-stream QUIC** | 8–32 parallel streams over a single connection |
| **Resume** | Disconnect mid-transfer? Reconnect and pick up where you left off |
| **Multi-file & directory** | Send entire directory trees in one command |
| **Bandwidth throttle** | `--rate 50` caps at 50 MB/s — be nice to your network |
| **Live monitoring** | JSON `/status` + Prometheus `/metrics` on both sides |
| **TLS built-in** | Every byte encrypted via QUIC's mandatory TLS 1.3 |
| **Zero config** | Auto-generated self-signed cert — just run and send |
| **Cross-platform** | Linux, macOS, Windows — single static binary |

## Quick Start

```bash
# Install
go install github.com/killernay/waiwai/cmd/waiwai@latest
```

**Receiver** (run first on the destination machine):

```bash
waiwai recv --out /data/incoming
```

**Sender** (run on the source machine):

```bash
waiwai send ./footage/ 100.64.1.2:4242
```

That's it. Files land in `/data/incoming`.

## Usage

### Send

```bash
waiwai send [flags] <file|directory>... <host:port>
```

| Flag | Default | Description |
|---|---|---|
| `-s, --streams` | `8` | Number of parallel QUIC streams |
| `-r, --rate` | `0` (unlimited) | Bandwidth limit in MB/s |
| `--resume` | | Session ID to resume a previous transfer |
| `--monitor` | | Start metrics server (e.g. `:9090`) |
| `--cert` | | TLS certificate file (optional) |
| `--key` | | TLS private key file (optional) |

```bash
# Send a single file
waiwai send photo.jpg 100.64.1.2:4242

# Send a directory with 16 streams
waiwai send -s 16 ./footage/ 100.64.1.2:4242

# Limit to 50 MB/s
waiwai send --rate 50 bigfile.bin 100.64.1.2:4242

# Resume an interrupted transfer
waiwai send --resume myjob bigfile.bin 100.64.1.2:4242

# Enable monitoring
waiwai send --monitor :9090 ./footage/ 100.64.1.2:4242
```

### Receive

```bash
waiwai recv [flags]
```

| Flag | Default | Description |
|---|---|---|
| `-l, --listen` | `0.0.0.0:4242` | Address to listen on |
| `-o, --out` | `.` | Output directory |
| `--monitor` | | Start metrics server (e.g. `:9091`) |
| `--cert` | | TLS certificate file (optional) |
| `--key` | | TLS private key file (optional) |

```bash
# Listen with defaults
waiwai recv

# Custom port and output directory
waiwai recv --listen 0.0.0.0:5000 --out /data/incoming

# Enable monitoring on receiver side
waiwai recv --out /data --monitor :9091
```

### Status

Query a running transfer's metrics in real time:

```bash
waiwai status localhost:9090
```

```
⚡ waiwai status @ 100.64.1.2:9090
  [████████████████░░░░░░░░░░░░░░] 53.2%
  ความเร็ว:   142.50 MB/s
  ส่งแล้ว:    5324.10 MB
  ไฟล์:       3 / 7
  Streams:    16 active
  เหลืออีก:   33s
```

## Monitoring

Both sender and receiver expose HTTP endpoints when started with `--monitor`:

| Endpoint | Format | Description |
|---|---|---|
| `GET /status` | JSON | Human-friendly snapshot |
| `GET /metrics` | Prometheus text | Scrapeable by Prometheus/Grafana |
| `GET /health` | Plain text | Health check (`ok`) |

## Architecture

```
waiwai/
├── cmd/waiwai/main.go            # CLI entry point (cobra)
├── pkg/protocol/protocol.go      # Wire format: control messages + binary chunks
├── internal/
│   ├── transfer/transfer.go      # Core send/receive engine
│   ├── transfer/tls_gen.go       # Self-signed TLS generation
│   ├── checkpoint/checkpoint.go  # Resume state persistence
│   ├── throttle/throttle.go      # Token-bucket bandwidth limiter
│   ├── monitor/monitor.go        # Metrics HTTP server
│   └── ui/ui.go                  # Terminal progress bars
├── .goreleaser.yaml              # Cross-platform release pipeline
└── .github/workflows/
    ├── ci.yml                    # Test on every PR
    └── release.yml               # Build + release on tag push
```

### Protocol Design

```
Stream 0   ─── Control (length-prefixed JSON) ───────────────────
                 hello → accept → file_info → file_ack → ... → done

Stream 1+  ─── Data (binary chunk headers + payload) ────────────
                 [fileID:2][chunkIdx:8][dataLen:4][sha256:32][data...]
```

- **Control stream** carries handshake, file metadata, and acks as JSON envelopes
- **Data streams** (1–32) carry file chunks with compact 46-byte binary headers
- Each chunk is individually SHA-256 verified for integrity
- Resume works by tracking received chunks in a checkpoint file

## Build from Source

```bash
# Build
make build                  # → bin/waiwai

# Run tests
make test                   # go test ./... -race

# Lint
make lint                   # golangci-lint

# Local snapshot release (all platforms)
make snapshot               # goreleaser --snapshot

# Create a release tag
make tag                    # prompts for version, pushes tag
```

## Requirements

- **Go 1.22+** (for building from source)
- **UDP connectivity** between sender and receiver on port 4242 (default)

> Works great with [Tailscale](https://tailscale.com) — no port forwarding needed.

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.

1. Fork the repo
2. Create your feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

## License

[MIT](LICENSE) — use it however you want.
