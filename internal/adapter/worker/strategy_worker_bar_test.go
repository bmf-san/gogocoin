package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// recordingStrategy captures every Analyze() call for assertion in tests.
// It embeds the package-level mockStrategy to inherit the no-op
// implementations of the rest of the Strategy interface.
type recordingStrategy struct {
	mockStrategy
	mu    sync.Mutex
	calls [][]strategy.MarketData
}

func (r *recordingStrategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]strategy.MarketData, len(data))
	copy(cp, data)
	r.calls = append(r.calls, cp)
	return &strategy.Signal{Action: strategy.SignalHold}, nil
}

func (r *recordingStrategy) snapshot() [][]strategy.MarketData {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]strategy.MarketData, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestBarBucketStart verifies UTC alignment of bar buckets matches the
// backtest resampler's expectations (left-labeled, UTC-aligned).
func TestBarBucketStart(t *testing.T) {
	hour := time.Hour
	cases := []struct {
		in   string
		want string
	}{
		{"2026-05-26T03:14:59Z", "2026-05-26T03:00:00Z"},
		{"2026-05-26T03:00:00Z", "2026-05-26T03:00:00Z"},
		{"2026-05-26T03:59:59.999Z", "2026-05-26T03:00:00Z"},
	}
	for _, c := range cases {
		ts, _ := time.Parse(time.RFC3339Nano, c.in)
		got := barBucketStart(ts, hour).Format(time.RFC3339Nano)
		want, _ := time.Parse(time.RFC3339Nano, c.want)
		if got != want.Format(time.RFC3339Nano) {
			t.Errorf("barBucketStart(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestStrategyWorker_BarAggregation_InvokesPerBar feeds ticks spanning three
// 1-minute buckets and verifies Analyze is invoked exactly twice (once per
// completed bar — the still-open final bar is not flushed) and that history
// passed to Analyze contains aggregated bars, not raw ticks.
func TestStrategyWorker_BarAggregation_InvokesPerBar(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level: "error", Format: "json", Output: "file", FilePath: "/dev/null",
	})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	rec := &recordingStrategy{}
	dataCh := make(chan domain.MarketData, 16)
	sigCh := make(chan *strategy.Signal, 16)
	w := NewStrategyWorker(log, rec, dataCh, sigCh)
	w.SetBarPeriod(time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx)
		close(done)
	}()

	base := time.Date(2026, 5, 26, 3, 0, 0, 0, time.UTC)
	ticks := []domain.MarketData{
		// Bucket A (03:00): three ticks, close=101
		{Symbol: "XRP_JPY", Price: 100, Timestamp: base.Add(10 * time.Second)},
		{Symbol: "XRP_JPY", Price: 102, Timestamp: base.Add(30 * time.Second)},
		{Symbol: "XRP_JPY", Price: 101, Timestamp: base.Add(50 * time.Second)},
		// Bucket B (03:01): two ticks, close=104
		{Symbol: "XRP_JPY", Price: 103, Timestamp: base.Add(70 * time.Second)},
		{Symbol: "XRP_JPY", Price: 104, Timestamp: base.Add(90 * time.Second)},
		// Bucket C (03:02): one tick, close=105 — opens bar but does not finalize it
		{Symbol: "XRP_JPY", Price: 105, Timestamp: base.Add(130 * time.Second)},
	}
	for _, m := range ticks {
		dataCh <- m
	}

	// Wait for at most ~2 s for the worker to drain ticks and finalize two bars.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(rec.snapshot()) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	calls := rec.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 Analyze calls (one per completed bar), got %d", len(calls))
	}

	// First call: only Bucket A bar in history, Close=101.
	if got := len(calls[0]); got != 1 {
		t.Errorf("first call history len = %d, want 1", got)
	} else if calls[0][0].Price != 101 {
		t.Errorf("first bar Close = %v, want 101", calls[0][0].Price)
	}
	// Second call: Bucket A + Bucket B, Bucket B Close=104.
	if got := len(calls[1]); got != 2 {
		t.Errorf("second call history len = %d, want 2", got)
	} else {
		if calls[1][0].Price != 101 {
			t.Errorf("history[0].Price = %v, want 101", calls[1][0].Price)
		}
		if calls[1][1].Price != 104 {
			t.Errorf("history[1].Price = %v, want 104", calls[1][1].Price)
		}
	}
	// Timestamps should be bucket starts (left-labeled).
	wantStart := base
	if !calls[1][0].Timestamp.Equal(wantStart) {
		t.Errorf("bar A timestamp = %v, want %v", calls[1][0].Timestamp, wantStart)
	}
	if !calls[1][1].Timestamp.Equal(base.Add(time.Minute)) {
		t.Errorf("bar B timestamp = %v, want %v", calls[1][1].Timestamp, base.Add(time.Minute))
	}

	cancel()
	close(dataCh)
	<-done
}

// TestStrategyWorker_BarAggregation_Disabled verifies that with BarPeriod=0
// (default) the worker preserves legacy per-tick behavior: every tick
// triggers an Analyze call.
func TestStrategyWorker_BarAggregation_Disabled(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level: "error", Format: "json", Output: "file", FilePath: "/dev/null",
	})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	rec := &recordingStrategy{}
	dataCh := make(chan domain.MarketData, 8)
	sigCh := make(chan *strategy.Signal, 8)
	w := NewStrategyWorker(log, rec, dataCh, sigCh)
	// Note: SetBarPeriod NOT called → tick mode.

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx)
		close(done)
	}()

	base := time.Date(2026, 5, 26, 3, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		dataCh <- domain.MarketData{Symbol: "XRP_JPY", Price: 100 + float64(i), Timestamp: base.Add(time.Duration(i) * time.Second)}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(rec.snapshot()) >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := len(rec.snapshot()); got != 3 {
		t.Errorf("expected 3 per-tick Analyze calls, got %d", got)
	}

	cancel()
	close(dataCh)
	<-done
}
