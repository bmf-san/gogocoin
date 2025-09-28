# gogocoin アーキテクチャ設計

## 現在の課題と改善計画

### 🚨 現在の問題点

#### 1. paper/liveモードの実装が混在
**問題**:
- `TradingService`が両方のモードを単一クラスで処理
- 各メソッドで`if ts.paperTrade`判定を繰り返し
- 共通処理と固有処理が混在し、保守性が低い

**具体例**:
```go
func (ts *TradingService) PlaceOrder(ctx context.Context, order *OrderRequest) (*OrderResult, error) {
    if ts.paperTrade {
        return ts.placePaperOrder(ctx, order)
    }
    return ts.placeLiveOrder(ctx, order)
}

func (ts *TradingService) GetBalance(ctx context.Context) ([]Balance, error) {
    if ts.paperTrade {
        return ts.getPaperBalance(), nil
    }
    return ts.getLiveBalance(ctx)
}
```

#### 2. 責任の不明確さ
- `TradingService`が以下を全て担当：
  - 注文実行（paper/live両方）
  - 残高管理（メモリ/API）
  - PnL計算（paperのみ）
  - データベース保存
  - ログ出力

#### 3. テスト性の低さ
- モードの切り替えが初期化時のみ
- モック化が困難
- 各モードを独立してテストしにくい

---

## ✅ 改善された設計（提案）

### アーキテクチャパターン: Strategy Pattern + Dependency Injection

```
┌─────────────────────────────────────────┐
│         TradingService (interface)       │
│  - PlaceOrder()                          │
│  - CancelOrder()                         │
│  - GetBalance()                          │
│  - GetOrders()                           │
└─────────────────────────────────────────┘
          ▲                    ▲
          │                    │
          │                    │
┌─────────┴──────────┐  ┌─────┴────────────┐
│ PaperTradingService│  │ LiveTradingService│
│                    │  │                    │
│ - paperOrders      │  │ - apiClient        │
│ - paperBalance     │  │ - orderMonitor     │
│ - pnlCalculator    │  │ - balanceCache     │
└────────────────────┘  └────────────────────┘
```

### 設計原則

#### 1. 単一責任の原則 (SRP)
各サービスは1つのモードのみを担当：
- `PaperTradingService`: シミュレーション取引
- `LiveTradingService`: 実際のAPI取引

#### 2. 依存性逆転の原則 (DIP)
インターフェースに依存：
```go
type TradingService interface {
    PlaceOrder(ctx context.Context, order *OrderRequest) (*OrderResult, error)
    CancelOrder(ctx context.Context, orderID string) error
    GetBalance(ctx context.Context) ([]Balance, error)
    GetOrders(ctx context.Context) ([]*OrderResult, error)
}
```

#### 3. 開放閉鎖の原則 (OCP)
新しいモード（例: TestMode, SimulationMode）を既存コード変更なしで追加可能

---

## 📦 新しいパッケージ構成

```
internal/
├── trading/                      # 取引サービス（新設）
│   ├── service.go               # TradingService interface
│   ├── paper/
│   │   ├── service.go           # PaperTradingService
│   │   ├── balance.go           # 残高管理
│   │   ├── order.go             # 注文処理
│   │   └── pnl.go               # 損益計算
│   ├── live/
│   │   ├── service.go           # LiveTradingService
│   │   ├── balance.go           # API残高取得
│   │   ├── order.go             # API注文実行
│   │   └── monitor.go           # 約定監視
│   └── factory.go               # Factory pattern
├── bitflyer/                     # bitFlyer APIクライアント
│   ├── client.go                # HTTPクライアント
│   ├── market_data.go           # 市場データ
│   └── types.go                 # 共通型定義
└── ...
```

---

## 🔧 実装計画

### Phase 1: インターフェース定義（最優先）
- [ ] `TradingService`インターフェースの明確化
- [ ] 共通型（`OrderRequest`, `OrderResult`, `Balance`）の整理
- [ ] テストダブルの準備

### Phase 2: PaperTradingService分離
- [ ] `internal/trading/paper/`パッケージ作成
- [ ] 既存の`placePaperOrder`等をリファクタリング
- [ ] PnL計算ロジックの分離
- [ ] ユニットテスト追加

### Phase 3: LiveTradingService分離
- [ ] `internal/trading/live/`パッケージ作成
- [ ] 既存の`placeLiveOrder`等をリファクタリング
- [ ] API呼び出しロジックの整理
- [ ] 約定監視の独立化

### Phase 4: Factory導入
- [ ] `NewTradingService(mode string, ...)`の実装
- [ ] 依存性注入の整理
- [ ] `Application`での利用

### Phase 5: 既存コードの移行
- [ ] `internal/bitflyer/trading.go`の段階的削除
- [ ] 全テストの更新
- [ ] ドキュメント更新

---

## 📊 メリット

### コード品質
✅ **保守性**: 各モードが独立して変更可能
✅ **可読性**: モード判定分岐が消え、コードが明確
✅ **テスト性**: 各サービスを独立してテスト可能

### 拡張性
✅ **新モード追加**: 既存コード変更なし
✅ **機能追加**: 各サービスに閉じた変更
✅ **バグ修正**: 影響範囲が明確

### パフォーマンス
✅ **不要なチェック削減**: `if ts.paperTrade`の繰り返しを削除
✅ **メモリ効率**: liveモード時にpaper用メモリ不要

---

## 🎯 現状維持のリスク

❌ **保守コスト増大**: 複雑なif文の増加
❌ **バグの温床**: モード間の処理漏れ
❌ **テスト困難**: 全パターンのテストが必要
❌ **拡張困難**: 新機能追加時の影響範囲が広い

---

## 📝 移行戦略

### 段階的リファクタリング
1. **インターフェース先行**: まずインターフェースを定義
2. **Paper分離**: リスクの低いpaperモードから移行
3. **Live移行**: 慎重にliveモードを移行
4. **並行運用**: 新旧両方のコードを一時的に維持
5. **段階的削除**: 十分なテスト後に旧コードを削除

### リスク軽減
- 既存のテストをすべてパスすることを確認
- 新旧両実装で同じ結果を返すことを検証
- ペーパーモードで十分にテストしてからライブモードを移行

---

## 📅 スケジュール（目安）

- **Phase 1-2**: 1-2日（Paper分離まで）
- **Phase 3**: 1日（Live分離）
- **Phase 4**: 半日（Factory導入）
- **Phase 5**: 1日（移行完了）

**合計**: 3-4日での完全な設計改善が可能

---

## 🚀 次のアクション

1. この設計を確認・承認
2. Phase 1のインターフェース定義から開始
3. 段階的にリファクタリングを進める
4. 各Phaseでのテスト・検証を徹底

**注**: 現在の実装は動作しているため、急ぐ必要はありません。
ただし、長期的な保守性のために、早めの改善を推奨します。

