# 設定リファレンス

設定ファイルは `configs/config.yaml`（`configs/config.example.yaml` からコピーして作成）で管理します。
APIキーは `.env` ファイルで環境変数として設定します。

## APIキー（.env）

```bash
BITFLYER_API_KEY=your_api_key_here
BITFLYER_API_SECRET=your_api_secret_here
```

---

## app

| キー | デフォルト | 説明 |
|---|---|---|
| `app.name` | `"gogocoin"` | アプリケーション名 |

---

## api

| キー | デフォルト | 説明 |
|---|---|---|
| `api.endpoint` | `https://api.bitflyer.com` | bitFlyer REST API エンドポイント |
| `api.websocket_endpoint` | `wss://ws.lightstream.bitflyer.com/json-rpc` | bitFlyer WebSocket エンドポイント |
| `api.credentials.api_key` | `${BITFLYER_API_KEY}` | APIキー（環境変数から注入） |
| `api.credentials.api_secret` | `${BITFLYER_API_SECRET}` | APIシークレット（環境変数から注入） |
| `api.timeout` | `30s` | APIリクエストタイムアウト |
| `api.retry_count` | `3` | リトライ回数 |
| `api.rate_limit.requests_per_minute` | `50` | 1分あたりのAPIリクエスト上限 |

---

## trading

| キー | デフォルト | 説明 |
|---|---|---|
| `trading.initial_balance` | `1000` | 初期資金（JPY）。リスク計算の基準値 |
| `trading.fee_rate` | `0.0015` | 取引手数料率（0.15%） |
| `trading.symbols` | `["XRP_JPY"]` | 取引対象の通貨ペア。少額取引には XRP_JPY を推奨 |
| `trading.strategy.name` | `"scalping"` | 使用する戦略名 |

### trading.risk_management

| キー | デフォルト | 説明 |
|---|---|---|
| `max_total_loss_percent` | `50.0` | 累計損失の上限（初期資金に対する%）。超えると取引停止 |
| `max_trade_loss_percent` | `10.0` | 1回の取引での最大損失（%） |
| `max_daily_loss_percent` | `30.0` | 1日の損失上限（%） |
| `max_trade_amount_percent` | `80.0` | 1回の取引で使用できる残高の上限（%） |
| `max_daily_trades` | `100` | 1日の最大取引回数（リスク管理上限） |
| `min_trade_interval` | `60s` | 取引間の最小インターバル |

> `max_daily_trades` はリスク管理の上限値です。実際の取引頻度は各戦略の `max_daily_trades` で制御します。

---

## strategy_params

`strategy_params.<strategy_name>` ブロックで戦略固有のパラメータを設定します。
設定は `pkg/strategy.Strategy.Initialize()` 経由で各戦略に渡されます。

同梱の Scalping 戦略のパラメータ詳細は [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md) を参照してください。

```yaml
# 例: 同梱のスキャルピング戦略
strategy_params:
  scalping:
    ema_fast_period: 9
    # ... 詳細は scalping/README.md を参照

# 例: カスタム戦略
strategy_params:
  mystrategy:
    my_param: 42
```

---

## ui

| キー | デフォルト | 説明 |
|---|---|---|
| `ui.host` | `"0.0.0.0"` | Web UI のリッスンホスト |
| `ui.port` | `8080` | Web UI のポート番号 |

---

## logging

| キー | デフォルト | 説明 |
|---|---|---|
| `logging.level` | `"info"` | グローバルログレベル（`debug` / `info` / `warn` / `error`） |
| `logging.format` | `"json"` | ログフォーマット |
| `logging.output` | `"both"` | 出力先（`stdout` / `file` / `both`） |
| `logging.file_path` | `"./logs/gogocoin.log"` | ログファイルパス |
| `logging.max_size_mb` | `50` | ログファイルの最大サイズ（MB） |
| `logging.max_backups` | `3` | ローテーション保持数 |
| `logging.max_age_days` | `7` | ログファイルの保持日数 |

### logging.categories

カテゴリごとにログレベルを個別設定できます。

| カテゴリ | デフォルト | 説明 |
|---|---|---|
| `trading` | `"debug"` | 取引関連ログ |
| `api` | `"info"` | API通信ログ |
| `strategy` | `"debug"` | 戦略シグナルログ |
| `ui` | `"info"` | Web UI / REST APIログ |

> 本番運用では `logging.level: "info"` を推奨します。`debug` は高頻度ログが出力されパフォーマンスに影響することがあります。

---

## data_retention

| キー | デフォルト | 説明 |
|---|---|---|
| `data_retention.retention_days` | `1` | DBに保持するデータの日数。`1` = 当日データのみ（最軽量） |

毎日 00:00 に `retention_days` より古いデータが自動削除されます。過去の取引履歴が必要な場合は bitFlyer 管理画面からダウンロードしてください。
