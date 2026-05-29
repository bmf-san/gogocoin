package cli

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bmf-san/gogocoin/pkg/backtest"
)

// Compare implements `backtest compare`.
//
// Runs N scenarios against the same data range and writes a comparison CSV.
func Compare(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	configPath := fs.String("config", "configs/backtest.yaml", "path to backtest YAML config")
	scenarios := fs.String("scenarios", "", "comma-separated scenario names (required)")
	out := fs.String("out", "out/compare", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarios == "" {
		fs.Usage()
		return flag.ErrHelp
	}
	cfg, err := backtest.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	names := splitTrim(*scenarios)
	type row struct {
		name string
		res  *backtest.Result
	}
	rows := make([]row, 0, len(names))
	for _, name := range names {
		r, err := runScenario(ctx, cfg, name, time.Time{}, time.Time{}, nil,
			filepath.Join(*out, name), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenario %q failed: %v\n", name, err)
			continue
		}
		rows = append(rows, row{name: name, res: r})
		fmt.Println()
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	csvPath := filepath.Join(*out, "summary.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"scenario", "trades", "win_rate", "net_pnl",
		"profit_factor", "max_dd_pct", "sharpe", "sortino", "fees", "slippage"}); err != nil {
		return err
	}
	for _, r := range rows {
		s := r.res.Summary
		if err := w.Write([]string{
			r.name,
			strconv.Itoa(s.TotalTrades),
			fmtFloat(s.WinRate),
			fmtFloat(s.NetPnL),
			fmtFloat(s.ProfitFactor),
			fmtFloat(s.MaxDrawdown * 100),
			fmtFloat(s.SharpeAnnualised),
			fmtFloat(s.Sortino),
			fmtFloat(s.TotalFees),
			fmtFloat(s.TotalSlippage),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", csvPath)
	return nil
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
