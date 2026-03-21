package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/bmf-san/gogocoin/internal/domain"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockTradingEnabledGetter struct {
	enabled bool
}

func (m *mockTradingEnabledGetter) IsTradingEnabled() bool { return m.enabled }

type mockRiskChecker struct {
	err error
}

func (m *mockRiskChecker) CheckRiskManagement(_ context.Context, _ *strategy.Signal) error {
	return m.err
}

type mockTrader struct {
	mu           sync.Mutex
	balances     []domain.Balance
	balanceErr   error
	placeOrderFn func(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error)
	orders       []*domain.OrderRequest
}

func (m *mockTrader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	m.mu.Lock()
	m.orders = append(m.orders, order)
	m.mu.Unlock()
	if m.placeOrderFn != nil {
		return m.placeOrderFn(ctx, order)
	}
	return &domain.OrderResult{OrderID: "test-order-id"}, nil
}

func (m *mockTrader) GetBalance(_ context.Context) ([]domain.Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balances, m.balanceErr
}

type mockPerformanceUpdater struct {
	err error
}

func (m *mockPerformanceUpdater) UpdateMetrics(_ context.Context) error { return m.err }

type mockLotSizeService struct {
	sizes map[string]float64
}

func (m *mockLotSizeService) GetMinimumOrderSize(symbol string) (float64, error) {
	if m.sizes != nil {
		if s, ok := m.sizes[symbol]; ok {
			return s, nil
		}
	}
	return 0, fmt.Errorf("unknown symbol: %s", symbol)
}

// mockSignalStrategy wraps BaseStrategy and exposes a configurable config map.
type mockSignalStrategy struct {
	*strategy.BaseStrategy
	cfg map[string]interface{}
}

func newMockStrategy(cfg map[string]interface{}) *mockSignalStrategy {
	return &mockSignalStrategy{
		BaseStrategy: strategy.NewBaseStrategy("mock", "mock strategy", "0.0.1"),
		cfg:          cfg,
	}
}

func (m *mockSignalStrategy) GetConfig() map[string]interface{} { return m.cfg }
func (m *mockSignalStrategy) GenerateSignal(_ context.Context, _ *strategy.MarketData, _ []strategy.MarketData) (*strategy.Signal, error) {
	return nil, nil
}
func (m *mockSignalStrategy) Analyze(_ []strategy.MarketData) (*strategy.Signal, error) { return nil, nil }
func (m *mockSignalStrategy) Initialize(_ map[string]interface{}) error                  { return nil }
func (m *mockSignalStrategy) UpdateConfig(cfg map[string]interface{}) error {
	m.cfg = cfg
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestWorker(t *testing.T, trader *mockTrader, strat strategy.Strategy, tradingEnabled bool) *SignalWorker {
	t.Helper()
	log := createTestLogger(t)
	signalCh := make(chan *strategy.Signal)
	return &SignalWorker{
		logger:               log,
		signalCh:             signalCh,
		tradingEnabledGetter: &mockTradingEnabledGetter{enabled: tradingEnabled},
		riskChecker:          &mockRiskChecker{},
		trader:               trader,
		currentStrategy:      strat,
		performanceUpdater:   &mockPerformanceUpdater{},
		lotSizeSvc: &mockLotSizeService{sizes: map[string]float64{
			"BTC_JPY": 0.001,
			"ETH_JPY": 0.01,
			"XRP_JPY": 1.0,
			"XLM_JPY": 10.0,
		}},
		sellSizePercentage: 0.95,
	}
}

func buySignal(price, qty float64) *strategy.Signal {
	return &strategy.Signal{
		Symbol:   "BTC_JPY",
		Action:   strategy.SignalBuy,
		Price:    price,
		Quantity: qty,
		Strength: 1.0,
	}
}

// ---------------------------------------------------------------------------
// Tests: computeScaledNotional
// ---------------------------------------------------------------------------

func TestComputeScaledNotional(t *testing.T) {
	tests := []struct {
		name         string
		base         float64
		available    float64
		cfg          autoScaleConfig
		wantMin      float64
		wantMax      float64
	}{
		{
			name:      "disabled returns base",
			base:      8000,
			available: 100000,
			cfg:       autoScaleConfig{enabled: false, balancePct: 5, feeRate: 0.0015},
			wantMin:   8000,
			wantMax:   8000,
		},
		{
			name:      "scale up when target > base",
			base:      8000,
			available: 200000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 5, feeRate: 0.0015},
			// target = 200000 * 5/100 = 10000; affordable = 200000/1.0015 ≈ 199700
			wantMin: 9900,
			wantMax: 10100,
		},
		{
			name:      "stay at base when target < base",
			base:      8000,
			available: 100000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 3, feeRate: 0.0015},
			// target = 100000 * 3/100 = 3000 < base → stays at 8000
			wantMin: 7900,
			wantMax: 8100,
		},
		{
			name:      "clamped by maxNotional",
			base:      8000,
			available: 200000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 10, maxNotional: 12000, feeRate: 0.0015},
			// target = 200000 * 10/100 = 20000; capped to 12000
			wantMin: 11900,
			wantMax: 12100,
		},
		{
			name:      "clamped by affordable (low balance)",
			base:      8000,
			available: 5000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 100, feeRate: 0.0015},
			// target = 5000; affordable = 5000/1.0015 ≈ 4993
			wantMin: 4980,
			wantMax: 5000,
		},
		{
			name:      "zero available returns 0",
			base:      8000,
			available: 0,
			cfg:       autoScaleConfig{enabled: true, balancePct: 5},
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "zero base returns 0",
			base:      0,
			available: 100000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 5},
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "maxNotional=0 means no cap",
			base:      8000,
			available: 1000000,
			cfg:       autoScaleConfig{enabled: true, balancePct: 10, maxNotional: 0, feeRate: 0.0015},
			// target = 100000; affordable = 1000000/1.0015 ≈ 998500
			wantMin: 99000,
			wantMax: 101000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeScaledNotional(tc.base, tc.available, tc.cfg)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("computeScaledNotional(%v, %v, %+v) = %v; want %v..%v",
					tc.base, tc.available, tc.cfg, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: applyAutoScaleToBuySignal
// ---------------------------------------------------------------------------

func TestApplyAutoScaleToBuySignal_Disabled(t *testing.T) {
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 100000}}}
	strat := newMockStrategy(map[string]interface{}{
		"auto_scale_enabled": false,
	})
	w := newTestWorker(t, trader, strat, true)

	sig := buySignal(5000000, 0.001) // qty = 8000 notional / 5000000 price
	origQty := sig.Quantity

	ok := w.applyAutoScaleToBuySignal(context.Background(), sig)
	if !ok {
		t.Fatal("expected ok=true when auto_scale disabled")
	}
	if sig.Quantity != origQty {
		t.Errorf("quantity should not change when disabled: got %v, want %v", sig.Quantity, origQty)
	}
}

func TestApplyAutoScaleToBuySignal_ScalesUp(t *testing.T) {
	// 200,000 JPY available; balancePct=5; base=8000 → target=10000
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 200000}}}
	strat := newMockStrategy(map[string]interface{}{
		"auto_scale_enabled":     true,
		"auto_scale_balance_pct": float64(5),
		"fee_rate":               float64(0.0015),
	})
	w := newTestWorker(t, trader, strat, true)

	price := 8000000.0
	origQty := 8000.0 / price // base notional = 8000
	sig := buySignal(price, origQty)

	ok := w.applyAutoScaleToBuySignal(context.Background(), sig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	effectiveNotional := sig.Quantity * price
	// After lot-size floor rounding the order may not exceed the base notional in JPY
	// (e.g. BTC lot 0.001 * 8_000_000 = 8000 JPY is the minimum increment).
	// Verify the quantity is a valid lot multiple and is not reduced below the base.
	if effectiveNotional < 8000 {
		t.Errorf("expected effective notional >= base (8000), got %v", effectiveNotional)
	}
	if effectiveNotional > 11000 {
		t.Errorf("effective notional %v unreasonably high", effectiveNotional)
	}
	// quantity must be a multiple of the BTC lot size (0.001)
	const btcLot = 0.001
	if sig.Quantity < btcLot || math.Abs(math.Mod(sig.Quantity, btcLot)) > 1e-9 {
		t.Errorf("quantity %v is not aligned to lot size %v", sig.Quantity, btcLot)
	}
	// Metadata should be populated
	if sig.Metadata == nil {
		t.Error("expected Metadata set")
	}
	if _, ok := sig.Metadata["order_notional_effective"]; !ok {
		t.Error("expected order_notional_effective in Metadata")
	}
}

func TestApplyAutoScaleToBuySignal_CappedByMaxNotional(t *testing.T) {
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 1000000}}}
	strat := newMockStrategy(map[string]interface{}{
		"auto_scale_enabled":      true,
		"auto_scale_balance_pct":  float64(10), // target = 100000
		"auto_scale_max_notional": float64(20000),
		"fee_rate":                float64(0.0015),
	})
	w := newTestWorker(t, trader, strat, true)

	price := 8000000.0
	sig := buySignal(price, 8000/price)

	ok := w.applyAutoScaleToBuySignal(context.Background(), sig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	effectiveNotional := sig.Quantity * price
	if effectiveNotional > 20100 {
		t.Errorf("expected notional capped at ~20000, got %v", effectiveNotional)
	}
}

func TestApplyAutoScaleToBuySignal_SkipOnInsufficientBalance(t *testing.T) {
	// Only 3000 JPY available but base notional = 8000
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 3000}}}
	strat := newMockStrategy(map[string]interface{}{
		"auto_scale_enabled":     true,
		"auto_scale_balance_pct": float64(5),
		"fee_rate":               float64(0.0015),
	})
	w := newTestWorker(t, trader, strat, true)

	price := 8000000.0
	sig := buySignal(price, 8000/price) // base notional = 8000, but only 3000 available

	ok := w.applyAutoScaleToBuySignal(context.Background(), sig)
	if ok {
		t.Error("expected ok=false when balance below base notional")
	}
}

func TestApplyAutoScaleToBuySignal_SkipOnBalanceFetchError(t *testing.T) {
	trader := &mockTrader{balanceErr: errors.New("exchange unavailable")}
	strat := newMockStrategy(map[string]interface{}{
		"auto_scale_enabled":     true,
		"auto_scale_balance_pct": float64(5),
	})
	w := newTestWorker(t, trader, strat, true)

	sig := buySignal(5000000, 0.001)

	ok := w.applyAutoScaleToBuySignal(context.Background(), sig)
	if ok {
		t.Error("expected ok=false when balance fetch fails")
	}
}

func TestApplyAutoScaleToBuySignal_InvalidPriceOrQty(t *testing.T) {
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 100000}}}
	strat := newMockStrategy(map[string]interface{}{"auto_scale_enabled": true})
	w := newTestWorker(t, trader, strat, true)

	tests := []struct {
		name  string
		price float64
		qty   float64
	}{
		{"zero price", 0, 0.001},
		{"negative price", -100, 0.001},
		{"zero qty", 5000000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sig := buySignal(tc.price, tc.qty)
			if ok := w.applyAutoScaleToBuySignal(context.Background(), sig); ok {
				t.Error("expected ok=false for invalid signal")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: processSignal
// ---------------------------------------------------------------------------

func TestProcessSignal_HoldIsNoOp(t *testing.T) {
	trader := &mockTrader{}
	w := newTestWorker(t, trader, newMockStrategy(nil), true)

	w.processSignal(context.Background(), &strategy.Signal{Action: strategy.SignalHold})

	if len(trader.orders) != 0 {
		t.Error("HOLD signal should not place any order")
	}
}

func TestProcessSignal_TradingDisabledSkipsBuy(t *testing.T) {
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 100000}}}
	strat := newMockStrategy(map[string]interface{}{"auto_scale_enabled": false})
	w := newTestWorker(t, trader, strat, false) // trading disabled

	sig := buySignal(5000000, 0.001)
	w.processSignal(context.Background(), sig)

	if len(trader.orders) != 0 {
		t.Error("should not place order when trading is disabled")
	}
}

func TestProcessSignal_RiskCheckFailurePreventsOrder(t *testing.T) {
	trader := &mockTrader{balances: []domain.Balance{{Currency: "JPY", Available: 100000}}}
	strat := newMockStrategy(map[string]interface{}{"auto_scale_enabled": false})
	log := createTestLogger(t)
	signalCh := make(chan *strategy.Signal)
	w := &SignalWorker{
		logger:               log,
		signalCh:             signalCh,
		tradingEnabledGetter: &mockTradingEnabledGetter{enabled: true},
		riskChecker:          &mockRiskChecker{err: errors.New("daily loss limit exceeded")},
		trader:               trader,
		currentStrategy:      strat,
		performanceUpdater:   &mockPerformanceUpdater{},
		sellSizePercentage:   0.95,
	}

	sig := buySignal(5000000, 0.001)
	w.processSignal(context.Background(), sig)

	if len(trader.orders) != 0 {
		t.Error("should not place order when risk check fails")
	}
}

func TestProcessSignal_BuyOrderPlaced(t *testing.T) {
	trader := &mockTrader{
		balances: []domain.Balance{{Currency: "JPY", Available: 100000}},
	}
	strat := newMockStrategy(map[string]interface{}{"auto_scale_enabled": false})
	w := newTestWorker(t, trader, strat, true)

	sig := buySignal(5000000, 0.001)
	w.processSignal(context.Background(), sig)

	// Wait for background goroutine
	w.wg.Wait()

	if len(trader.orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(trader.orders))
	}
	if trader.orders[0].Side != "BUY" {
		t.Errorf("expected BUY order, got %s", trader.orders[0].Side)
	}
}

func TestProcessSignal_SellSkippedWhenNoHoldings(t *testing.T) {
	trader := &mockTrader{
		// No crypto holdings
		balances: []domain.Balance{{Currency: "JPY", Available: 100000}},
	}
	w := newTestWorker(t, trader, newMockStrategy(nil), true)

	sig := &strategy.Signal{
		Symbol:   "BTC_JPY",
		Action:   strategy.SignalSell,
		Price:    5000000,
		Quantity: 0.001,
	}
	w.processSignal(context.Background(), sig)
	w.wg.Wait()

	if len(trader.orders) != 0 {
		t.Error("should skip SELL when no crypto holdings")
	}
}

// ---------------------------------------------------------------------------
// Tests: getAutoScaleConfig (edge cases for balancePct clamping)
// ---------------------------------------------------------------------------

func TestGetAutoScaleConfig_InvalidBalancePctClamped(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want float64
	}{
		{"zero pct falls back to default", 0, 80.0},
		{"negative pct falls back to default", -5, 80.0},
		{"over 100 falls back to default", 101, 80.0},
		{"valid 5 kept", 5, 5.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strat := newMockStrategy(map[string]interface{}{
				"auto_scale_enabled":     true,
				"auto_scale_balance_pct": tc.pct,
			})
			trader := &mockTrader{}
			w := newTestWorker(t, trader, strat, true)
			cfg := w.getAutoScaleConfig()
			if cfg.balancePct != tc.want {
				t.Errorf("balancePct = %v; want %v", cfg.balancePct, tc.want)
			}
		})
	}
}
