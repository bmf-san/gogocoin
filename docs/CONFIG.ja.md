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

> 起動時の設定検証は fail-fast です。`trading.symbols` に未知シンボルを指定するとエラーで起動停止します。

### trading.risk_management

| キー | デフォルト | 説明 |
|---|---|---|
| `max_total_loss_percent` | `50.0` | 累計損失の上限（初期資金に対する%）。超えると取引停止 |
| `max_trade_loss_percent` | `10.0` | 1回の取引での最大損失（%） |
| `max_daily_loss_percent` | `30.0` | 1日の損失上限（%） |
| `max_trade_amount_percent` | `80.0` | 1回の取引で使用できる残高の上限（%） |
| `max_daily_trades` | `100` | 1日の最大取引回数（リスク管理上限） |
| `min_trade_interval` | `60s` | 取引間の最小インターバル |
| `pnl_epoch` | _(空)_ | RFC3339 形式のタイムスタンプ (任意)。指定すると累積損益はこの時刻より前の取引を無視する |

> `max_daily_trades` はリスク管理の上限値です。実際の取引頻度は各戦略の `max_daily_trades` で制御します。

> `pnl_epoch` は、記録済みの取引履歴の一部が誤っていると分かっている場合のための設定です。累積損失は通常、全取引を集計した performance メトリクスから読み取るため、損益の計算が誤っていた期間があると上限判定が永久に歪みます。エポックを指定すると、取引を削除せずにその期間に線を引けます。

> エポックは損失上限だけでなく、累積値を出すすべての箇所に適用されます。取引ごとに書き込まれるメトリクス、`GET /api/performance`、`GET /api/v1/performance/symbols`、およびそれらから算出されるダッシュボードの合計値が対象です。エポックより前に記録された performance スナップショットは、値を補正するのではなく API から除外します。元になった取引単位の損益を再計算できないためです。ダッシュボードには集計の起点日をラベル表示し、範囲が狭まっていることが見て取れるようにしています。エポック以降の最初の決済が発生するまで合計は ¥0 と表示されますが、これは不具合ではなく事実です。

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
    auto_scale_enabled: true
    auto_scale_balance_pct: 80
    auto_scale_max_notional: 20000
    # ... 詳細は scalping/README.md を参照

# 例: カスタム戦略
strategy_params:
  mystrategy:
    my_param: 42
```

> `strategy_params.scalping.order_notional` は明示必須です（暗黙デフォルト値はありません）。

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
| `data_retention.retention_days` | `90`（example） | DBに保持するデータの日数。`1` = 当日データのみ（最軽量）、`90` = 直近 90 日（総損益をダッシュボードで全期間表示）。未設定時はコード側で `1` にフォールバック。 |

毎日 00:00 に `retention_days` より古いデータが自動削除されます。過去の取引履歴が必要な場合は bitFlyer 管理画面からダウンロードしてください。

---

## runtime

| キー | デフォルト | 説明 |
|---|---|---|
| `runtime.sell_size_percentage` | `0.95` | 決済時に売却可能数量のうち発注に使う割合。丸め誤差で残高を超えないよう `1.0` 未満にしています。`(0, 1]` の範囲。 |
| `runtime.shared_wallet` | `false` | 取引所口座にこのボットが管理していない資産も入っている場合に `true`。 |
| `runtime.history_limit` | `1000` | メモリ上に保持する市場データの最大件数。 |
| `runtime.signal_strength_threshold` | `0.5` | シグナルを採用する最小の強度。 |

### runtime.shared_wallet

決済数量は通常、ボット自身の建玉の残数量で頭打ちになります。買っていないコインを売らないための制限ですが、これには建玉テーブルが読める必要があります。読めなかったときは二択になり、どちらが正しいかは残高の持ち主によって変わります。

| | `shared_wallet: false`（既定） | `shared_wallet: true` |
|---|---|---|
| 建玉が読めないとき | 口座残高にフォールバック | 決済を見送ってエラーログを出す |
| 引き受けるリスク | 建玉を早く閉じてしまう | 損切りが遅れる |

専用口座なら残高は全部ボットのものなので、フォールバックは安全で、DB の不調が全ての決済を止めることを防げます。共有口座では同じフォールバックが他人の資産を売ることになり、約定した注文はリトライでは取り消せません。見送る側が復旧可能な失敗です。

建玉が読めている間はこのフラグは何もしません。通常の決済は変わりません。
