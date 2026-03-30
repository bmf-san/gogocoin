# gogocoin

[![CI](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/ci.yml)
[![Release](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml/badge.svg)](https://github.com/bmf-san/gogocoin/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bmf-san/gogocoin)](https://goreportcard.com/report/github.com/bmf-san/gogocoin)
[![GitHub license](https://img.shields.io/github/license/bmf-san/gogocoin)](https://github.com/bmf-san/gogocoin/blob/main/LICENSE)
[![GitHub release](https://img.shields.io/github/release/bmf-san/gogocoin.svg)](https://github.com/bmf-san/gogocoin/releases)

bitFlyer取引所向けの暗号通貨取引ボット

<img src="./docs/assets/icon.png" alt="gogocoin" title="gogocoin" width="100px">

This logo was created by [gopherize.me](https://gopherize.me/gopher/c3ef0a34f257bb18ea3b9b5a3ada0b1a0573e431).

## 概要

gogocoinは、bitFlyer暗号通貨取引所向けのGo言語製自動取引ボットです。EMAベースのスキャルピング戦略を使用し、設定可能な取引頻度で自動取引を実行します。

### 機能一覧

- **プラガブル戦略アーキテクチャ**: `pkg/strategy.Strategy` インターフェースを実装することで独自の取引戦略を差し込み可能
- 同梱デフォルト戦略: EMAクロスオーバー + RSI フィルタによるスキャルピング戦略
- リスク管理（利確・ストップロス・1日の取引回数制限・クールダウン）
- WebUIによる取引の開始・停止制御
- WebSocketによるリアルタイム市場データ取得・分析
- WebダッシュボードによるリアルタイムモニタリングUI（`http://localhost:8080`）
- SQLiteによるデータ永続化
- 取引データの自動クリーンアップ（`retention_days` で保持日数を設定可能）
- 構造化ログ（レベル別・カテゴリ別フィルタリング対応）
- 24/7稼働対応（冪等性・再起動耐性）

### スクリーンショット

![gogocoin Dashboard](./docs/assets/screenshot-dashboard.png)

### 技術スタック

- **言語**: Go 1.23以上（開発環境: Go 1.25.0）
- **依存関係**: 最小限（go-bitflyer-api-client + yaml.v3 + sqlite3のみ）
- **アーキテクチャ**: レイヤー分離されたモジュラーアーキテクチャ
- **公開API** (`pkg/`): `pkg/engine.Run()` + `pkg/strategy.Strategy` インターフェースにより外部リポジトリからの戦略差し込みが可能。セマンティックバージョニング対象の安定API
- **データベース**: SQLite（軽量・埋め込み・外部DB不要）
  - DB保持: `retention_days` で設定可能（デフォルト1日）
  - 過去データ: bitFlyerで確認可能
- **並行処理**: Goroutines + Channels による非同期ワーカー
- **通信**: WebSocket（リアルタイム） + REST API（Web UI）
- **ログ**: 標準log/slogベースの構造化ログ
  - 高頻度ログフィルタリング（DEBUGレベル・dataカテゴリの2種類）
  - DBインデックス最適化（timestamp DESC）
- **パフォーマンス最適化**:
  - バランスキャッシュ（60秒TTL、APIコール90%削減）
  - 429エラー98%削減
  - デッドロック防止設計
- **デプロイ**: 埋め込みWebアセット付きシングルバイナリ
- **品質保証**:
  - 静的解析ツール対応（golangci-lint）
  - 複数パッケージにわたるユニットテスト
  - モジュラーアーキテクチャ（レイヤー分離設計）
  - 型安全性（Go言語の型システム活用）
  - エラーハンドリング（適切な例外処理）

## 免責事項

**重要: 必ずお読みください**

**このソフトウェアは情報提供および開発目的でのみ提供されており、金融アドバイスや投資判断を構成することを意図していません。暗号通貨取引は高リスクであり、投資元本を失う可能性があります。**

**実際の取引成績は市場環境、設定、タイミング等により大きく変動します。過去のバックテスト結果やシミュレーション結果は将来の成績を保証するものではありません。**

**このソフトウェアの使用により生じるいかなる損失や損害についても、作者は一切の責任を負いません。ご自身の判断と責任において使用してください。**

**このライブラリはbitFlyerと一切関係ありません。使用前に各APIプロバイダーの利用規約を確認してください。**

**このライブラリは「現状のまま」提供され、正確性、完全性、将来の互換性についていかなる保証もありません。**

## クイックスタート

gogocoin には2つの使い方があります。

### A. ライブラリとして使う（推奨）

gogocoin を `go get` して自分のリポジトリに組み込む方法です。独自の取引戦略を実装して使えます。

```bash
go get github.com/bmf-san/gogocoin@latest
```

`example/` ディレクトリに動作するサンプルがあります。詳細は [example ディレクトリの使い方](#example-ディレクトリの使い方) を参照してください。

### B. Docker で素早く試す（動作確認・開発向け）

> **注意**: この方法でビルドされるバイナリは戦略が登録されていないため、実際のトレードは行えません。動作確認・開発目的専用です。

#### 前提条件

- Docker と Docker Compose
- bitFlyer APIキー（[管理画面](https://bitflyer.com/ja-jp/api)で取得）

> ローカル開発（Docker なし）の場合は Go 1.25.0 以上が必要です。

#### セットアップ

```bash
# 1. リポジトリのクローン
git clone https://github.com/bmf-san/gogocoin.git
cd gogocoin

# 2. 環境変数の設定
cp .env.example .env
# .envファイルを編集してAPIキーを設定

# 3. 設定ファイルの作成
make init

# 4. 起動
make up

# 5. Web UIにアクセス
open http://localhost:8080
```

#### .envファイルの設定例

```bash
BITFLYER_API_KEY=your_actual_api_key_here
BITFLYER_API_SECRET=your_actual_api_secret_here
```

**⚠️ 注意**: このボットはライブトレードのみ対応しています。実資金を使用するため、設定を十分に確認してから使用してください。

#### コンテナ管理

```bash
make logs     # ログ確認
make down     # 停止
make restart  # 再起動
make rebuild  # 再ビルド
```

## example ディレクトリの使い方

`example/` は gogocoin をライブラリとして使う際の完全な動作サンプルです。独自リポジトリを作る際の出発点として使えます。

### 構成

```
example/
├── cmd/
│   └── main.go                  # エントリーポイント (blank import で戦略登録)
├── strategy/scalping/
│   ├── params.go                # 戦略パラメータ定義
│   ├── strategy.go              # 戦略実装 (EMA + RSI + クールダウン)
│   └── register.go              # init() による自動登録
├── configs/
│   └── config.example.yaml      # 設定ファイルのテンプレート
├── go.mod                       # 独立した Go モジュール
└── Makefile                     # build / run ショートカット
```

### 動かし方

```bash
cd example

# 1. 設定ファイルを作成
cp configs/config.example.yaml configs/config.yaml
# configs/config.yaml を編集して API キーを設定

# 2. 実行
export BITFLYER_API_KEY=your_key
export BITFLYER_API_SECRET=your_secret
make run
# または: go run ./cmd/
```

### 独自リポジトリへの移植

`example/` をそのままコピーして自分のリポジトリとして使うか、以下のパターンを参考に実装してください。

**1. `go.mod` を作成**

```bash
go mod init github.com/yourname/your-bot
go get github.com/bmf-san/gogocoin@latest
```

**2. 戦略を実装して `init()` で登録**

```go
// strategy/scalping/register.go
package scalping

import "github.com/bmf-san/gogocoin/pkg/strategy"

func init() {
    strategy.Register("scalping", func() strategy.Strategy {
        return NewDefault()
    })
}
```

**3. `main.go` で blank import**

```go
import (
    "github.com/bmf-san/gogocoin/pkg/engine"
    _ "github.com/yourname/your-bot/strategy/scalping" // init() を呼ぶ
)

func main() {
    engine.Run(ctx, engine.WithConfigPath("./configs/config.yaml"))
}
```

> 参考実装: [bmf-san/my-gogocoin](https://github.com/bmf-san/my-gogocoin)

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/CONFIG.md](docs/CONFIG.md) | 設定リファレンス |
| [docs/STRATEGY.md](docs/STRATEGY.md) | 取引戦略リファレンス（プラガブルアーキテクチャ概要・同梱戦略一覧） |
| [docs/DESIGN_DOC.md](docs/DESIGN_DOC.md) | アーキテクチャ設計ドキュメント（**カスタム戦略の実装方法** § 5） |
| [docs/DATA_MANAGEMENT.md](docs/DATA_MANAGEMENT.md) | データ管理リファレンス |
| [docs/openapi.yaml](docs/openapi.yaml) | API仕様（OpenAPI 3.1） |

## Web UI

ブラウザで取引状況をリアルタイム監視できます: `http://localhost:8080`

取引の開始・停止もWeb UIから操作できます。

## 運用

### 推奨運用

1. Docker volume で `./data/` を永続化（設定済み）
2. 週1回程度の再起動で安定性向上
3. ログレベルは `info` を推奨（`debug` は開発時のみ）

### トラブルシューティング

- ログ確認: `make logs` または `docker compose logs -f`
- DB状態確認: `ls -lh ./data/gogocoin.db`
- コンテナ再起動: `make restart`

## 開発

### ローカル開発

```bash
# 依存関係インストール
make deps

# 開発ツールインストール（golangci-lint・oapi-codegen 等）
make install-tools

# テスト実行
make test

# カバレッジ確認
make test-coverage

# コードフォーマット
make fmt

# リンター実行
make lint

# Docker経由で実行
make up

```

### API コード生成

`docs/openapi.yaml` を変更した場合は、`oapi-codegen` でコードを再生成してコミットしてください。

```bash
# api.gen.go を再生成
make generate

```

> `internal/api/api.gen.go` は自動生成ファイルです。直接編集せず、必ず `make generate` 経由で更新してください。
> CI の `codegen` ジョブが spec と生成コードの同期を検証します。

## 関連

- [example/](example/) — gogocoin をライブラリとして使う動作サンプル（このリポジトリ内）
- [bmf-san/my-gogocoin](https://github.com/bmf-san/my-gogocoin) — gogocoin を使った実際の運用リポジトリ例
- [gogocoin-vps-template](https://github.com/bmf-san/gogocoin-vps-template) — VPS（ConoHa 等）に systemd + GitHub Actions でデプロイする運用構成のテンプレート

## コントリビューション

[CONTRIBUTING.md](.github/CONTRIBUTING.md) を参照してください。
