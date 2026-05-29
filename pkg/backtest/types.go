// Package backtest provides a deterministic, strategy-agnostic backtesting
// engine for the gogocoin trading framework.
//
// The engine replays historical market data through a strategy.Strategy
// implementation, simulates fills with a configurable fee/slippage model,
// and produces detailed performance reports (P&L, win rate, drawdown,
// Sharpe, profit factor, etc.).
//
// It is intentionally decoupled from the live trading engine so that the
// same Strategy code path can be validated against historical data before
// being deployed to production.
package backtest

import "time"

// Bar represents a single OHLCV candle. The backtest engine processes one
// Bar per step in chronological order.
type Bar struct {
	Symbol    string
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// Side is the direction of an open position. The backtest is long-only:
// SideLong is opened by a strategy BUY signal and closed by SELL / TP / SL.
type Side string

const (
	SideLong Side = "LONG"
)

// ExitReason describes why a position was closed.
type ExitReason string

const (
	ExitReasonSignal ExitReason = "signal" // strategy emitted SELL
	ExitReasonTP     ExitReason = "tp"     // take-profit hit
	ExitReasonSL     ExitReason = "sl"     // stop-loss hit
	ExitReasonEOD    ExitReason = "end"    // end of data while still holding
)

// Trade is a single completed round-trip (entry + exit).
type Trade struct {
	Symbol     string
	Side       Side
	EntryTime  time.Time
	EntryPrice float64
	ExitTime   time.Time
	ExitPrice  float64
	Quantity   float64 // base asset quantity
	Notional   float64 // JPY notional at entry (price * qty before slippage)
	GrossPnL   float64 // (exit - entry) * qty
	Fee        float64 // round-trip fee
	Slippage   float64 // total slippage cost (entry + exit)
	NetPnL     float64 // GrossPnL - Fee - Slippage
	Reason     ExitReason
	HoldBars   int
}

// IsWin returns true when NetPnL is strictly positive.
func (t Trade) IsWin() bool { return t.NetPnL > 0 }

// EquityPoint is one sample of the equity curve, taken once per Bar.
type EquityPoint struct {
	Timestamp time.Time
	Cash      float64 // realized cash (JPY)
	Equity    float64 // mark-to-market equity (cash + unrealized)
	Drawdown  float64 // peak-to-current drawdown ratio (negative or zero)
}

// SignalCount summarizes how often each (action, reason) pair was emitted.
// Used for diagnostic output ("why did the strategy not enter more trades?").
type SignalCount struct {
	Action string
	Reason string
	Count  int
}

// SameBarRule controls how a single Bar that contains both TP and SL prices
// is resolved. Real-life ordering is unknown, so the user picks a model.
type SameBarRule string

const (
	SameBarSLPriority SameBarRule = "sl_priority" // pessimistic (default)
	SameBarTPPriority SameBarRule = "tp_priority"
	SameBarSkip       SameBarRule = "skip" // close at Bar.Close, no TP/SL
)

// SimulatorConfig controls the fill model.
type SimulatorConfig struct {
	InitialBalance float64     // starting cash in JPY
	FeeRate        float64     // taker fee, e.g. 0.0015 = 0.15 %
	SlippageBps    float64     // slippage in basis points (1 bp = 0.01 %)
	SameBarRule    SameBarRule // how to resolve same-bar TP+SL
	// MinVolumeRatio rejects fills when the bar volume is below
	// MinVolumeRatio * (notional / price). 0 disables the check.
	MinVolumeRatio float64
}

// DefaultSimulatorConfig returns a conservative default configuration.
func DefaultSimulatorConfig() SimulatorConfig {
	return SimulatorConfig{
		InitialBalance: 100_000,
		FeeRate:        0.0015,
		SlippageBps:    5,
		SameBarRule:    SameBarSLPriority,
		MinVolumeRatio: 0,
	}
}
