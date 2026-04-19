# Trading Strategy Reference

## Pluggable Strategy Architecture

gogocoin's strategy system is **pluggable** — implement the `pkg/strategy.Strategy` interface to inject your own trading strategy.

- How to implement a custom strategy: [DESIGN_DOC.md § 5](DESIGN_DOC.md)
- Strategy interface definition: `pkg/strategy/strategy.go`

## Bundled Strategies

| Strategy Name | Package | Description |
|---|---|---|
| `scalping` | `pkg/strategy/scalping` | Minimal EMA crossover scalping strategy (reference implementation — no RSI/cooldown/daily-limit) |

For detailed configuration parameters and signal generation logic for each strategy, refer to the README in the respective package:

- [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md)

## Engine-Level Stop Loss

The stop loss is enforced by `StrategyWorker` on every tick, independently of the strategy's signal output.

On each tick, after the strategy generates a signal, `StrategyWorker` queries open BUY positions from the DB. If the current price has fallen to or below the stop price of any position, a `SELL` signal is injected regardless of what the strategy returned.

```
stop_price = entry_price × (1 - stop_loss_pct / 100)
```

This ensures the stop loss fires even when the strategy returns `HOLD` or `BUY`, closing the losing position immediately when the threshold is crossed.

The `stop_loss_pct` value is read from the strategy config (`strategy_params.scalping.stop_loss_pct` in `config.yaml`). Setting it to `0` disables the stop loss check.

## Disclaimer

Actual trading results vary greatly depending on market conditions and configuration. Past backtesting results do not guarantee future performance.
