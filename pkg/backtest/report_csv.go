package backtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// WriteCSVReports writes trades.csv, equity.csv, signals.csv, summary.json
// to dir. dir is created if missing.
func WriteCSVReports(dir string, r *Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeTrades(filepath.Join(dir, "trades.csv"), r.Trades); err != nil {
		return err
	}
	if err := writeEquity(filepath.Join(dir, "equity.csv"), r.Equity); err != nil {
		return err
	}
	if err := writeSignals(filepath.Join(dir, "signals.csv"), r.SignalCounts); err != nil {
		return err
	}
	return nil
}

func writeTrades(path string, trades []Trade) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"entry_time", "exit_time", "symbol", "side", "entry_price", "exit_price",
		"quantity", "notional", "gross_pnl", "fee", "slippage", "net_pnl",
		"reason", "hold_bars",
	}); err != nil {
		return err
	}
	for _, t := range trades {
		if err := w.Write([]string{
			t.EntryTime.Format(time.RFC3339Nano),
			t.ExitTime.Format(time.RFC3339Nano),
			t.Symbol,
			string(t.Side),
			ftoa(t.EntryPrice),
			ftoa(t.ExitPrice),
			ftoa(t.Quantity),
			ftoa(t.Notional),
			ftoa(t.GrossPnL),
			ftoa(t.Fee),
			ftoa(t.Slippage),
			ftoa(t.NetPnL),
			string(t.Reason),
			strconv.Itoa(t.HoldBars),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeEquity(path string, eq []EquityPoint) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"timestamp", "cash", "equity", "drawdown"}); err != nil {
		return err
	}
	for _, p := range eq {
		if err := w.Write([]string{
			p.Timestamp.Format(time.RFC3339Nano),
			ftoa(p.Cash),
			ftoa(p.Equity),
			ftoa(p.Drawdown),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeSignals(path string, counts []SignalCount) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sort.Slice(counts, func(i, j int) bool { return counts[i].Count > counts[j].Count })
	w := csv.NewWriter(f)
	if err := w.Write([]string{"action", "reason", "count"}); err != nil {
		return err
	}
	for _, c := range counts {
		if err := w.Write([]string{c.Action, c.Reason, strconv.Itoa(c.Count)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// PrintSummary writes a human-readable text summary to w.
func PrintSummary(w io.Writer, r *Result) {
	s := r.Summary
	_, _ = fmt.Fprintf(w, "===================================================\n")
	_, _ = fmt.Fprintf(w, " Backtest Result: %s / %s\n", r.StrategyName, r.Symbol)
	_, _ = fmt.Fprintf(w, "===================================================\n")
	_, _ = fmt.Fprintf(w, "Period:         %s ～ %s (%.1f days)\n",
		r.StartTime.Format("2006-01-02"), r.EndTime.Format("2006-01-02"), s.Days)
	_, _ = fmt.Fprintf(w, "Bars:           %d\n", r.Bars)
	_, _ = fmt.Fprintf(w, "Trades closed:  %d  (TP %d  /  SL %d  /  Signal %d  /  EOD %d)\n",
		s.TotalTrades, s.TPHits, s.SLHits, s.SignalExits, s.EODExits)
	_, _ = fmt.Fprintf(w, "\n[ P&L ]\n")
	_, _ = fmt.Fprintf(w, "  Net P&L:      %+.2f JPY  (%.2f %% on %.0f JPY)\n",
		s.NetPnL, s.ReturnPct, r.InitialCash)
	if s.TotalTrades > 0 {
		_, _ = fmt.Fprintf(w, "  Win rate:     %.1f %%  (%d W / %d L)\n", s.WinRate, s.Wins, s.Losses)
		_, _ = fmt.Fprintf(w, "  Avg / trade:  %+.2f JPY\n", s.AvgPnL)
		_, _ = fmt.Fprintf(w, "  Best / Worst: %+.2f / %+.2f JPY\n", s.BestTrade, s.WorstTrade)
		_, _ = fmt.Fprintf(w, "  Profit factor:%.3f\n", s.ProfitFactor)
	}
	_, _ = fmt.Fprintf(w, "\n[ Risk ]\n")
	_, _ = fmt.Fprintf(w, "  Max drawdown: %.2f %%  (%.2f JPY)\n", s.MaxDrawdown*100.0, s.MaxDrawdownAbs)
	if !s.MaxDrawdownAt.IsZero() {
		_, _ = fmt.Fprintf(w, "  DD timestamp: %s\n", s.MaxDrawdownAt.Format(time.RFC3339))
	}
	_, _ = fmt.Fprintf(w, "  Sharpe (ann): %.3f\n", s.SharpeAnnualised)
	_, _ = fmt.Fprintf(w, "  Sortino:      %.3f\n", s.Sortino)
	_, _ = fmt.Fprintf(w, "\n[ Costs ]\n")
	_, _ = fmt.Fprintf(w, "  Fees paid:    %.2f JPY\n", s.TotalFees)
	_, _ = fmt.Fprintf(w, "  Slippage:     %.2f JPY\n", s.TotalSlippage)
	if len(s.MonthlyPnL) > 0 {
		_, _ = fmt.Fprintf(w, "\n[ Monthly P&L ]\n")
		for _, m := range s.MonthlyPnL {
			_, _ = fmt.Fprintf(w, "  %04d-%02d: %+.2f JPY\n", m.Year, int(m.Month), m.PnL)
		}
	}
	if len(r.SignalCounts) > 0 {
		_, _ = fmt.Fprintf(w, "\n[ Top signal reasons ]\n")
		sorted := make([]SignalCount, len(r.SignalCounts))
		copy(sorted, r.SignalCounts)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
		for i, c := range sorted {
			if i >= 8 {
				break
			}
			_, _ = fmt.Fprintf(w, "  %-6s %-32s %d\n", c.Action, c.Reason, c.Count)
		}
	}
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
