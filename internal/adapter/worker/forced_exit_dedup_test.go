package worker

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// mockStrategyHold always returns a HOLD signal so the SL/TP override is
// exercised exclusively by the tests in this file.
type mockStrategyHold struct {
	mockStrategyWithConfig
}

func (m *mockStrategyHold) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	sym := ""
	if len(data) > 0 {
		sym = data[0].Symbol
	}
	return &strategy.Signal{Symbol: sym, Action: strategy.SignalHold}, nil
}

// newForcedExitWorker builds a StrategyWorker whose EMA always returns HOLD,
// so only SL/TP can inject a SELL into signalCh.
func newForcedExitWorker(t *testing.T, cfg map[string]any, positions []domain.Position) (*StrategyWorker, chan *strategy.Signal) {
	t.Helper()
	w := newTestStrategyWorker(t, cfg)
	// Replace the strategy with the HOLD-only variant
	w.strategy = &mockStrategyHold{mockStrategyWithConfig: mockStrategyWithConfig{cfg: cfg}}
	w.SetPositionReader(&mockPositionReader{positions: positions})
	// Replace signalCh with a buffered channel for assertions
	signalCh := make(chan *strategy.Signal, 20)
	w.signalCh = signalCh
	return w, signalCh
}

// callExecuteStrategy invokes executeStrategy with a single synthetic market data
// point at the given price.
func callExecuteStrategy(w *StrategyWorker, symbol string, price float64) {
	data := &domain.MarketData{Symbol: symbol, Price: price}
	history := []strategy.MarketData{{Symbol: symbol, Price: price}}
	w.executeStrategy(context.Background(), data, history)
}

// --- Stop-loss dedup bypass ---

// TestForcedExit_StopLossBypassesDedupAfterEmaSell confirms that a stop-loss SELL
// is dispatched to signalCh even when a previous EMA SELL already set
// lastSentSignals[symbol]=SELL (which would normally suppress the signal).
func TestForcedExit_StopLossBypassesDedupAfterEmaSell(t *testing.T) {
	const symbol = "XRP_JPY"
	positions := []domain.Position{
		{Symbol: symbol, Side: "BUY", EntryPrice: 230.0, Status: "PARTIAL"},
	}
	w, signalCh := newForcedExitWorker(t, map[string]any{"stop_loss_pct": 1.5}, positions)

	// Simulate a stale EMA SELL that set lastSentSignals without closing the position.
	w.lastSentSignals.Store(symbol, strategy.SignalSell)

	// Current price is below stop (230 * 0.985 = 226.55); stop-loss must override dedup.
	callExecuteStrategy(w, symbol, 224.0)

	select {
	case sig := <-signalCh:
		if sig.Action != strategy.SignalSell {
			t.Fatalf("expected SELL, got %v", sig.Action)
		}
		if sig.Metadata["reason"] != "stop_loss" {
			t.Fatalf("expected reason=stop_loss, got %v", sig.Metadata["reason"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stop-loss SELL should bypass stale EMA dedup but no signal received")
	}
}

// TestForcedExit_TakeProfitBypassesDedupAfterEmaSell confirms that a take-profit
// SELL is dispatched after a stale EMA SELL set lastSentSignals.
func TestForcedExit_TakeProfitBypassesDedupAfterEmaSell(t *testing.T) {
	const symbol = "ETH_JPY"
	positions := []domain.Position{
		{Symbol: symbol, Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"},
	}
	w, signalCh := newForcedExitWorker(t, map[string]any{"take_profit_pct": 3.0}, positions)

	w.lastSentSignals.Store(symbol, strategy.SignalSell)

	// 3% above 330000 = 339900; current 340000 triggers TP.
	callExecuteStrategy(w, symbol, 340000.0)

	select {
	case sig := <-signalCh:
		if sig.Action != strategy.SignalSell {
			t.Fatalf("expected SELL, got %v", sig.Action)
		}
		if sig.Metadata["reason"] != "take_profit" {
			t.Fatalf("expected reason=take_profit, got %v", sig.Metadata["reason"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("take-profit SELL should bypass stale EMA dedup but no signal received")
	}
}

// --- Cooldown enforcement ---

// TestForcedExit_CooldownBlocksRepeat confirms that a second forced exit for the
// same symbol within the cooldown window is suppressed (no duplicate order).
func TestForcedExit_CooldownBlocksRepeat(t *testing.T) {
	const symbol = "XRP_JPY"
	positions := []domain.Position{
		{Symbol: symbol, Side: "BUY", EntryPrice: 230.0, Status: "PARTIAL"},
	}
	w, signalCh := newForcedExitWorker(t, map[string]any{"stop_loss_pct": 1.5}, positions)

	// First call — should enqueue the SELL.
	callExecuteStrategy(w, symbol, 224.0)

	select {
	case sig := <-signalCh:
		if sig.Action != strategy.SignalSell {
			t.Fatalf("first call: expected SELL, got %v", sig.Action)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first stop-loss SELL should have been enqueued")
	}

	// Second call immediately — must be suppressed by cooldown.
	callExecuteStrategy(w, symbol, 224.0)

	select {
	case sig := <-signalCh:
		t.Fatalf("second call within cooldown should be suppressed, got %v", sig)
	case <-time.After(50 * time.Millisecond):
		// expected: no signal
	}
}

// TestForcedExit_CooldownExpiredAllowsRetry confirms that a forced exit IS
// re-sent after the cooldown window has elapsed.
func TestForcedExit_CooldownExpiredAllowsRetry(t *testing.T) {
	const symbol = "XRP_JPY"
	positions := []domain.Position{
		{Symbol: symbol, Side: "BUY", EntryPrice: 230.0, Status: "PARTIAL"},
	}
	w, signalCh := newForcedExitWorker(t, map[string]any{"stop_loss_pct": 1.5}, positions)

	// Simulate a forced exit that happened far in the past (beyond cooldown).
	pastTime := time.Now().Add(-(ForcedExitCooldown + time.Second))
	w.lastForcedExitAttempt.Store(symbol, pastTime)
	// Also set lastSentSignals=SELL to confirm the dedup is bypassed.
	w.lastSentSignals.Store(symbol, strategy.SignalSell)

	callExecuteStrategy(w, symbol, 224.0)

	select {
	case sig := <-signalCh:
		if sig.Action != strategy.SignalSell {
			t.Fatalf("expected SELL after cooldown expired, got %v", sig.Action)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stop-loss SELL should fire after cooldown expired but no signal received")
	}
}
