# Scalping Strategy

Stateless EMA-based scalping strategy. The default strategy shipped with gogocoin.

---

## Features

| Feature | Detail |
|---|---|
| Design | Stateless — minimal internal state, restart-safe |
| Indicator | Short EMA / long EMA crossover |
| RSI filter | Optional. Enabled when `rsi_period > 0` |
| Risk/reward | Default 2:1 (take-profit 2.0% / stop-loss 1.0%) |
| Cooldown | Configurable interval between entries (default 90 s) |
| Fee-aware | Fee included in P&L calculations |
| Stop loss | `stop_loss_pct` is enforced at the engine level on every market tick, independent of the EMA signal — see [STRATEGY.md § Engine-level Stop Loss](../docs/STRATEGY.md) |

---

## Signal Generation Logic

| Signal | Condition |
|---|---|
| BUY | short EMA > long EMA **and** price > short EMA |
| SELL | short EMA < long EMA **and** price < short EMA |
| HOLD | none of the above, or in cooldown, or daily limit reached |

> **Note:** The engine (`StrategyWorker`) overrides HOLD/BUY with a SELL signal when an open position breaches its stop-loss price, regardless of EMA state. See [STRATEGY.md § Engine-level Stop Loss](../docs/STRATEGY.md).

### RSI Filter (optional)

When `rsi_period` is set to a non-zero value, the RSI filter activates. In addition to the EMA conditions above, the following RSI conditions must also be met:

| Signal | RSI condition |
|---|---|
| BUY | RSI < `rsi_overbought` (not overbought) |
| SELL | RSI > `rsi_oversold` (not oversold) |

---

## Configuration Parameters (`strategy_params.scalping`)

Configured in the `strategy_params.scalping` block of `config.yaml`.

| Key | Default | Description |
|---|---|---|
| `ema_fast_period` | `9` | Short EMA period (bars) |
| `ema_slow_period` | `21` | Long EMA period (bars) |
| `take_profit_pct` | `2.0` | Take-profit threshold (%) |
| `stop_loss_pct` | `1.0` | Stop-loss threshold (%). Enforced by the engine on every tick. Set to `0` to disable. |
| `cooldown_sec` | `90` | Cooldown between entries (seconds). Prevents over-trading |
| `max_daily_trades` | `3` | Maximum trades per day (conservative default) |
| `order_notional` | `200` | **Order size in JPY — this is the actual trade amount** |
| `auto_scale_enabled` | `false` | When `true`, scales `order_notional` up automatically based on available balance |
| `auto_scale_balance_pct` | `80` | Target fraction of JPY balance when auto-scaling (`balance × pct%`) |
| `auto_scale_max_notional` | `0` | Cap on auto-scaled order size. `0` = no cap |
| `fee_rate` | `0.001` | Fee rate used in P&L calculations |
| `rsi_period` | `0` | RSI period. `0` disables the RSI filter |
| `rsi_overbought` | `70` | RSI overbought threshold — suppresses BUY above this level |
| `rsi_oversold` | `30` | RSI oversold threshold — suppresses SELL below this level |

### `order_notional`

`order_notional` sets the **order size in JPY** per trade.
The strategy calculates quantity as `order_notional / current_price`.
There is no automatic percentage-of-balance sizing without `auto_scale_enabled`.

Expected profit per trade: `order_notional × take_profit_pct / 100 − round-trip fees`

Example (`order_notional: 4000`, `take_profit_pct: 2.0`, `fee_rate: 0.0015`):
- Gross profit at take-profit: 4000 × 0.02 = 80 JPY
- Round-trip fees: 4000 × 0.0015 × 2 = 12 JPY
- Net profit: ~68 JPY

### Auto-scaling (`auto_scale_*`)

When `auto_scale_enabled: true`, the order size is recalculated on each BUY signal:

- Base: `order_notional` (or `symbol_params.<symbol>.order_notional`)
- Target: `JPY balance × auto_scale_balance_pct / 100`
- Effective size: `max(base, target)`, clamped by `auto_scale_max_notional` and affordable balance (after fees)

This allows order sizes to grow as the balance increases, compounding gains without manually updating `order_notional`.

### Per-symbol overrides (`symbol_params`)

Individual parameters can be overridden per currency pair. `0` or unset falls back to the global value.

| Key | Description |
|---|---|
| `ema_fast_period` | Short EMA period |
| `ema_slow_period` | Long EMA period |
| `cooldown_sec` | Cooldown in seconds |
| `order_notional` | Order size in JPY |

---

## Configuration Example

```yaml
strategy_params:
  scalping:
    ema_fast_period: 9
    ema_slow_period: 21
    take_profit_pct: 2.0
    stop_loss_pct: 1.0
    cooldown_sec: 90
    max_daily_trades: 3
    order_notional: 4000
    auto_scale_enabled: true
    auto_scale_balance_pct: 80
    auto_scale_max_notional: 20000
    fee_rate: 0.0015
    rsi_period: 14
    rsi_overbought: 70
    rsi_oversold: 30
    symbol_params:
      XLM_JPY:
        order_notional: 4000
        cooldown_sec: 120
```

---

## Tuning Guide

### EMA periods

`ema_fast_period` must always be less than `ema_slow_period`.

- Decrease `ema_fast_period` → faster reaction (more noise)
- Increase `ema_slow_period` → more trend-following, fewer signals

### Take-profit and stop-loss

Recommended risk/reward ratio ≥ 2:1 (`take_profit_pct >= stop_loss_pct * 2`).

Break-even win rate (ignoring fees): `1 / (1 + take_profit_pct / stop_loss_pct)`

With 0.15% round-trip fee, the required win rate to break even is:
`stop_loss_pct / (take_profit_pct + stop_loss_pct) + fee_overhead`

Example (take 2%, stop 1%): break-even win rate ≈ 43%.

### Order size and expected profit

| `order_notional` | Gross at 2% take-profit | Round-trip fees (0.15%) | Net profit |
|---|---|---|---|
| 200 | 4 JPY | 0.6 JPY | ~3 JPY |
| 1000 | 20 JPY | 3 JPY | ~17 JPY |
| 4000 | 80 JPY | 12 JPY | ~68 JPY |
| 7000 | 140 JPY | 21 JPY | ~119 JPY |

### Trade frequency

Increasing `max_daily_trades` and decreasing `cooldown_sec` raises trade frequency but also increases total fee cost. Always recalculate the break-even win rate before tightening cooldown.

With a short cooldown (e.g. 60 s), the engine's stop-loss can trigger a SELL just seconds after a BUY if the price immediately reverses. A longer cooldown (e.g. 300 s) is recommended for noisy, range-bound markets.

---

## Recommended Settings

| Item | Recommended | Reason |
|---|---|---|
| Symbol | XRP_JPY | Low minimum order size (1 XRP) |
| Operation | 24/7 | Captures signals at any hour |
| Trade frequency | Tune via `max_daily_trades` | Default of 3 is conservative |
