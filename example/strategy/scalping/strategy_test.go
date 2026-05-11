package scalping

import (
	"context"
	"testing"
	"time"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

func newTestStrategy() *Strategy {
	return New(Params{
		EMAFastPeriod:  3,
		EMASlowPeriod:  5,
		TakeProfitPct:  1.0,
		StopLossPct:    0.5,
		CooldownSec:    300,
		MaxDailyTrades: 1000,
		OrderNotional:  100.0,
	})
}

func makeHistory(prices ...float64) []strategy.MarketData {
	out := make([]strategy.MarketData, len(prices))
	for i, p := range prices {
		out[i] = strategy.MarketData{Symbol: "BTC_JPY", Price: p}
	}
	return out
}

// TestCooldown_UsesSimulatedTime guarantees that cooldown elapses against the
// observed MarketData.Timestamp rather than wall-clock time. Without this,
// backtest replay (which processes long periods of ticks in seconds) would
// trigger an indefinite "cooldown" HOLD after the very first BUY, producing
// the well-known trade=1 plateau across all parameter combinations.
func TestCooldown_UsesSimulatedTime(t *testing.T) {
	s := newTestStrategy()
	hist := makeHistory(100, 100, 100, 100, 100, 110, 120, 130, 140, 150)
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// T=0: first signal call sets lastSeenTimestamp = base.
	tick0 := &strategy.MarketData{Symbol: "BTC_JPY", Price: hist[len(hist)-1].Price, Timestamp: base}
	if _, err := s.GenerateSignal(context.Background(), tick0, hist); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.RecordTrade()

	// T+1s: still in cooldown (300s).
	tick1 := &strategy.MarketData{Symbol: "BTC_JPY", Price: hist[len(hist)-1].Price, Timestamp: base.Add(1 * time.Second)}
	if _, err := s.GenerateSignal(context.Background(), tick1, hist); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.isInCooldown(300) {
		t.Errorf("at +1s simulated: expected isInCooldown=true (cooldown=300s)")
	}

	// T+301s: cooldown elapsed in simulated time. Wall-clock-based code would
	// still report cooldown here because only microseconds have actually passed.
	tick2 := &strategy.MarketData{Symbol: "BTC_JPY", Price: hist[len(hist)-1].Price, Timestamp: base.Add(301 * time.Second)}
	if _, err := s.GenerateSignal(context.Background(), tick2, hist); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.isInCooldown(300) {
		t.Errorf("at +301s simulated: expected isInCooldown=false (cooldown elapsed)")
	}
}
