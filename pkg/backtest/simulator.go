package backtest

import (
	"math"
	"time"

	pkgstrategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// Simulator implements the fill model. It is stateful so the engine can call
// Step() once per Bar without owning portfolio internals directly.
type Simulator struct {
	cfg       SimulatorConfig
	portfolio *Portfolio
}

// NewSimulator returns a Simulator wired to portfolio.
func NewSimulator(cfg SimulatorConfig, portfolio *Portfolio) *Simulator {
	if cfg.SameBarRule == "" {
		cfg.SameBarRule = SameBarSLPriority
	}
	return &Simulator{cfg: cfg, portfolio: portfolio}
}

// Portfolio returns the underlying portfolio (read-only convenience).
func (s *Simulator) Portfolio() *Portfolio { return s.portfolio }

// HandleExit checks whether the current Bar triggers TP/SL on the open
// position. Must be called BEFORE evaluating new signals on the same bar so
// that TP/SL are honored ahead of any new entry.
//
// It returns true when the position was closed.
func (s *Simulator) HandleExit(bar Bar) (Trade, bool) {
	pos := s.portfolio.Position()
	if pos == nil {
		return Trade{}, false
	}
	pos.BarsHeld++

	tpHit := pos.TPPrice > 0 && bar.High >= pos.TPPrice
	slHit := pos.SLPrice > 0 && bar.Low <= pos.SLPrice

	var (
		exitPrice float64
		reason    ExitReason
		hit       bool
	)
	switch {
	case tpHit && slHit:
		// Same-bar collision; resolve via configured rule.
		switch s.cfg.SameBarRule {
		case SameBarTPPriority:
			exitPrice, reason, hit = pos.TPPrice, ExitReasonTP, true
		case SameBarSkip:
			// Treat as no exit on this bar.
			hit = false
		default:
			exitPrice, reason, hit = pos.SLPrice, ExitReasonSL, true
		}
	case tpHit:
		exitPrice, reason, hit = pos.TPPrice, ExitReasonTP, true
	case slHit:
		exitPrice, reason, hit = pos.SLPrice, ExitReasonSL, true
	}
	if !hit {
		return Trade{}, false
	}
	exitProceeds := exitPrice * pos.Quantity
	exitSlip := exitProceeds * s.cfg.SlippageBps / 10_000.0
	exitFee := exitProceeds * s.cfg.FeeRate
	tr := s.portfolio.CloseLong(bar.Timestamp, exitPrice, exitFee, exitSlip, reason)
	return tr, true
}

// HandleSignal applies a strategy Signal to the portfolio. nextOpen is the
// open price of the NEXT bar — entries fill there to prevent lookahead bias.
// When isLastBar is true and there is still an open position, it is closed
// at bar.Close (fee+slippage applied).
func (s *Simulator) HandleSignal(bar, nextBar Bar, signal *pkgstrategy.Signal, hasNext bool, strat pkgstrategy.Strategy) (Trade, bool) {
	if signal == nil {
		return Trade{}, false
	}
	switch signal.Action {
	case pkgstrategy.SignalBuy:
		if s.portfolio.Position() != nil {
			// Already long; ignore.
			return Trade{}, false
		}
		if !hasNext {
			// Cannot fill without a next bar.
			return Trade{}, false
		}
		entryRaw := nextBar.Open
		// Slippage: pay above the open.
		slipPct := s.cfg.SlippageBps / 10_000.0
		entryPrice := entryRaw * (1.0 + slipPct)
		notionalReq := strat.GetBaseNotional(bar.Symbol)
		if notionalReq <= 0 {
			return Trade{}, false
		}
		// Volume sanity check (optional).
		if s.cfg.MinVolumeRatio > 0 && nextBar.Volume > 0 {
			required := s.cfg.MinVolumeRatio * notionalReq / entryPrice
			if nextBar.Volume < required {
				return Trade{}, false
			}
		}
		// Cash check.
		if s.portfolio.Cash() < notionalReq*(1.0+s.cfg.FeeRate+slipPct) {
			return Trade{}, false
		}
		qty := notionalReq / entryPrice
		notionalActual := entryPrice * qty
		entryFee := notionalActual * s.cfg.FeeRate
		entrySlip := notionalActual * slipPct
		pos := Position{
			Symbol:     bar.Symbol,
			EntryTime:  nextBar.Timestamp,
			EntryPrice: entryPrice,
			Quantity:   qty,
			Notional:   notionalActual,
			EntryFee:   entryFee,
			EntrySlip:  entrySlip,
			TPPrice:    strat.GetTakeProfitPrice(entryPrice),
			SLPrice:    strat.GetStopLossPrice(entryPrice),
		}
		s.portfolio.OpenLong(pos)
		// Notify strategy so cooldown / daily counter advance.
		strat.RecordTrade()

	case pkgstrategy.SignalSell:
		if s.portfolio.Position() == nil {
			return Trade{}, false
		}
		if !hasNext {
			return Trade{}, false
		}
		exitRaw := nextBar.Open
		slipPct := s.cfg.SlippageBps / 10_000.0
		exitPrice := exitRaw * (1.0 - slipPct)
		pos := s.portfolio.Position()
		exitProceeds := exitPrice * pos.Quantity
		exitSlip := exitProceeds * slipPct
		exitFee := exitProceeds * s.cfg.FeeRate
		tr := s.portfolio.CloseLong(nextBar.Timestamp, exitPrice, exitFee, exitSlip, ExitReasonSignal)
		strat.RecordTrade()
		return tr, true
	}
	return Trade{}, false
}

// ForceClose closes any open position at price (no slippage adjustment beyond
// fee). Used at end-of-data to liquidate so equity is comparable.
func (s *Simulator) ForceClose(t time.Time, price float64) (Trade, bool) {
	if s.portfolio.Position() == nil {
		return Trade{}, false
	}
	exitProceeds := price * s.portfolio.Position().Quantity
	exitFee := exitProceeds * s.cfg.FeeRate
	tr := s.portfolio.CloseLong(t, price, exitFee, 0, ExitReasonEOD)
	return tr, true
}

// roundLot rounds qty down to the nearest multiple of step. step <= 0 means
// no rounding.
//
//nolint:unused // reserved for future per-symbol lot-size handling
func roundLot(qty, step float64) float64 {
	if step <= 0 {
		return qty
	}
	return math.Floor(qty/step) * step
}
