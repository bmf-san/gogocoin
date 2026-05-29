package backtest

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	pkgstrategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// fakeStrategy is a deterministic strategy used by engine tests.
// It emits the next pre-programmed signal on each GenerateSignal call.
type fakeStrategy struct {
	*pkgstrategy.BaseStrategy
	signals  []pkgstrategy.SignalAction
	idx      int
	tpPct    float64
	slPct    float64
	notional float64
	recorded int
}

func newFakeStrategy(actions []pkgstrategy.SignalAction, tpPct, slPct, notional float64) *fakeStrategy {
	return &fakeStrategy{
		BaseStrategy: pkgstrategy.NewBaseStrategy("fake", "test fake", "0"),
		signals:      actions,
		tpPct:        tpPct,
		slPct:        slPct,
		notional:     notional,
	}
}

func (f *fakeStrategy) GenerateSignal(_ context.Context, d *pkgstrategy.MarketData, _ []pkgstrategy.MarketData) (*pkgstrategy.Signal, error) {
	if f.idx >= len(f.signals) {
		return f.CreateSignal(d.Symbol, pkgstrategy.SignalHold, 0, d.Price, 0, nil), nil
	}
	a := f.signals[f.idx]
	f.idx++
	return f.CreateSignal(d.Symbol, a, 1, d.Price, 0, map[string]interface{}{"reason": "fake"}), nil
}

func (f *fakeStrategy) Analyze(_ []pkgstrategy.MarketData) (*pkgstrategy.Signal, error) {
	return nil, nil
}

func (f *fakeStrategy) Initialize(_ map[string]interface{}) error   { return nil }
func (f *fakeStrategy) UpdateConfig(_ map[string]interface{}) error { return nil }
func (f *fakeStrategy) GetTakeProfitPrice(entry float64) float64 {
	if f.tpPct == 0 {
		return 0
	}
	return entry * (1 + f.tpPct/100)
}
func (f *fakeStrategy) GetStopLossPrice(entry float64) float64 {
	if f.slPct == 0 {
		return 0
	}
	return entry * (1 - f.slPct/100)
}
func (f *fakeStrategy) GetBaseNotional(_ string) float64 { return f.notional }
func (f *fakeStrategy) GetAutoScaleConfig() pkgstrategy.AutoScaleConfig {
	return pkgstrategy.AutoScaleConfig{}
}
func (f *fakeStrategy) RecordTrade() { f.recorded++ }

// sliceDS is a Datasource backed by an in-memory slice.
type sliceDS struct {
	bars []Bar
	i    int
}

func (s *sliceDS) Next(ctx context.Context) (Bar, error) {
	if err := ctx.Err(); err != nil {
		return Bar{}, err
	}
	if s.i >= len(s.bars) {
		return Bar{}, io.EOF
	}
	b := s.bars[s.i]
	s.i++
	return b, nil
}
func (s *sliceDS) Close() error { return nil }

func mkBars(prices [][4]float64, start time.Time) []Bar {
	bars := make([]Bar, len(prices))
	for i, p := range prices {
		bars[i] = Bar{
			Symbol:    "XRP_JPY",
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			Open:      p[0],
			High:      p[1],
			Low:       p[2],
			Close:     p[3],
			Volume:    1_000_000,
		}
	}
	return bars
}

func TestSimulator_TPHit(t *testing.T) {
	// Bars: bar0 buy signal, bar1 entry @ open=100, bar2 high=101 hits TP.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100}, // bar0 — signal: BUY
		{100, 100, 100, 100}, // bar1 — fill entry at open=100 (post-slip 100*1.0005=100.05)
		{100, 102, 100, 101}, // bar2 — TP hit (entry*1.005=100.55, high=102 ≥ TP)
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 0.5, 1.0, 1000)
	cfg := EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator: SimulatorConfig{
			InitialBalance: 100_000,
			FeeRate:        0.0015,
			SlippageBps:    5,
			SameBarRule:    SameBarSLPriority,
		},
	}
	res, err := NewEngine(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(res.Trades))
	}
	tr := res.Trades[0]
	if tr.Reason != ExitReasonTP {
		t.Errorf("expected TP exit, got %s", tr.Reason)
	}
	if tr.NetPnL <= 0 {
		t.Errorf("TP exit should be a winner, got NetPnL=%.4f", tr.NetPnL)
	}
}

func TestSimulator_SLHit(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100},
		{100, 100, 100, 100},
		{100, 100, 98, 99}, // SL=99.05 hit by low=98 → SL exit
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 5.0, 1.0, 1000)
	cfg := EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator: SimulatorConfig{
			InitialBalance: 100_000,
			FeeRate:        0.0015,
			SlippageBps:    5,
		},
	}
	res, err := NewEngine(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(res.Trades))
	}
	if res.Trades[0].Reason != ExitReasonSL {
		t.Errorf("expected SL exit, got %s", res.Trades[0].Reason)
	}
	if res.Trades[0].NetPnL >= 0 {
		t.Errorf("SL exit should be a loser, got %.4f", res.Trades[0].NetPnL)
	}
}

func TestSimulator_SameBarSLPriority(t *testing.T) {
	// Bar where both TP and SL are reachable; SL must win under default rule.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100},
		{100, 100, 100, 100},
		{100, 102, 98, 100}, // both TP=100.55 and SL=99.05 reachable
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 0.5, 1.0, 1000)
	cfg := EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator: SimulatorConfig{
			InitialBalance: 100_000,
			FeeRate:        0.0015,
			SlippageBps:    5,
			SameBarRule:    SameBarSLPriority,
		},
	}
	res, err := NewEngine(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(res.Trades))
	}
	if res.Trades[0].Reason != ExitReasonSL {
		t.Errorf("expected SL under sl_priority, got %s", res.Trades[0].Reason)
	}
}

func TestSimulator_SignalExit(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100},
		{100, 100, 100, 100}, // entry @ 100*(1+slip)
		{101, 101, 101, 101},
		{101, 101, 101, 101}, // SELL fill @ open=101*(1-slip)
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{
		pkgstrategy.SignalBuy,
		pkgstrategy.SignalHold,
		pkgstrategy.SignalSell,
	}, 0, 0, 1000)
	cfg := EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator: SimulatorConfig{
			InitialBalance: 100_000,
			FeeRate:        0.0,
			SlippageBps:    0,
		},
	}
	res, err := NewEngine(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(res.Trades))
	}
	if res.Trades[0].Reason != ExitReasonSignal {
		t.Errorf("expected signal exit, got %s", res.Trades[0].Reason)
	}
	if res.Trades[0].NetPnL <= 0 {
		t.Errorf("expected profit on price up, got %.4f", res.Trades[0].NetPnL)
	}
}

func TestSimulator_EODForceClose(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100},
		{100, 100, 100, 100},
		{100, 100, 100, 100}, // last bar — still holding
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 0, 0, 1000)
	cfg := EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator: SimulatorConfig{
			InitialBalance: 100_000,
			FeeRate:        0.0,
			SlippageBps:    0,
		},
	}
	res, err := NewEngine(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 EOD trade, got %d", len(res.Trades))
	}
	if res.Trades[0].Reason != ExitReasonEOD {
		t.Errorf("expected EOD exit, got %s", res.Trades[0].Reason)
	}
}

func TestSimulator_FeeAndSlippageReduceProfit(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := mkBars([][4]float64{
		{100, 100, 100, 100},
		{100, 100, 100, 100}, // buy fill
		{102, 102, 102, 102}, // tp likely hit since high>=tp
	}, start)
	strat := newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 0.5, 5, 1000)

	// no fee / slip
	rNo, err := NewEngine(EngineConfig{
		Strategy:   newFakeStrategy([]pkgstrategy.SignalAction{pkgstrategy.SignalBuy}, 0.5, 5, 1000),
		Datasource: &sliceDS{bars: bars},
		Simulator:  SimulatorConfig{InitialBalance: 100_000},
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rWith, err := NewEngine(EngineConfig{
		Strategy:   strat,
		Datasource: &sliceDS{bars: bars},
		Simulator:  SimulatorConfig{InitialBalance: 100_000, FeeRate: 0.0015, SlippageBps: 10},
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rNo.Trades) != 1 || len(rWith.Trades) != 1 {
		t.Fatalf("expected one trade each, got no=%d with=%d", len(rNo.Trades), len(rWith.Trades))
	}
	if rWith.Trades[0].NetPnL >= rNo.Trades[0].NetPnL {
		t.Errorf("fees/slippage should reduce P&L: no=%.4f with=%.4f",
			rNo.Trades[0].NetPnL, rWith.Trades[0].NetPnL)
	}
}

func TestEngine_NoStrategy(t *testing.T) {
	_, err := NewEngine(EngineConfig{}).Run(context.Background())
	if err == nil {
		t.Fatal("expected error for missing strategy")
	}
}

func TestEngine_NoDatasource(t *testing.T) {
	_, err := NewEngine(EngineConfig{Strategy: newFakeStrategy(nil, 0, 0, 0)}).Run(context.Background())
	if err == nil {
		t.Fatal("expected error for missing datasource")
	}
}

func TestEngine_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bars := mkBars([][4]float64{{100, 100, 100, 100}, {100, 100, 100, 100}}, time.Now())
	_, err := NewEngine(EngineConfig{
		Strategy:   newFakeStrategy(nil, 0, 0, 1000),
		Datasource: &sliceDS{bars: bars},
		Simulator:  SimulatorConfig{InitialBalance: 100_000},
	}).Run(ctx)
	if err == nil {
		t.Fatal("expected context cancel error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Logf("non-canonical error: %v", err)
	}
}
