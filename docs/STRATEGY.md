# Strategy Reference

## Pluggable Strategy Architecture

gogocoin uses a **pluggable** strategy design. Any type that implements the `pkg/strategy.Strategy` interface can be registered and used.

- How to implement a custom strategy: [DESIGN_DOC.md § 5](DESIGN_DOC.md)
- Strategy interface definition: `pkg/strategy/strategy.go`

## Built-in Strategies

| Name | Package | Description |
|---|---|---|
| `scalping` | `pkg/strategy/scalping` | EMA crossover + optional RSI filter scalping strategy |

For configuration parameters and signal generation details, see the package README:

- [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md)

## Engine-level Stop Loss

Stop-loss is enforced by `StrategyWorker` on every market tick, independent of the strategy's signal output.

On each tick, after the strategy generates a signal, `StrategyWorker` queries open BUY positions from the database. If the current price has fallen at or below the stop price for any open position, a `SELL` signal is injected regardless of what the strategy returned.

```
stop_price = entry_price × (1 - stop_loss_pct / 100)
```

This means stop-loss fires even when the strategy outputs `HOLD` or `BUY`, cutting losing positions as soon as the threshold is breached.

The `stop_loss_pct` value is read from the strategy's configuration (set via `strategy_params.scalping.stop_loss_pct` in `config.yaml`). Setting it to `0` disables the stop-loss check.

## Disclaimer

Actual trading results vary significantly depending on market conditions and configuration. Past backtest results do not guarantee future performance.
