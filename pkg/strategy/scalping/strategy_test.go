package scalping

import (
	"context"
	"testing"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// makeHistory returns a slice of MarketData with the given prices, all for "BTC_JPY".
func makeHistory(prices ...float64) []strategy.MarketData {
	out := make([]strategy.MarketData, len(prices))
	for i, p := range prices {
		out[i] = strategy.MarketData{Symbol: "BTC_JPY", Price: p}
	}
	return out
}

func newTestStrategy() *Strategy {
	return New(Params{
		EMAFastPeriod: 3,
		EMASlowPeriod: 5,
		TakeProfitPct: 1.0,
		StopLossPct:   0.5,
		OrderNotional: 100.0,
	})
}

// ── calcEMA ───────────────────────────────────────────────────────────────────

func TestCalcEMA_InsufficientHistory(t *testing.T) {
	if got := calcEMA(makeHistory(100, 110), 3); got != 0.0 {
		t.Errorf("expected 0.0 for insufficient history, got %v", got)
	}
}

func TestCalcEMA_ConstantSeries(t *testing.T) {
	hist := makeHistory(200, 200, 200, 200, 200, 200, 200)
	for _, period := range []int{3, 5, 7} {
		if got := calcEMA(hist, period); got != 200.0 {
			t.Errorf("period=%d: expected 200.0, got %v", period, got)
		}
	}
}

func TestCalcEMA_FastAboveSlowInUptrend(t *testing.T) {
	hist := makeHistory(100, 101, 103, 106, 110, 115, 121, 128, 136, 145, 155)
	fast, slow := calcEMA(hist, 3), calcEMA(hist, 5)
	if fast <= slow {
		t.Errorf("expected fast EMA (%v) > slow EMA (%v) in rising trend", fast, slow)
	}
}

// ── GenerateSignal ────────────────────────────────────────────────────────────

func TestGenerateSignal_NilData(t *testing.T) {
	s := newTestStrategy()
	_, err := s.GenerateSignal(context.Background(), nil, makeHistory(100, 100, 100, 100, 100))
	if err == nil {
		t.Error("expected error for nil market data")
	}
}

func TestGenerateSignal_InsufficientHistory(t *testing.T) {
	s := newTestStrategy()
	hist := makeHistory(100, 100, 100) // 3 < slow=5
	sig, err := s.GenerateSignal(context.Background(), &strategy.MarketData{Symbol: "BTC_JPY", Price: 100}, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalHold {
		t.Errorf("expected HOLD for insufficient history, got %v", sig.Action)
	}
}

func TestGenerateSignal_BuyWhenFastAboveSlow(t *testing.T) {
	s := newTestStrategy()
	hist := makeHistory(100, 100, 100, 100, 100, 110, 120, 130, 140, 150)
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: 150}
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalBuy {
		t.Errorf("expected BUY when fast EMA > slow EMA, got %v (metadata: %v)", sig.Action, sig.Metadata)
	}
}

func TestGenerateSignal_SellWhenFastBelowSlow(t *testing.T) {
	s := newTestStrategy()
	hist := makeHistory(150, 150, 150, 150, 150, 140, 130, 120, 110, 100)
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: 100}
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalSell {
		t.Errorf("expected SELL when fast EMA < slow EMA, got %v (metadata: %v)", sig.Action, sig.Metadata)
	}
}

// ── SL / TP prices ────────────────────────────────────────────────────────────

func TestGetTakeProfitPrice(t *testing.T) {
	s := New(Params{EMAFastPeriod: 3, EMASlowPeriod: 5, TakeProfitPct: 2.0, StopLossPct: 1.0, OrderNotional: 100.0})
	if got, want := s.GetTakeProfitPrice(1000.0), 1020.0; got != want {
		t.Errorf("GetTakeProfitPrice(1000) = %v, want %v", got, want)
	}
}

func TestGetStopLossPrice(t *testing.T) {
	s := New(Params{EMAFastPeriod: 3, EMASlowPeriod: 5, TakeProfitPct: 2.0, StopLossPct: 1.0, OrderNotional: 100.0})
	if got, want := s.GetStopLossPrice(1000.0), 990.0; got != want {
		t.Errorf("GetStopLossPrice(1000) = %v, want %v", got, want)
	}
}

// ── Initialize / validate ─────────────────────────────────────────────────────

func TestInitialize_ValidParams(t *testing.T) {
	s := newTestStrategy()
	err := s.Initialize(map[string]interface{}{
		"ema_fast_period": 5,
		"ema_slow_period": 10,
		"take_profit_pct": 1.5,
		"stop_loss_pct":   0.8,
		"order_notional":  500.0,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitialize_FastPeriodGTSlow(t *testing.T) {
	s := newTestStrategy()
	err := s.Initialize(map[string]interface{}{
		"ema_fast_period": 10,
		"ema_slow_period": 5,
		"take_profit_pct": 1.0,
		"stop_loss_pct":   0.5,
		"order_notional":  100.0,
	})
	if err == nil {
		t.Error("expected error when ema_fast_period >= ema_slow_period")
	}
}

// ── Analyze ───────────────────────────────────────────────────────────────────

func TestAnalyze_EmptyData(t *testing.T) {
	s := newTestStrategy()
	if _, err := s.Analyze(nil); err == nil {
		t.Error("expected error for empty data in Analyze")
	}
}

func TestAnalyze_DelegatesCorrectly(t *testing.T) {
	s := newTestStrategy()
	hist := makeHistory(100, 100, 100, 100, 100, 110, 120, 130, 140, 150)
	sig, err := s.Analyze(hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil {
		t.Fatal("expected non-nil signal from Analyze")
	}
}

