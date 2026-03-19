# 取引戦略リファレンス

## プラガブル戦略アーキテクチャ

gogocoin の戦略は**プラガブル**設計になっており、`pkg/strategy.Strategy` インターフェースを実装することで独自戦略を差し込めます。

- カスタム戦略の実装手順: [DESIGN_DOC.md § 5](DESIGN_DOC.md)
- Strategy インターフェース定義: `pkg/strategy/strategy.go`

## 同梱戦略

| 戦略名 | パッケージ | 説明 |
|---|---|---|
| `scalping` | `pkg/strategy/scalping` | EMA クロスオーバー + RSI フィルタによるスキャルピング戦略 |

各戦略の設定パラメータ・シグナル生成ロジックの詳細は、パッケージ内の README を参照してください:

- [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md)

## 免責事項

実際の取引成績は市場環境や設定により大きく変動します。過去のバックテスト結果は将来の成績を保証するものではありません。
