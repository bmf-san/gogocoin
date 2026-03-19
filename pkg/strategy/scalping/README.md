# Scalping Strategy

EMAベースのステートレス・スキャルピング戦略。gogocoin 同梱のデフォルト戦略として提供される。

---

## 特徴

| 項目 | 内容 |
|---|---|
| 設計 | ステートレス設計: 再起動に強い（内部状態を最小限に保持） |
| インジケーター | 短期EMA と中期EMA のクロスオーバー |
| RSI フィルタ | オプション。`rsi_period > 0` で有効化 |
| リスク/リワード比 | デフォルト 2:1（利確2.0% / 損切1.0%） |
| クールダウン | 取引後のインターバル（デフォルト90秒） |
| 手数料考慮 | 取引手数料を考慮した損益計算 |

---

## シグナル生成ロジック

| シグナル | 条件 |
|---|---|
| 買い（BUY） | 短期EMA > 中期EMA かつ 現在価格 > 短期EMA |
| 売り（SELL） | 短期EMA < 中期EMA かつ 現在価格 < 短期EMA |
| 待機（HOLD） | 上記以外、またはクールダウン中、または日次制限到達時 |

### RSI フィルタ（オプション）

`rsi_period` を 0 以外に設定すると RSI フィルタが有効になる。上記 EMA 条件に加え、以下の条件も満たす場合のみシグナルが発行される:

| シグナル | RSI 条件 |
|---|---|
| BUY | RSI < `rsi_overbought`（買われ過ぎでない） |
| SELL | RSI > `rsi_oversold`（売られ過ぎでない） |

---

## 設定パラメータ（`strategy_params.scalping`）

`config.yaml` の `strategy_params.scalping` ブロックで設定する。

| キー | デフォルト | 説明 |
|---|---|---|
| `ema_fast_period` | `9` | 短期EMAの期間（バー数） |
| `ema_slow_period` | `21` | 中期EMAの期間（バー数） |
| `take_profit_pct` | `2.0` | 利確ライン（%） |
| `stop_loss_pct` | `1.0` | 損切ライン（%）。リスク/リワード比 = 1:2 |
| `cooldown_sec` | `90` | 取引後のクールダウン時間（秒）。過剰取引を防止 |
| `max_daily_trades` | `3` | 1日の最大取引回数（保守的運用のデフォルト） |
| `order_notional` | `200` | **注文金額（JPY）。この値が実際の注文サイズになる** |
| `auto_scale_enabled` | `false` | `true` の場合、BUY時に残高連動で `order_notional` を自動拡大 |
| `auto_scale_balance_pct` | `80` | `auto_scale_enabled=true` 時の目標割合（`JPY残高 × この%`） |
| `auto_scale_max_notional` | `0` | 自動拡大時の上限。`0` は上限なし |
| `fee_rate` | `0.001` | 手数料率（損益計算に使用） |
| `rsi_period` | `0` | RSIの期間。`0` で RSI フィルタは無効 |
| `rsi_overbought` | `70` | RSI 買われ過ぎしきい値。超えると BUY を抑制 |
| `rsi_oversold` | `30` | RSI 売られ過ぎしきい値。下回ると SELL を抑制 |

### `order_notional` について

`order_notional` は1回あたりの**注文金額（JPY）**を指定する。
戦略は `order_notional / 現在価格` で数量を計算する。残高の何%を使うかで自動算出する機能はない。

1トレードあたりの利益の目安: `order_notional × take_profit_pct / 100 − 往復手数料`

例（`order_notional: 4000`, `take_profit_pct: 2.0`, `fee_rate: 0.0015`）:
- 利確時グロス: 4000 × 0.02 = 80 JPY
- 往復手数料: 4000 × 0.0015 × 2 = 12 JPY
- 純利益: ~68 JPY

### 自動スケール（`auto_scale_*`）

`auto_scale_enabled: true` の場合、BUYシグナル時に次の式で注文金額を再計算する。

- ベース: `order_notional`（または `symbol_params.<symbol>.order_notional`）
- 目標: `JPY残高 × auto_scale_balance_pct / 100`
- 実際の注文金額: `max(ベース, 目標)` をベースに、`auto_scale_max_notional` と手数料込みの残高上限でクランプ

これにより、残高が増えると注文金額が自動で増加し、手動で `order_notional` を更新しなくても複利的にスケールできる。

### symbol_params（シンボル個別オーバーライド）

通貨ペアごとに一部パラメータをオーバーライドできる。0 または未設定の場合はグローバル設定にフォールバックする。

| キー | 説明 |
|---|---|
| `ema_fast_period` | 短期EMA期間 |
| `ema_slow_period` | 中期EMA期間 |
| `cooldown_sec` | クールダウン秒数 |
| `order_notional` | 注文金額 |

---

## 設定例

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

## パラメータ調整ガイド

### EMA期間

`ema_fast_period < ema_slow_period` を必ず守ること。

- `ema_fast_period` を小さくする → 反応が早い（ノイズも増える）
- `ema_slow_period` を大きくする → トレンド追従型になる

### 利確・損切

リスク/リワード比を 2:1 以上に保つことを推奨（`take_profit_pct >= stop_loss_pct * 2`）。

### 注文金額と利益の関係

| `order_notional` | 利確2%時のグロス | 往復手数料(0.15%) | 純利益 |
|---|---|---|---|
| 200 | 4 JPY | 0.6 JPY | ~3 JPY |
| 1000 | 20 JPY | 3 JPY | ~17 JPY |
| 4000 | 80 JPY | 12 JPY | ~68 JPY |
| 7000 | 140 JPY | 21 JPY | ~119 JPY |

### 取引頻度

取引頻度を上げる場合は `max_daily_trades` を増やし `cooldown_sec` を短くする。ただし手数料コストが増加するため、損益分岐点の試算を行うこと。

---

## 推奨設定

| 項目 | 推奨値 | 理由 |
|---|---|---|
| 通貨ペア | XRP_JPY | 少額取引に最適（最小1 XRP） |
| 稼働形態 | 24/7稼働 | 常時監視で機会を逃さない |
| 取引頻度 | `max_daily_trades` で調整 | デフォルト3回は保守的な設定 |
