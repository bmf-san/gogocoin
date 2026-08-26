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

## Walk-forward

`walkforward` slices the data into rolling `[train, test]` windows, picks the
best grid combination on each train window, and reports the results of the
following test window only. Those out-of-sample numbers are the only ones that
say anything about future performance — a grid ranking on its own is fitted to
the data it was ranked on.

The date range comes from `--from` / `--to`, falling back to `data.from` /
`data.to`, falling back to the actual extent of the dataset. Leaving all of them
unset is the normal case: the command then covers every bar available, and there
is no hardcoded date to go stale as new data arrives.

Two files are written to `--out`:

| File | Contents |
| --- | --- |
| `windows.csv` | One row per window, with the selected params and both train and test metrics. |
| `aggregate.csv` | `windows,trades,win_rate,net_pnl` for the test windows combined. Intended for automation that must gate on out-of-sample results. |

## Notes

- Uses the same strategy registry in `pkg/strategy`.
- `scalping` is registered via blank import in `cmd/backtest/main.go`.
- Data source supports SQLite and CSV based on the backtest config.
