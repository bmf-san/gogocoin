package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// buyStrategy always emits a BUY so that entry-suppression can be exercised.
type buyStrategy struct {
	mockStrategyWithConfig
}

func (b *buyStrategy) Analyze([]strategy.MarketData) (*strategy.Signal, error) {
	return &strategy.Signal{
		Symbol:   "XRP_JPY",
		Action:   strategy.SignalBuy,
		Strength: 1.0,
	}, nil
}

func testSignal(action strategy.SignalAction) *strategy.Signal {
	return &strategy.Signal{
		Symbol:    "XRP_JPY",
		Action:    action,
		Strength:  1.0,
		Price:     100.0,
		Timestamp: time.Now(),
	}
}

// A forced exit protects an open position, so a full channel must not cause it
// to be discarded: the send waits for the consumer to catch up.
func TestSendSignal_ForcedExitWaitsForRoomInsteadOfDropping(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	ch := make(chan *strategy.Signal, 1)
	w.signalCh = ch
	ch <- testSignal(strategy.SignalBuy) // fill the buffer

	// Drain shortly after, simulating a consumer that is briefly behind.
	go func() {
		time.Sleep(50 * time.Millisecond)
		<-ch
	}()

	if !w.sendSignal(context.Background(), testSignal(strategy.SignalSell), true) {
		t.Fatal("forced-exit signal was not enqueued; it must never be silently dropped")
	}
	if got := atomic.LoadInt64(&w.droppedSignals); got != 0 {
		t.Fatalf("expected no dropped signals, got %d", got)
	}
}

// Ordinary strategy signals are regenerated on the next tick, so dropping them
// under back-pressure is still the correct behavior.
func TestSendSignal_OrdinarySignalIsDroppedWhenChannelFull(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	ch := make(chan *strategy.Signal, 1)
	w.signalCh = ch
	ch <- testSignal(strategy.SignalBuy)

	if w.sendSignal(context.Background(), testSignal(strategy.SignalSell), false) {
		t.Fatal("ordinary signal should be dropped when the channel is full")
	}
	if got := atomic.LoadInt64(&w.droppedSignals); got != 1 {
		t.Fatalf("expected dropped_signals=1, got %d", got)
	}
}

// Waiting for room must not outlive shutdown.
func TestSendSignal_ForcedExitAbortsOnContextCancel(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	ch := make(chan *strategy.Signal, 1)
	w.signalCh = ch
	ch <- testSignal(strategy.SignalBuy)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan bool, 1)
	go func() { done <- w.sendSignal(ctx, testSignal(strategy.SignalSell), true) }()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected send to fail after context cancellation")
		}
	case <-time.After(ForcedExitSendTimeout):
		t.Fatal("sendSignal did not return after context cancellation")
	}
}

// A failed position read means stop-loss state is unknown; it must surface as an
// error rather than being reported as "no exit required".
func TestForcedExitSignal_PositionReadErrorIsReported(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0, "take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{err: errors.New("database is locked")})

	sig, err := w.forcedExitSignal("XRP_JPY", 100.0)
	if err == nil {
		t.Fatal("expected an error when open positions cannot be read")
	}
	if sig != nil {
		t.Fatalf("expected no signal on read failure, got %+v", sig)
	}
	if got := atomic.LoadInt64(&w.positionReadFailures); got != 0 {
		t.Fatalf("counter should only advance when the failure is reported, got %d", got)
	}

	w.reportPositionReadFailure("XRP_JPY", err)
	if got := atomic.LoadInt64(&w.positionReadFailures); got != 1 {
		t.Fatalf("expected position_read_failures=1, got %d", got)
	}
}

// While positions cannot be read the stop-loss cannot be enforced, so opening a
// new position would add risk that cannot be managed.
func TestExecuteStrategy_SuppressesBuyWhenPositionReadFails(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	ch := make(chan *strategy.Signal, 10)
	w.signalCh = ch
	w.strategy = &buyStrategy{mockStrategyWithConfig{cfg: map[string]any{"stop_loss_pct": 1.0}}}
	w.SetPositionReader(&mockPositionReader{err: errors.New("database is locked")})

	w.executeStrategy(context.Background(), &domain.MarketData{Symbol: "XRP_JPY", Price: 100.0}, nil)

	select {
	case sig := <-ch:
		t.Fatalf("BUY must be suppressed while position state is unreadable, got %+v", sig)
	default:
	}
	if got := atomic.LoadInt64(&w.positionReadFailures); got != 1 {
		t.Fatalf("expected position_read_failures=1, got %d", got)
	}
}

// The same failure must not block an exit: a SELL still goes through.
func TestExecuteStrategy_AllowsSellWhenPositionReadFails(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	ch := make(chan *strategy.Signal, 10)
	w.signalCh = ch
	w.SetPositionReader(&mockPositionReader{err: errors.New("database is locked")})

	signal := testSignal(strategy.SignalSell)
	w.logger.LogStrategySignal("test", signal.Symbol, string(signal.Action), signal.Strength, nil)
	if !w.sendSignal(context.Background(), signal, false) {
		t.Fatal("SELL should be enqueued even while position reads are failing")
	}
}
