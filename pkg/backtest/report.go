package backtest

import (
	"math"
	"sort"
	"time"
)

// Summary aggregates trade-level and equity-level statistics.
type Summary struct {
	TotalTrades      int
	Wins             int
	Losses           int
	WinRate          float64
	GrossProfit      float64
	GrossLoss        float64
	NetPnL           float64
	AvgPnL           float64
	BestTrade        float64
	WorstTrade       float64
	ProfitFactor     float64
	TotalFees        float64
	TotalSlippage    float64
	MaxDrawdown      float64 // ratio (e.g. -0.12 = -12 %)
	MaxDrawdownAbs   float64 // JPY
	MaxDrawdownAt    time.Time
	SharpeAnnualised float64 // assumes equity samples are uniform
	Sortino          float64
	TPHits           int
	SLHits           int
	SignalExits      int
	EODExits         int
	ExitReasons      map[ExitReason]int
	Days             float64
	ReturnPct        float64 // (finalEquity - initial) / initial * 100
	MonthlyPnL       []MonthlyPnL
}

// MonthlyPnL is the realized P&L for a single calendar month.
type MonthlyPnL struct {
	Year  int
	Month time.Month
	PnL   float64
}

// ComputeSummary derives a Summary from a Result. It does not mutate r.
func ComputeSummary(r *Result) Summary {
	s := Summary{ExitReasons: map[ExitReason]int{}}
	for _, t := range r.Trades {
		s.TotalTrades++
		s.NetPnL += t.NetPnL
		s.TotalFees += t.Fee
		s.TotalSlippage += t.Slippage
		s.ExitReasons[t.Reason]++
		switch t.Reason {
		case ExitReasonTP:
			s.TPHits++
		case ExitReasonSL:
			s.SLHits++
		case ExitReasonSignal:
			s.SignalExits++
		case ExitReasonEOD:
			s.EODExits++
		}
		if t.IsWin() {
			s.Wins++
			s.GrossProfit += t.NetPnL
		} else {
			s.Losses++
			s.GrossLoss += -t.NetPnL
		}
		if t.NetPnL > s.BestTrade {
			s.BestTrade = t.NetPnL
		}
		if t.NetPnL < s.WorstTrade {
			s.WorstTrade = t.NetPnL
		}
	}
	if s.TotalTrades > 0 {
		s.WinRate = float64(s.Wins) / float64(s.TotalTrades) * 100.0
		s.AvgPnL = s.NetPnL / float64(s.TotalTrades)
	}
	if s.GrossLoss > 0 {
		s.ProfitFactor = s.GrossProfit / s.GrossLoss
	} else if s.GrossProfit > 0 {
		s.ProfitFactor = math.Inf(1)
	}
	if r.InitialCash > 0 {
		s.ReturnPct = (r.FinalEquity - r.InitialCash) / r.InitialCash * 100.0
	}
	if !r.StartTime.IsZero() && !r.EndTime.IsZero() {
		s.Days = r.EndTime.Sub(r.StartTime).Hours() / 24.0
	}

	// Drawdown stats from equity curve.
	for _, p := range r.Equity {
		if p.Drawdown < s.MaxDrawdown {
			s.MaxDrawdown = p.Drawdown
			// Drawdown = (equity - peak) / peak  =>  abs = equity * dd / (1+dd).
			if 1+p.Drawdown > 0 {
				s.MaxDrawdownAbs = p.Equity * p.Drawdown / (1 + p.Drawdown)
			}
			s.MaxDrawdownAt = p.Timestamp
		}
	}

	// Sharpe / Sortino from equity returns.
	if len(r.Equity) > 2 {
		rets := equityReturns(r.Equity)
		s.SharpeAnnualised = annualisedSharpe(rets, len(r.Equity), s.Days)
		s.Sortino = annualisedSortino(rets, len(r.Equity), s.Days)
	}

	// Monthly P&L.
	monthly := map[[2]int]float64{}
	for _, t := range r.Trades {
		y, m, _ := t.ExitTime.Date()
		key := [2]int{y, int(m)}
		monthly[key] += t.NetPnL
	}
	for k, v := range monthly {
		s.MonthlyPnL = append(s.MonthlyPnL, MonthlyPnL{Year: k[0], Month: time.Month(k[1]), PnL: v})
	}
	sort.Slice(s.MonthlyPnL, func(i, j int) bool {
		a, b := s.MonthlyPnL[i], s.MonthlyPnL[j]
		if a.Year != b.Year {
			return a.Year < b.Year
		}
		return a.Month < b.Month
	})
	return s
}

// peakAtPoint is unused but retained for future drawdown reconstruction.
//
//nolint:unused
func peakAtPoint(curve []EquityPoint, p EquityPoint) float64 {
	peak := p.Equity
	for _, q := range curve {
		if !q.Timestamp.After(p.Timestamp) && q.Equity > peak {
			peak = q.Equity
		}
	}
	return peak
}

func equityReturns(curve []EquityPoint) []float64 {
	rets := make([]float64, 0, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Equity
		if prev <= 0 {
			continue
		}
		rets = append(rets, (curve[i].Equity-prev)/prev)
	}
	return rets
}

func annualisedSharpe(rets []float64, samples int, days float64) float64 {
	if len(rets) < 2 || days <= 0 {
		return 0
	}
	mean, std := meanStd(rets)
	if std == 0 {
		return 0
	}
	samplesPerDay := float64(samples) / days
	return mean / std * math.Sqrt(samplesPerDay*365.0)
}

func annualisedSortino(rets []float64, samples int, days float64) float64 {
	if len(rets) < 2 || days <= 0 {
		return 0
	}
	mean, _ := meanStd(rets)
	var downSum float64
	var n int
	for _, r := range rets {
		if r < 0 {
			downSum += r * r
			n++
		}
	}
	if n == 0 {
		return 0
	}
	downStd := math.Sqrt(downSum / float64(n))
	if downStd == 0 {
		return 0
	}
	samplesPerDay := float64(samples) / days
	return mean / downStd * math.Sqrt(samplesPerDay*365.0)
}

func meanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var ss float64
	for _, x := range xs {
		d := x - mean
		ss += d * d
	}
	return mean, math.Sqrt(ss / float64(len(xs)))
}
