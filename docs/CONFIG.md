# Configuration Reference

The configuration file is managed at `configs/config.yaml` (copy from `configs/config.example.yaml`).
API keys are set as environment variables in the `.env` file.

## API Keys (.env)

```bash
BITFLYER_API_KEY=your_api_key_here
BITFLYER_API_SECRET=your_api_secret_here
```

---

## app

| Key | Default | Description |
|---|---|---|
| `app.name` | `"gogocoin"` | Application name |

---

## api

| Key | Default | Description |
|---|---|---|
| `api.endpoint` | `https://api.bitflyer.com` | bitFlyer REST API endpoint |
| `api.websocket_endpoint` | `wss://ws.lightstream.bitflyer.com/json-rpc` | bitFlyer WebSocket endpoint |
| `api.credentials.api_key` | `${BITFLYER_API_KEY}` | API key (injected from environment variable) |
| `api.credentials.api_secret` | `${BITFLYER_API_SECRET}` | API secret (injected from environment variable) |
| `api.timeout` | `30s` | API request timeout |
| `api.retry_count` | `3` | Number of retries |
| `api.rate_limit.requests_per_minute` | `50` | Maximum API requests per minute |

---

## trading

| Key | Default | Description |
|---|---|---|
| `trading.initial_balance` | `1000` | Initial balance (JPY). Used as base for risk calculations |
| `trading.fee_rate` | `0.0015` | Trading fee rate (0.15%) |
| `trading.symbols` | `["XRP_JPY"]` | Trading pairs. XRP_JPY is recommended for small-amount trading |
| `trading.strategy.name` | `"scalping"` | Name of the strategy to use |

> Config validation at startup is fail-fast. Specifying an unknown symbol in `trading.symbols` will cause an error and prevent startup.

### trading.risk_management

| Key | Default | Description |
|---|---|---|
| `max_total_loss_percent` | `50.0` | Maximum cumulative loss limit (% of initial balance). Trading stops when exceeded |
| `max_trade_loss_percent` | `10.0` | Maximum loss per trade (%) |
| `max_daily_loss_percent` | `30.0` | Daily loss limit (%) |
| `max_trade_amount_percent` | `80.0` | Maximum percentage of balance that can be used in a single trade (%) |
| `max_daily_trades` | `100` | Maximum number of trades per day (risk management upper bound) |
| `min_trade_interval` | `60s` | Minimum interval between trades |
| `pnl_epoch` | _(empty)_ | Optional RFC3339 timestamp. When set, cumulative PnL ignores trades executed before it |

> `max_daily_trades` is the risk management upper bound. Actual trade frequency is controlled by each strategy's own `max_daily_trades`.

> `pnl_epoch` exists for the case where part of the recorded trade history is known to be wrong. The cumulative loss is normally read from the stored performance metrics, which span every trade ever made, so a period of mis-calculated PnL keeps distorting the limit forever. Setting an epoch draws a line under such a period without deleting the trades.

> The epoch applies to everything that reports a cumulative figure, not just the loss limit: the metrics written after each trade, `GET /api/performance`, `GET /api/v1/performance/symbols`, and the dashboard totals derived from them. Performance snapshots recorded before the epoch are withheld from the API rather than rebased, because the per-trade PnL behind them cannot be recomputed. The dashboard labels the affected figures with the cut-off date so the narrower scope is visible. Expect the totals to read ¥0 until the first trade after the epoch settles — that is the honest answer, not a fault.

---

## strategy_params

Strategy-specific parameters are configured under the `strategy_params.<strategy_name>` block.
The config is passed to each strategy via `pkg/strategy.Strategy.Initialize()`.

For the bundled Scalping strategy parameters, see [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md).

```yaml
# Example: bundled scalping strategy
strategy_params:
  scalping:
    ema_fast_period: 9
    auto_scale_enabled: true
    auto_scale_balance_pct: 80
    auto_scale_max_notional: 20000
    # ... see scalping/README.md for full details

# Example: custom strategy
strategy_params:
  mystrategy:
    my_param: 42
```

> `strategy_params.scalping.order_notional` must be set explicitly (no implicit default).

---

## ui

| Key | Default | Description |
|---|---|---|
| `ui.host` | `"0.0.0.0"` | Web UI listen host |
| `ui.port` | `8080` | Web UI port number |

---

## logging

| Key | Default | Description |
|---|---|---|
| `logging.level` | `"info"` | Global log level (`debug` / `info` / `warn` / `error`) |
| `logging.format` | `"json"` | Log format |
| `logging.output` | `"both"` | Output destination (`stdout` / `file` / `both`) |
| `logging.file_path` | `"./logs/gogocoin.log"` | Log file path |
| `logging.max_size_mb` | `50` | Maximum log file size (MB) |
| `logging.max_backups` | `3` | Number of rotated log files to retain |
| `logging.max_age_days` | `7` | Number of days to retain log files |

### logging.categories

Log levels can be configured per category.

| Category | Default | Description |
|---|---|---|
| `trading` | `"debug"` | Trade-related logs |
| `api` | `"info"` | API communication logs |
| `strategy` | `"debug"` | Strategy signal logs |
| `ui` | `"info"` | Web UI / REST API logs |

> `logging.level: "info"` is recommended for production. `debug` generates high-frequency logs and may impact performance.

---

## data_retention

| Key | Default | Description |
|---|---|---|
| `data_retention.retention_days` | `90` (example) | Number of days to retain data in the DB. `1` = current day only (lightest), `90` = last 90 days (keeps full "総損益" window). Code falls back to `1` when unset. |

Data older than `retention_days` is automatically deleted at 00:00 every day. If you need historical trade records, download them from the bitFlyer dashboard.
