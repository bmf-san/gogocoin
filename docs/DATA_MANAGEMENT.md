# Data Management Reference

gogocoin prioritizes a lightweight footprint and retains only a configurable number of days worth of data.

---

## Automatic Cleanup

### Schedule

| Item | Details |
|---|---|
| Run time | Daily at 00:00 (midnight) |
| Retention period | Configurable (example default: 90 days; code fallback: 1 day when unset) |
| Deleted records | Data older than the retention period |

### Cleanup flow (retention_days = 1)

```
00:00 (midnight): Data older than yesterday is automatically deleted
  ↓
Result: DB retains only today's data → lightweight container maintained
```

### Retention period examples

| retention_days | Effect |
|---|---|
| `1` (default) | Retain current day only (lightest) |
| `7` | Retain last 7 days of data |
| `30` | Retain last 30 days of data |

See the `data_retention` section in [docs/CONFIG.md](CONFIG.md) for configuration details.

### Affected tables

The following tables are retained for the number of days set by `retention_days`.

| Table | Contents | Deletion column |
|---|---|---|
| `logs` | Logs (all levels) | `timestamp` |
| `market_data` | Market data | `timestamp` |
| `balances` | Balance snapshots | `timestamp` |
| `trades` | Trade history | `executed_at` |
| `positions` | Closed positions (OPEN positions are retained) | `updated_at` |
| `performance_metrics` | Performance metrics (daily) | `date` |

> The `app_state` table uses fixed-key upserts (2 rows) and is not subject to cleanup.

### Accessing historical data

Deleted trade records can be viewed from the bitFlyer dashboard. Download them from bitFlyer if needed for tax filing or other purposes.

---

## Idempotency Guarantees

The system is safe to restart at any time.

| Feature | Details |
|---|---|
| Duplicate prevention | Date-based execution tracking prevents double-running cleanup |
| Missed cleanup recovery | Any missed cleanup jobs are automatically run on startup |
| State restoration | Trading state is restored from the DB on restart |
