# gogocoin

[![CI](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml)
[![Release](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bmf-san/gogocoin)](https://goreportcard.com/report/github.com/bmf-san/gogocoin)
[![GitHub license](https://img.shields.io/github/license/bmf-san/gogocoin)](https://github.com/bmf-san/gogocoin/blob/main/LICENSE)
[![GitHub release](https://img.shields.io/github/release/bmf-san/gogocoin.svg)](https://github.com/bmf-san/gogocoin/releases)

Automated cryptocurrency trading bot for the bitFlyer exchange.

<img src="./docs/assets/icon.png" alt="gogocoin" title="gogocoin" width="100px">

This logo was created by [gopherize.me](https://gopherize.me/gopher/c3ef0a34f257bb18ea3b9b5a3ada0b1a0573e431).

## Overview

gogocoin is an automated trading bot for the bitFlyer cryptocurrency exchange, written in Go. It executes trades using an EMA-based scalping strategy with configurable trade frequency.

### Features

- **Pluggable strategy architecture**: Implement the `pkg/strategy.Strategy` interface to plug in your own trading strategy
- Bundled default strategy: EMA crossover + RSI filter scalping strategy
- Risk management (take-profit, stop-loss, daily trade limit, cooldown)
- Web UI for starting and stopping trading
- Real-time market data ingestion and analysis via WebSocket
- Real-time monitoring dashboard (`http://localhost:8080`)
- Data persistence with SQLite
- Automatic trade data cleanup (configurable via `retention_days`)
- Structured logging with level and category filtering
- 24/7 operation support (idempotent, restart-safe)

### Screenshot

![gogocoin Dashboard](./docs/assets/screenshot-dashboard.png)

### Tech Stack

- **Language**: Go 1.23+ (development: Go 1.25.0)
- **Dependencies**: Minimal (go-bitflyer-api-client + yaml.v3 + sqlite3 only)
- **Architecture**: Layered modular architecture
- **Public API** (`pkg/`): `pkg/engine.Run()` + `pkg/strategy.Strategy` interface allow strategy injection from external repositories. Stable, semantically versioned API
- **Database**: SQLite (lightweight, embedded, no external DB required)
  - Retention: configurable via `retention_days` (example default: 90 days; the code falls back to 1 day when unset)
  - Historical data: accessible via bitFlyer
- **Concurrency**: Asynchronous workers via Goroutines + Channels
- **Transport**: WebSocket (real-time) + REST API (Web UI)
- **Logging**: Structured logging based on the standard `log/slog` package
  - High-frequency log filtering (DEBUG level and `data` category)
  - DB index optimization (`timestamp DESC`)
- **Performance optimizations**:
  - Balance cache (60s TTL, ~90% reduction in API calls)
  - ~98% reduction in 429 errors
  - Deadlock-safe design
- **Deployment**: Single binary with embedded web assets
- **Quality assurance**:
  - Static analysis with golangci-lint
  - Unit tests across multiple packages
  - Layered modular architecture
  - Type safety via Go's type system
  - Proper error handling

## Disclaimer

**Important: Please read carefully.**

**This software is provided for informational and development purposes only and does not constitute financial advice or investment recommendations. Cryptocurrency trading carries significant risk and you may lose your entire investment.**

**Actual trading results vary greatly depending on market conditions, configuration, and timing. Past backtesting or simulation results do not guarantee future performance.**

**The author accepts no responsibility for any losses or damages arising from the use of this software. Use it at your own discretion and risk.**

**This library is not affiliated with bitFlyer in any way. Please review each API provider's terms of service before use.**

**This library is provided "as is" with no warranties regarding accuracy, completeness, or future compatibility.**

## Quick Start

gogocoin can be used in two ways.

### A. Use as a library (recommended)

Install gogocoin via `go get` and integrate it into your own repository. You can implement and plug in your own trading strategy.

```bash
go get github.com/bmf-san/gogocoin@latest
```

A working sample is available in the `example/` directory. See [Using the example directory](#using-the-example-directory) for details.

### B. Try quickly with Docker

The `example/` directory includes a `Dockerfile` and `docker-compose.yml` that build a fully working binary with the bundled EMA+RSI scalping strategy registered.

#### Prerequisites

- Docker and Docker Compose
- bitFlyer API key (obtain from the [API settings page](https://bitflyer.com/en-jp/api))

#### Setup

```bash
# 1. Clone the repository
git clone https://github.com/bmf-san/gogocoin.git
cd gogocoin/example

# 2. Create the config file
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your API keys

# 3. Start (build context is the repo root, so run from example/)
make up

# 4. Open the Web UI
open http://localhost:8080
```

**⚠️ Warning**: This bot supports live trading only. It uses real funds — review your configuration carefully before use.

#### Container management

```bash
make logs     # View logs
make down     # Stop
make restart  # Restart
make rebuild  # Rebuild
```

## Using the example directory

`example/` is a fully working sample showing how to use gogocoin as a library. It serves as a starting point for building your own repository.

### Structure

```
example/
├── cmd/
│   └── main.go                  # Entry point (registers strategy via blank import)
├── strategy/scalping/
│   ├── params.go                # Strategy parameter definitions
│   ├── strategy.go              # Strategy implementation (EMA + RSI + cooldown)
│   └── register.go              # Auto-registration via init()
├── configs/
│   └── config.example.yaml      # Config file template
├── go.mod                       # Independent Go module
├── Makefile                     # build / run / Docker shortcuts
├── Dockerfile                   # Docker image (build context: repo root)
└── docker-compose.yml           # Docker Compose config
```

### Running the example

**With Docker (simplest):**

```bash
cd example
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your API keys
make up
```

**Without Docker:**

```bash
cd example

# 1. Create the config file
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your API keys

# 2. Run
export BITFLYER_API_KEY=your_key
export BITFLYER_API_SECRET=your_secret
make run
# or: go run ./cmd/
```

### Adapting to your own repository

Copy `example/` as-is to use as your own repository, or follow the pattern below.

**1. Create `go.mod`**

```bash
go mod init github.com/yourname/your-bot
go get github.com/bmf-san/gogocoin@latest
```

**2. Implement your strategy and register it via `init()`**

```go
// strategy/scalping/register.go
package scalping

import "github.com/bmf-san/gogocoin/pkg/strategy"

func init() {
    strategy.Register("scalping", func() strategy.Strategy {
        return NewDefault()
    })
}
```

**3. Blank import in `main.go`**

```go
import (
    "github.com/bmf-san/gogocoin/pkg/engine"
    _ "github.com/yourname/your-bot/strategy/scalping" // triggers init()
)

func main() {
    engine.Run(ctx, engine.WithConfigPath("./configs/config.yaml"))
}
```

> Reference implementation: [bmf-san/my-gogocoin](https://github.com/bmf-san/my-gogocoin)

## Documentation

| Document | Description |
|---|---|
| [docs/CONFIG.md](docs/CONFIG.md) | Configuration reference |
| [docs/STRATEGY.md](docs/STRATEGY.md) | Trading strategy reference (pluggable architecture overview and bundled strategies) |
| [docs/DESIGN_DOC.md](docs/DESIGN_DOC.md) | Architecture design document (**how to implement a custom strategy** § 5) |
| [docs/DATA_MANAGEMENT.md](docs/DATA_MANAGEMENT.md) | Data management reference |
| [docs/openapi.yaml](docs/openapi.yaml) | API specification (OpenAPI 3.1) |

## Web UI

Monitor trading activity in real time in your browser: `http://localhost:8080`

You can also start and stop trading from the Web UI.

## Operations

### Recommended practices

1. Persist `./data/` via a Docker volume (already configured)
2. Restart roughly once a week for stability
3. Use log level `info` in production (`debug` for development only)

### Troubleshooting

- View logs: `make logs` or `docker compose logs -f`
- Check DB: `ls -lh ./data/gogocoin.db`
- Restart container: `make restart`

## Development

### Local development

```bash
# Install dependencies
make deps

# Install dev tools (golangci-lint, oapi-codegen, etc.)
make install-tools

# Run tests
make test

# Check coverage
make test-coverage

# Format code
make fmt

# Run linter
make lint

# Run via Docker (from example/ directory)
# cd example && make up
```

### API code generation

When you modify `docs/openapi.yaml`, regenerate the code with `oapi-codegen` and commit it.

```bash
# Regenerate api.gen.go
make generate

```

> `internal/api/api.gen.go` is an auto-generated file. Do not edit it directly — always update it via `make generate`.
> The CI `codegen` job verifies that the spec and generated code are in sync.

## Related

- [example/](example/) — Working sample for using gogocoin as a library (in this repository)
- [bmf-san/my-gogocoin](https://github.com/bmf-san/my-gogocoin) — Example production repository using gogocoin
- [gogocoin-vps-template](https://github.com/bmf-san/gogocoin-vps-template) — Template for deploying to a VPS (ConoHa, etc.) with systemd + GitHub Actions

## Contributing

See [CONTRIBUTING.md](.github/CONTRIBUTING.md).
