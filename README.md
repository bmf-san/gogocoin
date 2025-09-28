# gogocoin

bitFlyer取引所向けの暗号通貨取引ボット

[![CI](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml)
[![Release](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bmf-san/gogocoin)](https://goreportcard.com/report/github.com/bmf-san/gogocoin)
[![GitHub license](https://img.shields.io/github/license/bmf-san/gogocoin)](https://github.com/bmf-san/gogocoin/blob/main/LICENSE)
[![GitHub release](https://img.shields.io/github/release/bmf-san/gogocoin.svg)](https://github.com/bmf-san/gogocoin/releases)

## 概要

gogocoinは、bitFlyer暗号通貨取引所向けに設計されたGo言語ベースの自動取引ボットです。自動取引戦略、リアルタイム市場データ処理、リスク管理、Webベースの監視インターフェースを備えています。

## 機能

- 🤖 **自動取引**: bitFlyer API統合による設定可能な戦略（Scalping）
- 🎛️ **取引制御**: WebUI経由でリアルタイム取引開始・停止制御
- 📊 **リアルタイムデータ**: WebSocketベースの市場データ処理・保存
- 🛡️ **リスク管理**: 設定可能なストップロス・利確・緊急停止パラメータ
- 🧪 **ペーパートレード**: 実際のお金を使わない安全なテストモード（完全実装）
- 💰 **ライブトレード**: 本番取引モード（完全実装・本番準備完了）
- 🌐 **Webダッシュボード**: リアルタイム更新付きの監視インターフェース
  - 🎛️ **取引制御ボタン**: ワンクリックで取引開始・停止
  - 📊 **損益履歴**: 時系列での詳細な損益追跡
  - 📋 **注文履歴**: リアルタイム注文情報表示
  - 📊 **パフォーマンス指標**: 勝率・最大ドローダウン・シャープレシオ
- 💾 **データ永続化**: 軽量なSQLiteベースのデータベース
- 📝 **構造化ログ**: 標準Go slogベースのログシステム
- ✅ **包括的テスト**: 全パッケージの完全なテストカバレッジ（132テスト・59%+）
- 🔧 **設定管理**: YAML + 環境変数による柔軟な設定
- 🚀 **高品質コード**: Linter対応・型安全・エラーハンドリング

## 技術詳細

- **言語**: Go 1.25.0
- **依存関係**: 最小限（go-bitflyer-api-client + yaml.v3 + sqlite3のみ）
- **アーキテクチャ**: レイヤー分離された**モジュラーアーキテクチャ**
- **データベース**: SQLite（軽量・埋め込み・外部DB不要）
- **並行処理**: Goroutines + Channels による**非同期ワーカー**
- **通信**: WebSocket（リアルタイム） + REST API（Web UI）
- **ログ**: 標準log/slogベースの**構造化ログ**
- **デプロイ**: 埋め込みWebアセット付き**シングルバイナリ**
- **品質保証**:
  - Linter対応（gocritic, misspell, hugeParam, rangeValCopy）- 0 issues
  - テストカバレッジ 59%+（132テスト・7パッケージ）
  - 型安全性（ポインタ使用による効率化）
  - エラーハンドリング（堅牢な例外処理）
  - 本番準備完了（ライブモード完全実装）

## 免責事項

**このソフトウェアは情報提供および開発目的でのみ提供されており、金融アドバイスや投資判断を構成することを意図していません。このソフトウェアの使用により生じるいかなる損失や損害についても、作者は責任を負いません。**

**このライブラリはbitFlyerと一切関係ありません。使用前に各APIプロバイダーの利用規約を確認してください。**

**このライブラリは「現状のまま」提供され、正確性、完全性、将来の互換性についていかなる保証もありません。**

## インストール

```bash
git clone https://github.com/bmf-san/gogocoin.git
cd gogocoin
make deps
```

## 設定

1. サンプル設定をコピー:
```bash
make init-config
```

2. `configs/config.yaml`を編集して設定を変更:

### 完全な設定例
```yaml
# アプリケーション基本設定
app:
  name: gogocoin

# bitFlyer API設定
api:
  endpoint: https://api.bitflyer.com
  websocket_endpoint: wss://ws.lightstream.bitflyer.com/json-rpc
  credentials:
    api_key: "${BITFLYER_API_KEY}"      # 環境変数から取得
    api_secret: "${BITFLYER_API_SECRET}" # 環境変数から取得
  timeout: 30s
  retry_count: 3
  rate_limit:
    requests_per_minute: 500
  enabled: true                         # Web API サーバーの有効化
  port: 8080                            # Web API サーバーのポート番号

# 取引設定
trading:
  enabled: true
  mode: paper                           # paper（仮想取引）, live（実取引）
  fee_rate: 0.0015                      # ペーパーモード専用手数料率（ライブモードはAPI自動取得）
  initial_balance: 1000000              # ペーパートレード用初期残高（円）
  symbols:                              # 取引対象通貨ペア
    - BTC_JPY
    - ETH_JPY
    - XRP_JPY
    - XLM_JPY
    - MONA_JPY
  strategy:
    name: scalping                      # 使用する戦略: scalping
  risk_management:
    max_total_loss_percent: 20          # 最大総損失率（%）
    max_trade_loss_percent: 2           # 最大取引損失率（%）
    max_daily_loss_percent: 5           # 最大日次損失率（%）
    max_trade_amount_percent: 5         # 最大取引金額率（%）
    max_daily_trades: 200               # 最大日次取引数
    min_trade_interval: 30s             # 最小取引間隔
    stop_loss_percent: 2                # ストップロス率（%）
    take_profit_percent: 5              # 利確率（%）

# 戦略パラメータ
strategy_params:
  scalping:
    ema_fast_period: 9                  # 短期EMA期間（9バー）
    ema_slow_period: 21                 # 中期EMA期間（21バー）
    take_profit_pct: 0.8                # 利確率（%）※エントリーから+0.8%で利確
    stop_loss_pct: 0.4                  # 損切率（%）※エントリーから-0.4%で損切
    cooldown_sec: 90                    # クールダウン時間（秒）※取引後90秒待機
    max_daily_trades: 3                 # 1日最大取引回数（3回まで）
    min_notional: 200                   # 最小注文金額（JPY）※200円から対応
    fee_rate: 0.001                     # 取引手数料率（0.1%）

# データ設定
data:
  storage:
    type: sqlite                        # データベースタイプ
    path: ./data                        # データ保存パス
  market_data:
    realtime_enabled: true              # リアルタイムデータ取得
    channels:                           # WebSocket購読チャンネル
      - lightning_ticker_BTC_JPY
      - lightning_executions_BTC_JPY
      - lightning_board_BTC_JPY
    history_enabled: true               # 履歴データ保存
    history_days: 365                   # 履歴データ保持日数

# Web UI設定
ui:
  enabled: true                         # Web UI有効化
  host: localhost                       # バインドホスト
  port: 8080                            # ポート番号
  static_path: ./web                    # 静的ファイルパス

# ログ設定
logging:
  level: info                           # ログレベル: debug, info, warn, error
  format: json                          # ログ形式: json, text
  output: file                          # 出力先: console, file, both
  file_path: ./logs/gogocoin.log        # ログファイルパス
  max_size_mb: 100                      # ログファイル最大サイズ（MB）
  max_backups: 5                        # ローテーション保持ファイル数
  max_age_days: 30                      # ログファイル保持日数
  categories:                           # カテゴリ別ログレベル
    api: info
    strategy: info
    trading: info
    ui: info

# 開発設定
development:
  metrics_enabled: true                 # メトリクス収集機能
```

### 環境変数設定
```bash
# bitFlyer API認証情報
export BITFLYER_API_KEY="your_api_key_here"
export BITFLYER_API_SECRET="your_api_secret_here"

# 取引モード（本番環境での安全性確保）
export GOGOCOIN_MODE="paper"  # paper/live
```

### 設定の優先順位
1. コマンドライン引数
2. 環境変数
3. 設定ファイル（config.yaml）
4. デフォルト値

## モード選択ガイド

### 🚀 **初めて使用する場合**
1. **ペーパートレード** → 戦略をテスト
2. **ライブトレード** → 実際の取引開始

### 📊 **目的別モード選択**

| 目的 | 推奨モード | 理由 |
|------|------------|------|
| 🔰 **学習・練習** | ペーパートレード | 安全にシステムを理解できる |
| 🧪 **戦略テスト** | ペーパートレード | リスクなしで効果を確認 |
| 🛠️ **設定調整** | ペーパートレード | パラメータを安全に試行錯誤 |
| 🐛 **問題調査** | 開発モード | 詳細ログで原因を特定 |
| 💰 **実際の取引** | ライブトレード | 十分テスト後の本番運用 |

## 使用方法

### 🚀 **クイックスタート**

#### 1. ペーパートレード（推奨）
```bash
# 設定ファイルを初期化
make init-config

# 環境変数を設定（オプション）
export BITFLYER_API_KEY="your_api_key"
export BITFLYER_API_SECRET="your_api_secret"

# ペーパートレードで起動
make run-paper
```

#### 2. Web UIでモニタリング・制御
```bash
# ブラウザで以下にアクセス
open http://localhost:8080
```

**🎛️ 取引制御:**
- システム状況カードで現在の取引状態を確認
- 「取引開始」ボタンで取引を開始
- 「取引停止」ボタンで取引を停止
- 状態変更は即座にリアルタイム反映

### 📋 **実行モード詳細**

#### ペーパートレード（テスト推奨）
```bash
make run-paper
# または
./bin/gogocoin -mode paper
```
**仮想的な取引モード** - 実際のお金を使わずに取引戦略をテストできます。

**✅ 基本特性:**
- ✅ **安全**: 実際の資金は使用されません（完全シミュレーション）
- 📊 **学習**: 戦略の効果を確認できます
- 🔧 **調整**: パラメータを安全に試行錯誤できます
- 💡 **推奨**: 初回使用時や新しい戦略のテスト時
- 🎯 **完全実装**: ポジション管理・パフォーマンス計算・チャート表示

**🔧 実装詳細:**

1. **初期残高設定**
   - `config.yaml`の`trading.initial_balance`で設定（デフォルト: 1,000,000円）
   - JPY: 設定額から開始
   - 暗号通貨（BTC/ETH/XRP/XLM/MONA）: 0から開始

2. **市場データ取得**
   - **リアルタイム**: bitFlyer WebSocketから実際の市場データを取得
   - **価格**: 本番と同じティッカーデータ使用
   - **注意**: WebSocket未接続時はモックデータで動作

3. **注文処理**
   - **即座約定**: 注文は即座に約定（スリッページなし）
   - **手数料**: 0.15%（bitFlyer実取引と同じ）
   - **残高チェック**: 注文前に必ず残高を検証
   - **注文ID**: `paper_<timestamp>`形式で生成

4. **現物取引シミュレーション**
   - **取引タイプ**: 現物取引（Spot Trading）をシミュレーション
   - **ポジション**: なし（現物取引のため）
   - **残高管理**: 購入した暗号通貨は即座に残高に反映
   - **例**: BTC 0.001を購入 → BTC残高が0.001増加、JPY残高が減少
   - **信用取引との違い**: レバレッジなし、証拠金なし、建玉なし

5. **手数料計算（詳細）**
   ```
   手数料率: ペーパーモード専用設定（デフォルト: 0.15% = 0.0015）

   設定方法:
   trading:
     fee_rate: 0.0015  # ペーパーモード専用手数料率
                       # ライブモードでは使用されない（API自動取得）

   計算式（ライブモードと完全に同じ）:
   fee = 約定数量 × 平均約定価格 × 手数料率

   ペーパーモード:
   - 約定数量 = 注文数量（即座に全量約定）
   - 平均約定価格 = 注文価格
   - 手数料率 = config.yamlの設定値（fee_rate）
   - 理由: API認証不要でテスト可能にするため

   ライブモード:
   - 約定数量 = APIから取得（部分約定対応）
   - 平均約定価格 = APIから取得
   - 手数料 = APIから取得（TotalCommission）
   - 理由: bitFlyerの実際の手数料体系を反映

   例（fee_rate: 0.0015の場合）:
   - BTC 0.001を4,500,000円で購入
     約定数量: 0.001
     平均約定価格: 4,500,000円
     fee = 0.001 × 4,500,000 × 0.0015 = 6.75円

   残高への影響:
   - 買い注文: 必要資金 = (約定数量 × 平均約定価格) + 手数料
     例: 4,500円 + 6.75円 = 4,506.75円がJPYから引かれる

   - 売り注文: 受取金額 = (約定数量 × 平均約定価格) - 手数料
     例: 4,500円 - 6.75円 = 4,493.25円がJPYに入る

   ※ ペーパーモードでもライブモードと完全に同じ計算式を使用
   ```

5. **残高管理**
   - **買い注文**: JPY減少、暗号通貨増加（手数料込み）
   - **売り注文**: 暗号通貨減少、JPY増加（手数料差引後）
   - **リアルタイム更新**: 各取引後に即座に残高更新
   - **データベース永続化**: 全残高変動を記録

6. **取引記録**
   - **データベース保存**: 全取引をSQLiteに記録
   - **ポジション追跡**: 各取引のポジション状態を保存
   - **履歴管理**: Web UIで全取引履歴を確認可能
   - **パフォーマンス計算**: PnL、勝率、最大ドローダウン等を自動算出

7. **制限事項**
   - ✅ **約定保証**: 注文は100%約定（本番では約定しない場合あり）
   - ✅ **スリッページなし**: 指定価格で必ず約定（本番では価格変動あり）
   - ✅ **板情報無視**: 市場の流動性を考慮しない（本番では考慮必要）
   - ✅ **遅延なし**: ネットワーク遅延やAPI制限の影響なし

8. **動作確認方法**
   ```bash
   # ペーパーモードで起動
   make run-paper

   # Web UIで確認
   open http://localhost:8080

   # ログで取引を確認
   tail -f logs/gogocoin.log | grep PAPER_ORDER

   # 残高を確認
   curl -s http://localhost:8080/api/balance | jq .
   ```

**📊 ペーパーモードの精度:**
- **価格データ**: 100%リアル（bitFlyer WebSocket）
- **手数料**: 100%リアル（0.15%）
- **残高管理**: 100%正確（メモリ + DB永続化）
- **約定タイミング**: 簡略化（即座約定）
- **市場影響**: 考慮なし（大口注文でも価格不変）

**⚠️ 本番移行時の注意:**
- ペーパーモードで利益が出ても、本番で同じ結果になるとは限りません
- 本番では約定しない、スリッページ、遅延などの要因があります
- 必ず少額から始めて、徐々に取引額を増やしてください

#### ライブトレード（本番準備完了）
```bash
make run
# または
./bin/gogocoin -mode live
```
**実際の取引モード** - 本物のbitFlyer APIを使用して実際に取引を行います。

**✅ 完全実装済み機能:**
- 🔒 **残高チェック**: 注文前の必須残高検証
- ⚡ **部分約定対応**: FilledSize監視・段階的記録
- 🔄 **約定監視**: 30秒間の非同期ポーリング
- 💾 **データ永続化**: 取引・ポジション・残高の完全記録
- 🛡️ **エラーハンドリング**: API障害・ネットワークエラー対応
- 📊 **レート制限**: bitFlyer API制限の自動遵守

**💰 手数料について:**
- **取得方法**: bitFlyer APIから実際の手数料を取得（`TotalCommission`フィールド）
- **手数料率**: bitFlyerの実際の手数料体系に従う
  - 通常: 0.01%〜0.15%（取引量・通貨ペアによって変動）
  - 詳細: [bitFlyer手数料一覧](https://bitflyer.com/ja-jp/commission)
- **記録**: 約定時にAPIから返される実際の手数料を記録
- **表示**: Web UIの取引履歴で確認可能
- **注意**: ペーパーモードは固定0.15%、ライブモードはbitFlyerの実手数料

**⚠️ 使用時の注意:**
- 💰 **実資金**: 本当の利益・損失が発生します
- 🔑 **要認証**: bitFlyer APIキーが必要です
- 📈 **推奨**: ペーパートレードで十分テストした後に使用
- 💸 **手数料**: 実際の手数料が発生（ペーパーより高い/低い場合あり）

#### 開発モード
```bash
make dev
# または
./bin/gogocoin -mode paper -log-level debug
```
**開発・デバッグモード** - 開発者向けの詳細ログ付きで実行されます。
- 🐛 **デバッグ**: 詳細なログが出力されます
- 🔍 **監視**: システムの内部動作を確認できます
- 🛠️ **開発**: 新機能の開発やバグ修正時に使用
- 📝 **ログ**: より多くの情報がコンソールに表示されます
- 🧪 **安全**: ペーパートレードベースで実際の資金は使用されません

### 🌐 **Web UI**
```
http://localhost:8080
```
**リアルタイム監視インターフェース** - ブラウザで取引状況を確認できます。

#### 🖥️ **Web UIの機能**
- 📊 **ダッシュボード**: 残高、ポジション、損益の一覧表示
- 🎛️ **取引制御**: ワンクリックで取引開始・停止（リアルタイム反映）
- 📊 **損益履歴**: 時系列での詳細な損益追跡テーブル
- 📋 **取引履歴**: 過去の取引記録と詳細分析
- 🔄 **注文情報**: リアルタイム注文履歴表示
- 📊 **パフォーマンス**: 勝率・最大ドローダウン・シャープレシオ
- 📝 **ログ監視**: レベル別ログフィルタリング
- 📱 **レスポンシブ**: PC・タブレット・スマホ対応

#### 🔧 **アクセス方法**
1. アプリケーションを起動（任意のモード）
2. ブラウザで `http://localhost:8080` を開く
3. リアルタイムで取引状況を監視

### ⚡ **コマンドライン オプション**
```bash
# バージョン確認
./bin/gogocoin -version

# ヘルプ表示
./bin/gogocoin -help

# カスタム設定ファイル使用
./bin/gogocoin -config ./configs/production.yaml

# データベース初期化
./bin/gogocoin -init-db

```


## アーキテクチャ構成

### 🏗️ **システム全体設計**

gogocoinは**マイクロサービス指向**の**モジュラーアーキテクチャ**を採用しており、各コンポーネントが明確な責任分離を持っています。

```
┌─────────────────────────────────────────────────────────────┐
│                     🌐 Web UI (Browser)                     │
│                    http://localhost:8080                    │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP/WebSocket
┌─────────────────────▼───────────────────────────────────────┐
│                  📡 API Server (internal/api)               │
│              REST API + Static File Serving                │
└─────────────────────┬───────────────────────────────────────┘
                      │ In-Process Communication
┌─────────────────────▼───────────────────────────────────────┐
│                🤖 Application Core (internal/app)           │
│                   Main Business Logic                       │
├─────────────────────┼─────────────────────┬─────────────────┤
│  📊 Market Data     │  🧠 Strategy Engine │  💱 Trading     │
│     Worker          │       Worker        │    Worker       │
└─────────────────────┼─────────────────────┼─────────────────┘
                      │                     │
                      ▼                     ▼
              ┌───────────────┐    ┌─────────────────┐
              │  📈 Strategy  │    │ 🔄 bitFlyer API │
              │   (MA Cross)  │    │    Client       │
              └───────────────┘    └─────────────────┘
                      │                     │
                      ▼                     ▼
              ┌───────────────┐    ┌─────────────────┐
              │ 💾 SQLite DB  │    │ 🌐 bitFlyer     │
              │   (Local)     │    │   Exchange      │
              └───────────────┘    └─────────────────┘
```

### 🔄 **データフロー**

```
1. 📡 WebSocket Data Reception
   bitFlyer → WebSocket Client → Market Data Service → Channel

2. 🧠 Strategy Processing
   Channel → Strategy Worker → Moving Average Analysis → Signal

3. 💱 Trade Execution
   Signal → Trading Service → bitFlyer API → Order Placement

4. 💾 Data Persistence
   All Events → Database Service → SQLite → Disk Storage

5. 🌐 Web UI Updates
   Database → API Server → HTTP Response → Browser Updates
```

### 🏛️ **レイヤーアーキテクチャ**

#### **1. Presentation Layer (表示層)**
```
web/                    # 静的アセット
├── index.html         # メインUI
├── style.css          # スタイリング
└── script.js          # クライアントサイドロジック

internal/api/          # HTTP APIサーバー
├── server.go          # RESTエンドポイント
├── handlers.go        # リクエストハンドラー
└── middleware.go      # 認証・ログ・CORS
```

#### **2. Application Layer (アプリケーション層)**
```
internal/app/          # ビジネスロジック
├── app.go            # メインアプリケーション
├── workers.go        # バックグラウンドワーカー
└── lifecycle.go      # 起動・停止管理

cmd/gogocoin/         # エントリーポイント
└── main.go           # アプリケーション初期化
```

#### **3. Domain Layer (ドメイン層)**
```
internal/strategy/     # 取引戦略
├── strategy.go       # 戦略インターフェース
├── scalping.go       # スキャルピング戦略実装
└── scalping_test.go  # テストコード

```

#### **4. Infrastructure Layer (インフラ層)**
```
internal/bitflyer/    # 外部API統合
├── client.go         # APIクライアント
├── websocket.go      # リアルタイムデータ
├── trading.go        # 取引実行
└── market_data.go    # 市場データ取得

internal/database/    # データ永続化
├── database.go       # SQLite操作
├── models.go         # データモデル
└── migrations.go     # スキーマ管理

internal/config/      # 設定管理
├── config.go         # 設定読み込み
└── validation.go     # 設定検証

internal/logger/      # ログ管理
├── logger.go         # 構造化ログ
└── formatters.go     # ログフォーマット
```

### 🔧 **コンポーネント間通信**

#### **1. 同期通信**
- **HTTP REST API**: Web UI ↔ API Server
- **関数呼び出し**: Application Core内のコンポーネント間
- **SQLite クエリ**: アプリケーション ↔ データベース

#### **2. 非同期通信**
- **WebSocket**: bitFlyer ↔ Market Data Service
- **Go Channels**: Worker間のデータ受け渡し
- **Event Sourcing**: 取引イベントの記録・再生

#### **3. 設定駆動**
- **YAML設定**: 戦略パラメータ、API認証、リスク管理
- **環境変数**: デプロイメント固有の設定
- **実行時更新**: Web UIからの設定変更

### ⚡ **並行処理設計**

```go
// メインアプリケーションの並行ワーカー
go app.runMarketDataWorker(ctx)    // 市場データ収集
go app.runStrategyWorker(ctx)      // 戦略実行
go app.runTradingWorker(ctx)       // 取引実行
go app.runAPIServer(ctx)           // Web API提供

// チャンネルベースの通信
marketDataCh := make(chan MarketData, 100)
signalCh := make(chan Signal, 50)
orderCh := make(chan Order, 25)
```

### 🛡️ **エラー処理・回復性**

#### **1. 回復可能エラー**
- **API接続エラー**: 指数バックオフによる自動リトライ
- **WebSocket切断**: 自動再接続機能
- **一時的な市場データ欠損**: バッファリング・補完

#### **2. 致命的エラー**
- **設定エラー**: 起動時バリデーション・即座停止
- **認証エラー**: セキュアな停止・ログ記録
- **データベース破損**: 自動バックアップ・復旧

#### **3. 監視・アラート**
- **構造化ログ**: JSON形式・レベル別出力
- **メトリクス収集**: パフォーマンス・エラー統計
- **Web UI表示**: リアルタイム状態監視

### 📦 **デプロイメント**

#### **1. シングルバイナリ**
```bash
# 全アセットを埋め込んだ単一実行ファイル
./bin/gogocoin -config configs/config.yaml -mode paper
```

#### **2. 設定外部化**
```
configs/config.yaml    # メイン設定
.env                   # 環境変数（API認証）
data/                  # ランタイムデータ
logs/                  # ログファイル
```

#### **3. 軽量依存**
- **ゼロ外部依存**: 追加サーバー・DB不要
- **最小リソース**: CPU・メモリ効率的
- **クロスプラットフォーム**: Linux・macOS・Windows対応

## プロジェクト構造

```
gogocoin/
├── cmd/gogocoin/          # アプリケーションエントリーポイント
├── internal/
│   ├── api/              # Web APIサーバー
│   ├── app/              # メインアプリケーションロジック
│   ├── bitflyer/         # bitFlyer APIクライアントラッパー
│   ├── config/           # 設定管理
│   ├── database/         # SQLiteベースのデータ永続化
│   ├── logger/           # 構造化ログ
│   └── strategy/         # 取引戦略
├── configs/              # 設定ファイル
├── web/                  # Web UIアセット
├── data/                 # ランタイムデータストレージ
└── logs/                 # ログファイル
```

## 取引戦略

gogocoinは現在、スキャルピング戦略をサポートしています。

### Scalping戦略（v2.0）
EMAベースのステートレス・スキャルピング戦略です。

**特徴:**
- **ステートレス設計**: 再起動に強い（内部状態を最小限に保持）
- **少額対応**: 200-300円から取引可能
- **EMAベース**: 短期EMA（9バー）と中期EMA（21バー）のクロスオーバー
- **リスク管理**: 利確0.8%、損切0.4%、クールダウン90秒
- **取引制限**: 1日最大3回（手数料負け防止）

**シグナル生成ロジック:**
- **買い（BUY）**: 短期EMA > 中期EMA かつ 現在価格 > 短期EMA
- **売り（SELL）**: 短期EMA < 中期EMA かつ 現在価格 < 短期EMA
- **待機（HOLD）**: 上記以外

**設定例:**
```yaml
strategy:
  name: scalping

strategy_params:
  scalping:
    ema_fast_period: 9       # 短期EMA期間（9バー）
    ema_slow_period: 21      # 中期EMA期間（21バー）
    take_profit_pct: 0.8     # 利確率（%）※+0.8%で利確
    stop_loss_pct: 0.4       # 損切率（%）※-0.4%で損切
    cooldown_sec: 90         # クールダウン時間（秒）
    max_daily_trades: 3      # 1日最大取引回数
    min_notional: 200        # 最小注文金額（JPY）
    fee_rate: 0.001          # 取引手数料率（0.1%）
```

**推奨設定:**
- 初心者: デフォルト設定をそのまま使用
- 上級者: EMA期間やリスクパラメータを調整可能

### カスタム戦略開発
`Strategy`インターフェースを実装してカスタム取引戦略を作成できます:

```go
type Strategy interface {
    Name() string
    Description() string
    Version() string
    Initialize(config map[string]interface{}) error
    UpdateConfig(config map[string]interface{}) error
    GetConfig() map[string]interface{}
    GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error)
    Analyze(data []MarketData) (*Signal, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    IsRunning() bool
    GetStatus() StrategyStatus
    GetMetrics() StrategyMetrics
    Reset() error
}
```

**実装例:**
```go
type MyCustomStrategy struct {
    *BaseStrategy
    // 戦略固有のパラメータ
}

func (s *MyCustomStrategy) GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error) {
    // カスタムロジックを実装
    return s.CreateSignal(
        data.Symbol,
        SignalBuy, // SignalBuy, SignalSell, SignalHold
        1.0,       // シグナル強度
        data.Price,
        0.001,     // 取引量
        map[string]interface{}{
            "reason": "custom_logic",
        },
    ), nil
}
```

**戦略の登録:**
新しい戦略を追加するには、`internal/strategy/strategy.go`の`CreateStrategy`メソッドに戦略を登録します。

## リスク管理

- **ポジションサイジング**: 取引あたりの残高の設定可能な割合
- **ストップロス**: 指定された割合での自動損切り
- **利確**: 指定された割合での自動利確
- **日次制限**: 1日あたりの最大取引数
- **緊急停止**: 重大な損失時の自動シャットダウン

## Webインターフェース

Webインターフェースでは以下を提供:
- リアルタイム取引状況
- 現在の残高とポジション
- 損益追跡
- 取引履歴
- パフォーマンス指標
- 戦略設定
- ログ監視

## API 一覧

gogocoinは以下のREST APIエンドポイントを提供します。デフォルトでは`http://localhost:8080`でアクセス可能です。

### 📋 **API エンドポイント一覧**

| カテゴリ | エンドポイント | メソッド | 説明 |
|---------|---------------|----------|------|
| **システム** | `/api/status` | GET | システム状態・稼働時間・取引統計 |
| **残高・ポジション** | `/api/balance` | GET | 現在の残高情報（全通貨） |
| | `/api/positions` | GET | アクティブなポジション一覧 |
| **取引・注文** | `/api/trades` | GET | 取引履歴（limit指定可能） |
| | `/api/orders` | GET | 注文履歴（limit指定可能） |
| **パフォーマンス** | `/api/performance` | GET | 損益・勝率・各種指標 |
| **設定・管理** | `/api/config` | GET/POST | 設定の取得・更新 |
| | `/api/strategy/reset` | POST | 戦略状態のリセット |
| **取引制御** | `/api/trading/start` | POST | 取引開始 |
| | `/api/trading/stop` | POST | 取引停止 |
| | `/api/trading/status` | GET | 取引状態確認 |
| **ログ** | `/api/logs` | GET | システムログ（level・limit指定可能） |

### 🔧 **クイックアクセス**

```bash
# システム状態確認
curl http://localhost:8080/api/status

# 残高確認
curl http://localhost:8080/api/balance

# 最新10件の取引履歴
curl http://localhost:8080/api/trades?limit=10

# エラーログのみ表示
curl http://localhost:8080/api/logs?level=ERROR&limit=20

# 取引開始
curl -X POST http://localhost:8080/api/trading/start

# 取引停止
curl -X POST http://localhost:8080/api/trading/stop

# 取引状態確認
curl http://localhost:8080/api/trading/status

# 戦略リセット
curl -X POST http://localhost:8080/api/strategy/reset

# 設定更新（JSON）
curl -X POST http://localhost:8080/api/config \
  -H "Content-Type: application/json" \
  -d '{"trading":{"mode":"paper"}}'
```

### 🌐 **WebUI連携**

APIはWebUIから自動的に呼び出されますが、直接アクセスも可能：

- **リアルタイム監視**: `http://localhost:8080` でWebUI
- **JSON API**: `http://localhost:8080/api/*` で生データ
- **ログ監視**: WebUIのログタブまたは `/api/logs`
- **設定変更**: WebUIの設定タブまたは `/api/config`

### ⚡ **API特徴**

- **🔒 セキュア**: 15秒タイムアウト・適切なHTTPステータス
- **📊 リアルタイム**: ライブデータ・ペーパートレード対応
- **🎯 RESTful**: 標準的なHTTPメソッド・JSONレスポンス
- **🔧 柔軟**: クエリパラメータによるフィルタリング・制限
- **📝 ログ**: 全API呼び出しの詳細ログ記録
- **🚀 高速**: 軽量SQLite・効率的なクエリ

## API リファレンス

以下に各エンドポイントの詳細仕様を示します。

### システム情報

#### `GET /api/status`
システムの現在状態を取得します。

**レスポンス例:**
```json
{
  "status": "running",
  "mode": "ペーパートレード",
  "strategy": "scalping",
  "last_update": "2025-09-30T17:22:13.609489+09:00",
  "uptime": "12m",
  "total_trades": 4,
  "active_orders": 4
}
```

### 残高・ポジション

#### `GET /api/balance`
現在の残高情報を取得します。

**レスポンス例:**
```json
[
  {
    "currency": "JPY",
    "available": 1000000.42,
    "amount": 1000000.42,
    "timestamp": "2025-09-30T13:38:52.207134+09:00"
  },
  {
    "currency": "BTC",
    "available": 10,
    "amount": 10,
    "timestamp": "2025-09-30T13:38:52.207134+09:00"
  }
]
```

#### `GET /api/positions`
アクティブなポジション情報を取得します。

**レスポンス例:**
```json
[
  {
    "product_code": "BTC_JPY",
    "side": "BUY",
    "size": 0.001,
    "price": 4500000,
    "pnl": 1500.0,
    "open_time": "2025-09-30T10:00:00Z"
  }
]
```

### 取引・注文

#### `GET /api/trades`
取引履歴を取得します。

**クエリパラメータ:**
- `limit`: 取得件数（デフォルト: 50、最大: 100）

**レスポンス例:**
```json
[
  {
    "id": 1,
    "symbol": "BTC_JPY",
    "side": "BUY",
    "size": 0.001,
    "price": 4500000,
    "fee": 67.5,
    "status": "COMPLETED",
    "executed_at": "2025-09-30T10:00:00Z",
    "strategy": "scalping"
  }
]
```

#### `GET /api/orders`
注文履歴を取得します。

**クエリパラメータ:**
- `limit`: 取得件数（デフォルト: 20、最大: 100）

**レスポンス例:**
```json
[
  {
    "order_id": "paper_1759222695463540000",
    "symbol": "XRP_JPY",
    "side": "SELL",
    "type": "MARKET",
    "size": 0.001,
    "price": 423.8,
    "status": "COMPLETED",
    "executed_at": "2025-09-30T17:48:11.454007+09:00",
    "created_at": "2025-09-30T17:48:11.454007+09:00"
  }
]
```

### パフォーマンス

#### `GET /api/performance`
パフォーマンス指標を取得します。

**レスポンス例:**
```json
[
  {
    "id": 1,
    "date": "2025-09-30T13:42:25.081249+09:00",
    "total_return": -0.0000642015,
    "daily_return": 0,
    "win_rate": 0,
    "max_drawdown": 0.0000642015,
    "sharpe_ratio": 0,
    "profit_factor": 0,
    "total_trades": 1,
    "winning_trades": 0,
    "losing_trades": 1,
    "average_win": 0,
    "average_loss": 0.000642015,
    "largest_win": 0,
    "largest_loss": 0.000642015,
    "total_pnl": -0.000642015
  }
]
```

### 設定・管理

#### `GET /api/config`
現在の設定を取得します。

#### `POST /api/config`
設定を更新します。

**リクエスト例:**
```json
{
  "trading": {
    "mode": "paper",
    "max_position_size": 0.1
  },
  "strategy": {
    "name": "scalping"
  },
  "risk": {
    "stop_loss": 0.02,
    "take_profit": 0.05
  }
}
```

#### `POST /api/strategy/reset`
戦略の状態をリセットします。

**レスポンス例:**
```json
{
  "status": "success",
  "message": "Strategy reset successfully"
}
```

### 取引制御

#### `POST /api/trading/start`
取引を開始します。

**レスポンス例:**
```json
{
  "enabled": true,
  "status": "success",
  "message": "Trading started successfully",
  "timestamp": "2025-10-01T22:00:00Z"
}
```

#### `POST /api/trading/stop`
取引を停止します。

**レスポンス例:**
```json
{
  "enabled": false,
  "status": "success",
  "message": "Trading stopped successfully",
  "timestamp": "2025-10-01T22:00:00Z"
}
```

#### `GET /api/trading/status`
現在の取引状態を取得します。

**レスポンス例:**
```json
{
  "enabled": true,
  "status": "running",
  "message": "Trading is currently active",
  "timestamp": "2025-10-01T22:00:00Z"
}
```


### ログ

#### `GET /api/logs`
システムログを取得します。

**クエリパラメータ:**
- `limit`: 取得件数（デフォルト: 100、最大: 200）
- `level`: ログレベルフィルタ（ERROR, WARN, INFO, DEBUG）

**レスポンス例:**
```json
[
  {
    "id": 1,
    "level": "INFO",
    "category": "trading",
    "message": "Order placed successfully",
    "timestamp": "2025-09-30T10:00:00Z"
  }
]
```

### エラーレスポンス

全てのAPIエンドポイントは、エラー時に適切なHTTPステータスコードと共にエラー情報を返します。

**エラーレスポンス例:**
```json
{
  "error": "Invalid request parameters",
  "message": "Missing required field: strategy"
}
```

**HTTPステータスコード:**
- `200`: 成功
- `400`: リクエストエラー
- `405`: メソッドが許可されていない
- `500`: サーバー内部エラー

## トラブルシューティング

### よくある問題と解決方法

#### 🔧 **起動時の問題**

**問題**: `data`ディレクトリが存在しない
```bash
Error: failed to initialize database: no such file or directory
```
**解決方法**:
```bash
mkdir -p data logs
```

**問題**: 設定ファイルが見つからない
```bash
Error: config file not found
```
**解決方法**:
```bash
make init-config
# または
cp configs/config.example.yaml configs/config.yaml
```

#### 🔑 **API認証の問題**

**問題**: bitFlyer API認証エラー
```bash
Error: API authentication failed
```
**解決方法**:
1. API キーと秘密鍵が正しく設定されているか確認
2. 環境変数が正しくエクスポートされているか確認
```bash
echo $BITFLYER_API_KEY
echo $BITFLYER_API_SECRET
```
3. bitFlyerでAPIキーの権限を確認（取引権限が必要）

#### 📊 **取引が実行されない問題**

**問題**: ペーパーモードで取引が実行されない
**解決方法**:
1. 戦略の最大取引回数に達していないか確認
2. WebUIで戦略をリセット
3. ログでエラーメッセージを確認
```bash
curl -s http://localhost:8080/api/logs?level=ERROR | jq .
```

**問題**: 「取引間隔が短すぎる」エラー
**解決方法**:
設定ファイルでクールダウン時間を調整:
```yaml
strategy_params:
  scalping:
    cooldown_sec: 120  # より長い間隔に設定（120秒）
```

#### 🌐 **WebUIアクセスの問題**

**問題**: `http://localhost:8080`にアクセスできない
**解決方法**:
1. プロセスが起動しているか確認
```bash
ps aux | grep gogocoin
```
2. ポートが使用中でないか確認
```bash
lsof -i :8080
```
3. ファイアウォール設定を確認

#### 💾 **データベースの問題**

**問題**: SQLiteエラーが発生
**解決方法**:
1. データベースファイルの権限を確認
2. データベースファイルを削除して再作成
```bash
rm data/gogocoin.db
# アプリケーションを再起動すると自動で再作成される
```

#### 📈 **パフォーマンス問題**

**問題**: メモリ使用量が多い
**解決方法**:
1. 市場データ履歴の保存期間を短縮
```yaml
data:
  market_data:
    history_days: 30  # デフォルト365日から短縮
```
2. ログレベルを下げる
```yaml
logging:
  level: warn  # debugからwarnに変更
```

### ログの確認方法

#### WebUI経由
1. ブラウザで`http://localhost:8080`にアクセス
2. 「ログ」タブを選択
3. レベルフィルタで「ERROR」を選択

#### API経由
```bash
# 最新のエラーログを取得
curl -s "http://localhost:8080/api/logs?limit=10&level=ERROR" | jq .

# 戦略関連のログを取得
curl -s "http://localhost:8080/api/logs?limit=20" | jq '.[] | select(.category == "strategy")'
```

#### ファイル直接確認
```bash
# 最新のログを確認
tail -f logs/gogocoin.log

# エラーログのみを確認
grep "ERROR" logs/gogocoin.log | tail -10
```

### サポートとバグ報告

問題が解決しない場合は、以下の情報と共にIssueを作成してください:

1. **環境情報**:
   - OS (macOS/Linux/Windows)
   - Go バージョン
   - gogocoin バージョン

2. **設定情報**:
   - 使用中の設定ファイル（機密情報は削除）
   - 実行モード（paper/live）

3. **エラー情報**:
   - エラーメッセージ
   - 関連するログ
   - 再現手順

4. **期待する動作**:
   - 何をしようとしていたか
   - 期待していた結果

## 開発

### 前提条件
- Go 1.25.0以降
- make
- Git

### セットアップ
```bash
# リポジトリをクローン
git clone https://github.com/bmf-san/gogocoin.git
cd gogocoin

# 依存関係をインストール
make deps

# 開発ツールをインストール
make install-tools

# 設定ファイルを初期化
make init-config

# ビルド
make build
```

### 開発ワークフロー
```bash
# 1. コード変更後、リンターを実行
make lint

# 2. テストを実行
make test

# 3. カバレッジ付きテストを実行
make test-coverage

# 4. ビルドして動作確認
make build
make run-paper

# 5. Web UIで動作確認
open http://localhost:8080
```

### テスト
```bash
# 全テストを実行（7つのテストスイート）
make test

# カバレッジ付きテストを実行
make test-coverage

# 特定のパッケージのテスト
go test ./internal/strategy -v
go test ./internal/config -v
go test ./internal/database -v
go test ./internal/logger -v
go test ./internal/api -v

# ベンチマークテスト
go test -bench=. ./internal/strategy
```

### コード品質
```bash
# リンターを実行（全ルール対応済み）
make lint

# 対応済みリンタールール:
# - gocritic (hugeParam, rangeValCopy)
# - misspell
# - 型安全性チェック
# - エラーハンドリング検証
```

### デバッグ
```bash
# デバッグモードで実行
make dev

# 詳細ログ付きで実行
./bin/gogocoin -mode paper -log-level debug

# ログファイルを監視
tail -f logs/gogocoin.log

# API エンドポイントをテスト
curl -s http://localhost:8080/api/status | jq .
curl -s http://localhost:8080/api/performance | jq .
```

## 貢献

貢献を歓迎します！イシューやプルリクエストをお気軽に提出してください。

### 開発ガイドライン
1. **コード品質**: Goのベストプラクティスに従う
2. **テスト**: 新機能には必ずテストを書く（カバレッジ80%以上維持）
3. **ドキュメント**: README.md・コメント・API仕様を更新する
4. **品質チェック**: 提出前に必ずリンター・テストを実行する
5. **コミット**: 明確なコミットメッセージを書く

### プルリクエスト手順
```bash
# 1. フォーク・クローン
git clone https://github.com/your-username/gogocoin.git
cd gogocoin

# 2. ブランチ作成
git checkout -b feature/your-feature-name

# 3. 開発・テスト
make lint
make test
make test-coverage

# 4. コミット・プッシュ
git add .
git commit -m "feat: add your feature description"
git push origin feature/your-feature-name

# 5. プルリクエスト作成
```

### コードレビュー基準
- ✅ **リンター**: 全ルール通過必須
- ✅ **テスト**: 新機能・修正箇所のテスト追加
- ✅ **カバレッジ**: 80%以上維持
- ✅ **ドキュメント**: 適切な更新
- ✅ **動作確認**: ペーパーモードでの動作テスト

## ライセンス

このプロジェクトはMITライセンスの下でライセンスされています - 詳細は[LICENSE](LICENSE)ファイルを参照してください。

## サポート

このプロジェクトが役に立つと思われる場合は、以下をご検討ください:
- ⭐ リポジトリにスターを付ける
- 🐛 バグを報告する
- 💡 機能を提案する
- 🤝 コードを貢献する
- 📝 ドキュメントの改善
- 🧪 テストケースの追加

### 📊 **プロジェクト状況**
- **開発状況**: 本番準備完了 🚀
- **安定性**: ペーパー・ライブモード完全動作 ✅
- **取引制御**: WebUI・API経由でリアルタイム制御 🎛️
- **テストカバレッジ**: 59%+（132テスト） 📈
- **コード品質**: Linter 0 issues 🔧
- **ドキュメント**: 完全なAPI仕様 📚
- **本番対応**: ライブモード完全実装 💰

### 🎯 **今後の予定**
- [x] ライブモード完全実装（完了）
- [x] 残高チェック機能（完了）
- [x] 部分約定対応（完了）
- [x] 取引開始・停止制御（完了）
- [ ] 取引状態の永続化（再起動時復元）
- [ ] 追加戦略の実装
- [ ] パフォーマンス最適化
- [ ] モバイルUI改善
- [ ] 多取引所対応検討

## 謝辞

- [go-bitflyer-api-client](https://github.com/bmf-san/go-bitflyer-api-client) - bitFlyer APIクライアントライブラリ
- 取引APIを提供するbitFlyer