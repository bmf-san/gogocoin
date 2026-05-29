# Backtest

Backtesting in gogocoin is implemented in a reusable package and CLI:

- Core package: `pkg/backtest`
- CLI command: `cmd/backtest`

## Quick Start

```bash
# Single scenario
make backtest

# Grid optimization
make backtest-grid BACKTEST_SCENARIO=scalping_xrp_grid

# Walk-forward analysis
make backtest-walkforward BACKTEST_SCENARIO=scalping_xrp_grid
```

Default config is `configs/backtest.yaml`.

## CLI

```bash
go run ./cmd/backtest run -h
go run ./cmd/backtest optimize -h
go run ./cmd/backtest walkforward -h
go run ./cmd/backtest compare -h
```

## Notes

- Uses the same strategy registry in `pkg/strategy`.
- `scalping` is registered via blank import in `cmd/backtest/main.go`.
- Data source supports SQLite and CSV based on the backtest config.
