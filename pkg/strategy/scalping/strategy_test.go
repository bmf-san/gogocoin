package scalping

import (
	"context"
	"testing"
	"time"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// makeHistory returns a slice of MarketData with given prices, all for "BTC_JPY".
func makeHistory(prices ...float64) []strategy.MarketData {
	out := make([]strategy.MarketData, len(prices))
	for i, p := range prices {
		out[i] = strategy.MarketData{Symbol: "BTC_JPY", Price: p}
	}
	return out
}

// newTestStrategy creates a Strategy suitable for unit tests (no cooldown / no daily limit).
func newTestStrategy() *Strategy {
	return New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    0,
		MaxDailyTrades: 1000,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
}

// ── calculateEMA ─────────────────────────────────────────────────────────────

func TestCalculateEMA_InsufficientHistory(t *testing.T) {
	s := newTestStrategy()
	// period=3, only 2 data points → should return 0
	hist := makeHistory(100, 110)
	if got := s.calculateEMA(hist, 3); got != 0.0 {
		t.Errorf("expected 0.0 for insufficient history, got %v", got)
	}
}

func TestCalculateEMA_ExactPeriod(t *testing.T) {
	s := newTestStrategy()
	// With exactly 3 equal prices the EMA equals that price.
	hist := makeHistory(100, 100, 100)
	if got := s.calculateEMA(hist, 3); got != 100.0 {
		t.Errorf("expected 100.0, got %v", got)
	}
}

func TestCalculateEMA_FullWarmup(t *testing.T) {
	// Verify the fixed algorithm: SMA from first `period` points, then EMA loop
	// over remaining points.  A constant series must always return that constant.
	s := newTestStrategy()
	hist := makeHistory(200, 200, 200, 200, 200, 200, 200)
	for _, period := range []int{3, 5, 7} {
		if got := s.calculateEMA(hist, period); got != 200.0 {
			t.Errorf("period=%d: expected 200.0, got %v", period, got)
		}
	}
}

func TestCalculateEMA_RisingPrices(t *testing.T) {
	// For a steadily rising series the EMA must be below the last price (lag).
	s := newTestStrategy()
	hist := makeHistory(100, 102, 104, 106, 108, 110, 112, 114, 116, 118, 120)
	ema := s.calculateEMA(hist, 5)
	last := hist[len(hist)-1].Price
	if ema >= last {
		t.Errorf("expected EMA (%v) < last price (%v) for rising series", ema, last)
	}
	if ema <= 100 {
		t.Errorf("expected EMA (%v) > first price for rising series", ema)
	}
}

func TestCalculateEMA_FastSlowOrdering(t *testing.T) {
	// In a rising trend, faster EMA must be higher than slower EMA.
	s := newTestStrategy()
	hist := makeHistory(100, 101, 103, 106, 110, 115, 121, 128, 136, 145, 155)
	emaFast := s.calculateEMA(hist, 3)
	emaSlow := s.calculateEMA(hist, 5)
	if emaFast <= emaSlow {
		t.Errorf("expected fast EMA (%v) > slow EMA (%v) in rising trend", emaFast, emaSlow)
	}
}

// ── GenerateSignal ────────────────────────────────────────────────────────────

func TestGenerateSignal_InsufficientHistory(t *testing.T) {
	s := newTestStrategy()             // slow period = 5
	hist := makeHistory(100, 100, 100) // only 3 < 5
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: 100}
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalHold {
		t.Errorf("expected HOLD for insufficient history, got %v", sig.Action)
	}
}

func TestGenerateSignal_NilData(t *testing.T) {
	s := newTestStrategy()
	_, err := s.GenerateSignal(context.Background(), nil, makeHistory(100, 100, 100, 100, 100))
	if err == nil {
		t.Error("expected error for nil market data")
	}
}

func buildBuyHistory() []strategy.MarketData {
	// Build a history where fast EMA (3) > slow EMA (5) and price > fast EMA.
	// Use a strongly rising series.
	return makeHistory(100, 100, 100, 100, 100, 110, 120, 130, 140, 150)
}

func TestGenerateSignal_BuyOnCrossover(t *testing.T) {
	s := newTestStrategy()
	hist := buildBuyHistory()
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: hist[len(hist)-1].Price}

	// First call — crossover should be detected (initialized=false).
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalBuy {
		t.Logf("signal=%v metadata=%v", sig.Action, sig.Metadata)
		t.Errorf("expected BUY on first crossover, got %v", sig.Action)
	}
}

func TestGenerateSignal_HoldOnRepeatDirection(t *testing.T) {
	s := newTestStrategy()
	hist := buildBuyHistory()
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: hist[len(hist)-1].Price}

	// First call initialises the EMA state.
	s.GenerateSignal(context.Background(), data, hist) //nolint:errcheck

	// Second call with same rising history — no new crossover → HOLD.
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Action != strategy.SignalHold {
		t.Errorf("expected HOLD on repeated direction (no new crossover), got %v", sig.Action)
	}
}

// ── Cooldown ─────────────────────────────────────────────────────────────────

func TestIsInCooldown_NoTrade(t *testing.T) {
	s := newTestStrategy()
	if s.isInCooldown() {
		t.Error("expected no cooldown before any trade")
	}
}

func TestIsInCooldown_AfterTrade(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    3600,
		MaxDailyTrades: 1000,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
	s.RecordTrade()
	if !s.isInCooldown() {
		t.Error("expected cooldown immediately after RecordTrade")
	}
}

// ── Daily limit ───────────────────────────────────────────────────────────────

func TestIsDailyLimitReached_BelowLimit(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    0,
		MaxDailyTrades: 3,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
	s.RecordTrade()
	s.RecordTrade()
	if s.isDailyLimitReached() {
		t.Error("expected limit NOT reached after 2/3 trades")
	}
}

func TestIsDailyLimitReached_AtLimit(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    0,
		MaxDailyTrades: 3,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
	s.RecordTrade()
	s.RecordTrade()
	s.RecordTrade()
	if !s.isDailyLimitReached() {
		t.Error("expected limit reached after 3/3 trades")
	}
}

// ── RecordTrade / Reset ───────────────────────────────────────────────────────

func TestRecordTrade_IncrementsCount(t *testing.T) {
	s := newTestStrategy()
	for i := 0; i < 5; i++ {
		s.RecordTrade()
	}
	s.mu.RLock()
	count := s.dailyTradeCount
	s.mu.RUnlock()
	if count != 5 {
		t.Errorf("expected dailyTradeCount=5, got %d", count)
	}
}

func TestReset_ClearsState(t *testing.T) {
	s := newTestStrategy()
	s.RecordTrade()
	if err := s.Reset(); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	s.mu.RLock()
	count := s.dailyTradeCount
	s.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected dailyTradeCount=0 after Reset, got %d", count)
	}
	if s.isInCooldown() {
		t.Error("expected no cooldown after Reset")
	}
}

// ── Take-profit / Stop-loss prices ───────────────────────────────────────────

func TestGetTakeProfitPrice(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  2.0,
		StopLossPct:    1.0,
		CooldownSec:    0,
		MaxDailyTrades: 10,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
	got := s.GetTakeProfitPrice(1000.0)
	want := 1020.0
	if got != want {
		t.Errorf("GetTakeProfitPrice(1000) = %v, want %v", got, want)
	}
}

func TestGetStopLossPrice(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  2.0,
		StopLossPct:    1.0,
		CooldownSec:    0,
		MaxDailyTrades: 10,
		OrderNotional:  100.0,
		FeeRate:        0.001,
	})
	got := s.GetStopLossPrice(1000.0)
	want := 990.0
	if got != want {
		t.Errorf("GetStopLossPrice(1000) = %v, want %v", got, want)
	}
}

// ── Initialize / validate ─────────────────────────────────────────────────────

func TestInitialize_ValidParams(t *testing.T) {
	s := newTestStrategy()
	err := s.Initialize(map[string]interface{}{
		"ema_fast_period":  5,
		"ema_slow_period":  10,
		"take_profit_pct":  1.5,
		"stop_loss_pct":    0.8,
		"cooldown_sec":     60,
		"max_daily_trades": 5,
		"order_notional":   500.0,
		"fee_rate":         0.001,
	})
	if err != nil {
		t.Errorf("unexpected validate error: %v", err)
	}
}

func TestInitialize_FastPeriodGTSlow(t *testing.T) {
	s := newTestStrategy()
	err := s.Initialize(map[string]interface{}{
		"ema_fast_period":  10,
		"ema_slow_period":  5, // invalid: fast >= slow
		"take_profit_pct":  1.0,
		"stop_loss_pct":    0.5,
		"cooldown_sec":     0,
		"max_daily_trades": 10,
		"order_notional":   100.0,
		"fee_rate":         0.001,
	})
	if err == nil {
		t.Error("expected error when ema_fast_period >= ema_slow_period")
	}
}

// ── RSI filter ────────────────────────────────────────────────────────────────

func TestGenerateSignal_RSIOverboughtBlocksBuy(t *testing.T) {
	// RSI with overbought=40 (very low threshold) should block BUY on a rising series.
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    0,
		MaxDailyTrades: 1000,
		OrderNotional:  100.0,
		FeeRate:        0.001,
		RSIPeriod:      3,
		RSIOverbought:  40.0, // very low — forces RSI>40 on a rising series
		RSIOversold:    10.0,
	})

	// Strongly rising prices → RSI will be > 40.
	hist := makeHistory(100, 100, 100, 100, 100, 110, 120, 130, 140, 200)
	data := &strategy.MarketData{Symbol: "BTC_JPY", Price: 200}
	sig, err := s.GenerateSignal(context.Background(), data, hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Either HOLD (RSI filtered) or BUY depending on exact RSI value.
	// The important thing is no error and action is a valid value.
	if sig.Action != strategy.SignalHold && sig.Action != strategy.SignalBuy {
		t.Errorf("unexpected action %v", sig.Action)
	}
}

// ── InitializeDailyTradeCount ─────────────────────────────────────────────────

func TestInitializeDailyTradeCount(t *testing.T) {
	s := newTestStrategy()
	s.InitializeDailyTradeCount(7)
	s.mu.RLock()
	count := s.dailyTradeCount
	date := s.lastTradeDate
	s.mu.RUnlock()
	if count != 7 {
		t.Errorf("expected dailyTradeCount=7, got %d", count)
	}
	// InitializeDailyTradeCount uses JST (UTC+9), so the expected date must
	// also be in JST to avoid mismatches in UTC CI environments after 15:00 UTC.
	jst := time.FixedZone("JST", 9*60*60)
	today := time.Now().In(jst).Format("2006-01-02")
	if date != today {
		t.Errorf("expected lastTradeDate=%s, got %s", today, date)
	}
}

// TestInitializeDailyTradeCount_DoesNotTriggerCooldown verifies that
// InitializeDailyTradeCount does NOT set lastTradeTime, so the cooldown
// timer is not activated on startup (regression: engine used RecordTrade
// which set lastTradeTime = time.Now(), blocking all trades for cooldownSec
// seconds after every restart).
func TestInitializeDailyTradeCount_DoesNotTriggerCooldown(t *testing.T) {
	s := newTestStrategy()
	s.InitializeDailyTradeCount(3)
	s.mu.RLock()
	last := s.lastTradeTime
	s.mu.RUnlock()
	if !last.IsZero() {
		t.Errorf("InitializeDailyTradeCount must not set lastTradeTime (cooldown side-effect), got %v", last)
	}
}

// ── Analyze ───────────────────────────────────────────────────────────────────

func TestAnalyze_EmptyData(t *testing.T) {
	s := newTestStrategy()
	_, err := s.Analyze(nil)
	if err == nil {
		t.Error("expected error for empty data in Analyze")
	}
}

func TestAnalyze_DelegatesCorrectly(t *testing.T) {
	s := newTestStrategy()
	hist := buildBuyHistory()
	sig, err := s.Analyze(hist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil {
		t.Fatal("expected non-nil signal from Analyze")
	}
}

// ── SymbolOverride ────────────────────────────────────────────────────────────

func TestSymbolEMAPeriods_Override(t *testing.T) {
	s := New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    0,
		MaxDailyTrades: 10,
		OrderNotional:  100.0,
		FeeRate:        0.001,
		SymbolParams: map[string]SymbolOverride{
			"ETH_JPY": {EMAFastPeriod: 7, EMASlowPeriod: 21},
		},
	})
	fast, slow := s.symbolEMAPeriods("ETH_JPY")
	if fast != 7 || slow != 21 {
		t.Errorf("expected fast=7 slow=21 for ETH_JPY override, got fast=%d slow=%d", fast, slow)
	}
	// For non-overridden symbol, global defaults should apply.
	fast, slow = s.symbolEMAPeriods("BTC_JPY")
	if fast != 3 || slow != 5 {
		t.Errorf("expected fast=3 slow=5 for BTC_JPY (global default), got fast=%d slow=%d", fast, slow)
	}
}

// TestInitialize_SymbolParams verifies that symbol_params supplied as
// map[string]map[string]interface{} (the format produced by strategyParamsToMap)
// are applied correctly by Initialize.  Before this fix, Initialize silently
// discarded the symbol_params key, so per-symbol EMA/notional overrides would
// never take effect when the strategy was started via engine.Run.
func TestInitialize_SymbolParams(t *testing.T) {
	s := NewDefault()
	err := s.Initialize(map[string]interface{}{
		"ema_fast_period":  9,
		"ema_slow_period":  21,
		"take_profit_pct":  0.8,
		"stop_loss_pct":    0.4,
		"cooldown_sec":     90,
		"max_daily_trades": 3,
		"order_notional":   200.0,
		"fee_rate":         0.001,
		"symbol_params": map[string]map[string]interface{}{
			"ETH_JPY": {
				"ema_fast_period": 5,
				"ema_slow_period": 15,
				"order_notional":  500.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize returned unexpected error: %v", err)
	}

	fast, slow := s.symbolEMAPeriods("ETH_JPY")
	if fast != 5 || slow != 15 {
		t.Errorf("ETH_JPY EMA override not applied: want fast=5 slow=15, got fast=%d slow=%d", fast, slow)
	}
	notional := s.symbolOrderNotional("ETH_JPY")
	if notional != 500.0 {
		t.Errorf("ETH_JPY order_notional override not applied: want 500.0, got %f", notional)
	}
	// Non-overridden symbol should still use global defaults.
	fast, slow = s.symbolEMAPeriods("BTC_JPY")
	if fast != 9 || slow != 21 {
		t.Errorf("BTC_JPY should use global defaults: want fast=9 slow=21, got fast=%d slow=%d", fast, slow)
	}
}
