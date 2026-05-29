package backtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	pkgstrategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// HistoryLimit is the maximum number of MarketData points retained when
// invoking Strategy.GenerateSignal. Mirrors live engine default.
const HistoryLimit = 1000

// EngineConfig wires the engine.
type EngineConfig struct {
	Strategy        pkgstrategy.Strategy
	Datasource      Datasource
	Simulator       SimulatorConfig
	HistoryLimit    int // 0 → HistoryLimit
	WarmupBars      int // initial bars excluded from signal evaluation
	EquityEveryNBar int // sample equity every N bars (default: 1)
	OnProgress      func(barCount int, last Bar)
}

// Result is the output of a backtest run.
type Result struct {
	StrategyName string
	Symbol       string
	StartTime    time.Time
	EndTime      time.Time
	Bars         int
	Trades       []Trade
	Equity       []EquityPoint
	SignalCounts []SignalCount
	InitialCash  float64
	FinalCash    float64
	FinalEquity  float64
	Summary      Summary
}

// Engine drives the backtest.
type Engine struct {
	cfg EngineConfig
}

// NewEngine constructs an Engine; arguments are validated lazily in Run.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{cfg: cfg}
}

// Run executes the backtest until the datasource is exhausted.
func (e *Engine) Run(ctx context.Context) (*Result, error) {
	if e.cfg.Strategy == nil {
		return nil, errors.New("backtest: Strategy is required")
	}
	if e.cfg.Datasource == nil {
		return nil, errors.New("backtest: Datasource is required")
	}
	if e.cfg.Simulator.InitialBalance <= 0 {
		return nil, errors.New("backtest: Simulator.InitialBalance must be > 0")
	}
	histLimit := e.cfg.HistoryLimit
	if histLimit <= 0 {
		histLimit = HistoryLimit
	}
	equityEvery := e.cfg.EquityEveryNBar
	if equityEvery <= 0 {
		equityEvery = 1
	}

	// Lifecycle: start the strategy.
	if err := e.cfg.Strategy.Start(ctx); err != nil {
		return nil, fmt.Errorf("strategy.Start: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.cfg.Strategy.Stop(stopCtx)
	}()

	portfolio := NewPortfolio(e.cfg.Simulator.InitialBalance)
	sim := NewSimulator(e.cfg.Simulator, portfolio)

	history := make([]pkgstrategy.MarketData, 0, histLimit)
	signalCounts := make(map[string]int)

	var (
		barCount  int
		startTime time.Time
		endTime   time.Time
		symbol    string
		curr, nxt Bar
		hasCurr   bool
	)

	// Two-bar lookahead window: signals from bar N execute on bar N+1's open.
	// We always keep one buffered bar so that when we get the new bar, the
	// previously-buffered one becomes "curr" and the new one becomes "nxt".
	advance := func(b Bar) {
		if hasCurr {
			// Promote previous nxt → curr. (curr is what we evaluate.)
			curr = nxt
		}
		nxt = b
		hasCurr = true
	}

	for {
		bar, err := e.cfg.Datasource.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("datasource: %w", err)
		}
		if barCount == 0 {
			startTime = bar.Timestamp
			symbol = bar.Symbol
		}
		endTime = bar.Timestamp
		barCount++

		// First bar: just buffer.
		if !hasCurr {
			advance(bar)
			continue
		}
		// From here on we always have (curr, nxt). Process curr, then
		// shift in the new bar.
		processed := curr
		nextBar := bar

		// 1. Mark-to-market at curr's close.
		if barCount%equityEvery == 0 {
			portfolio.MarkToMarket(processed.Timestamp, processed.Close)
		}

		// 2. TP/SL on the open position using curr's high/low.
		// Position may be closed via TP/SL; signal evaluation still runs
		// so a re-entry can be considered on this bar.
		_, _ = sim.HandleExit(processed)

		// 3. Append to history & evaluate strategy if past warmup.
		md := barToMarketData(processed)
		history = append(history, md)
		if len(history) > histLimit {
			history = history[len(history)-histLimit:]
		}

		if barCount > e.cfg.WarmupBars {
			signal, err := e.cfg.Strategy.GenerateSignal(ctx, &md, history)
			if err != nil {
				return nil, fmt.Errorf("GenerateSignal at %s: %w", processed.Timestamp.Format(time.RFC3339), err)
			}
			if signal != nil {
				key := signalKey(signal)
				signalCounts[key]++
				_, _ = sim.HandleSignal(processed, nextBar, signal, true, e.cfg.Strategy)
			}
		}

		if e.cfg.OnProgress != nil {
			e.cfg.OnProgress(barCount, processed)
		}
		advance(bar)
	}

	// Drain the final buffered bar (no nxt → cannot enter, only mark / TP / SL).
	if hasCurr {
		// Process the last bar without a "next bar" — only mark-to-market and
		// TP/SL via the bar's own range.
		final := nxt
		_, _ = sim.HandleExit(final)
		portfolio.MarkToMarket(final.Timestamp, final.Close)
		// Force-close any remaining open position at last close.
		if portfolio.Position() != nil {
			_, _ = sim.ForceClose(final.Timestamp, final.Close)
			portfolio.MarkToMarket(final.Timestamp, final.Close)
		}
	}

	// Final equity sample.
	finalPrice := 0.0
	if len(history) > 0 {
		finalPrice = history[len(history)-1].Price
	}
	finalEq := portfolio.Cash()
	if portfolio.Position() != nil && finalPrice > 0 {
		finalEq += portfolio.Position().MarkValue(finalPrice)
	}

	res := &Result{
		StrategyName: e.cfg.Strategy.Name(),
		Symbol:       symbol,
		StartTime:    startTime,
		EndTime:      endTime,
		Bars:         barCount,
		Trades:       portfolio.Trades(),
		Equity:       portfolio.Equity(),
		SignalCounts: flattenSignalCounts(signalCounts),
		InitialCash:  e.cfg.Simulator.InitialBalance,
		FinalCash:    portfolio.Cash(),
		FinalEquity:  finalEq,
	}
	res.Summary = ComputeSummary(res)
	return res, nil
}

func barToMarketData(b Bar) pkgstrategy.MarketData {
	return pkgstrategy.MarketData{
		Symbol:    b.Symbol,
		Timestamp: b.Timestamp,
		Price:     b.Close,
		Volume:    b.Volume,
		Open:      b.Open,
		High:      b.High,
		Low:       b.Low,
		Close:     b.Close,
	}
}

func signalKey(s *pkgstrategy.Signal) string {
	reason := ""
	if s.Metadata != nil {
		if r, ok := s.Metadata["reason"].(string); ok {
			reason = r
		}
	}
	return string(s.Action) + "|" + reason
}

func flattenSignalCounts(m map[string]int) []SignalCount {
	out := make([]SignalCount, 0, len(m))
	for k, v := range m {
		// split on first '|'
		idx := -1
		for i := 0; i < len(k); i++ {
			if k[i] == '|' {
				idx = i
				break
			}
		}
		var act, reason string
		if idx < 0 {
			act = k
		} else {
			act = k[:idx]
			reason = k[idx+1:]
		}
		out = append(out, SignalCount{Action: act, Reason: reason, Count: v})
	}
	return out
}
