# 取引戦略リファレンス

## プラガブル戦略アーキテクチャ

gogocoin の戦略は**プラガブル**設計になっており、`pkg/strategy.Strategy` インターフェースを実装することで独自戦略を差し込めます。

- カスタム戦略の実装手順: [DESIGN_DOC.ja.md § 5](DESIGN_DOC.ja.md)
- Strategy インターフェース定義: `pkg/strategy/strategy.go`

## 同梱戦略

| 戦略名 | パッケージ | 説明 |
|---|---|---|
| `scalping` | `pkg/strategy/scalping` | EMA クロスオーバーによる最小スキャルピング戦略（リファレンス実装 — RSI / クールダウン / 日次上限は含まない） |

各戦略の設定パラメータ・シグナル生成ロジックの詳細は、パッケージ内の README を参照してください:

- [pkg/strategy/scalping/README.md](../pkg/strategy/scalping/README.md)

## エンジンレベルのストップロス

ストップロスは `StrategyWorker` が毎ティックで強制適用します。戦略のシグナル出力とは独立して動作します。

各ティックで戦略がシグナルを生成した後、`StrategyWorker` はDBからオープン中のBUYポジションを照会します。現在価格がいずれかのポジションのストップ価格以下に下落した場合、戦略の出力に関わらず `SELL` シグナルが注入されます。

```
stop_price = entry_price × (1 - stop_loss_pct / 100)
```

これにより、戦略が `HOLD` や `BUY` を返している場合でもストップロスが発火し、閾値を超えた瞬間に損失ポジションを決済します。

`stop_loss_pct` の値は戦略の設定から読み込まれます（`config.yaml` の `strategy_params.scalping.stop_loss_pct`）。`0` に設定するとストップロスチェックが無効になります。

## 免責事項

実際の取引成績は市場環境や設定により大きく変動します。過去のバックテスト結果は将来の成績を保証するものではありません。
