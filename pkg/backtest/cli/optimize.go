package cli

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/pkg/backtest"
)

// Optimize implements `backtest optimize`.
//
// It expands scenarios[name].grid into the cartesian product, runs each
// combination, and writes a ranking CSV (one row per combination).
func Optimize(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("optimize", flag.ContinueOnError)
	configPath := fs.String("config", "configs/backtest.yaml", "path to backtest YAML config")
	scenario := fs.String("scenario", "", "scenario name (required, must define a `grid` block)")
	out := fs.String("out", "out/optimize", "output directory")
	workers := fs.Int("workers", 4, "number of parallel workers")
	sortBy := fs.String("sort", "sharpe", "sort key: sharpe|net_pnl|win_rate|profit_factor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenario == "" {
		fs.Usage()
		return flag.ErrHelp
	}
	cfg, err := backtest.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	sc, ok := cfg.Scenarios[*scenario]
	if !ok {
		return fmt.Errorf("scenario %q not found", *scenario)
	}
	if len(sc.Grid) == 0 {
		return fmt.Errorf("scenario %q has no grid block", *scenario)
	}

	combos := expandGrid(sc.Grid, sc.Fixed)
	fmt.Fprintf(os.Stderr, "optimizing %s: %d combinations, %d workers\n", *scenario, len(combos), *workers)

	type result = optResult
	jobs := make(chan map[string]interface{})
	results := make(chan result, len(combos))
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for params := range jobs {
				r, err := runScenario(ctx, cfg, *scenario, time.Time{}, time.Time{}, params, "", true)
				results <- result{params: params, res: r, err: err}
			}
		}()
	}
	go func() {
		for _, c := range combos {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- c:
			}
		}
		close(jobs)
	}()
	go func() { wg.Wait(); close(results) }()

	rows := make([]result, 0, len(combos))
	for r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "  combo failed: %v\n", r.err)
			continue
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return cmpKey(rows[i].res, *sortBy) > cmpKey(rows[j].res, *sortBy)
	})

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	csvPath := filepath.Join(*out, "grid.csv")
	if err := writeGridCSV(csvPath, rows, sortedKeys(sc.Grid)); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d rows, sorted by %s)\n", csvPath, len(rows), *sortBy)
	if len(rows) > 0 {
		printTopRows(rows, sortedKeys(sc.Grid), 10)
	}
	return nil
}

// expandGrid is the cartesian product over `grid`, merging in `fixed`.
func expandGrid(grid map[string][]interface{}, fixed map[string]interface{}) []map[string]interface{} {
	keys := sortedKeys(grid)
	combos := []map[string]interface{}{copyParams(fixed)}
	for _, k := range keys {
		next := make([]map[string]interface{}, 0, len(combos)*len(grid[k]))
		for _, c := range combos {
			for _, v := range grid[k] {
				nc := copyParams(c)
				nc[k] = v
				next = append(next, nc)
			}
		}
		combos = next
	}
	return combos
}

func sortedKeys(m map[string][]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func copyParams(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cmpKey(r *backtest.Result, key string) float64 {
	switch key {
	case "win_rate":
		return r.Summary.WinRate
	case "net_pnl":
		return r.Summary.NetPnL
	case "profit_factor":
		return r.Summary.ProfitFactor
	case "sharpe":
		return r.Summary.SharpeAnnualised
	default:
		return r.Summary.SharpeAnnualised
	}
}

type optResult struct {
	params map[string]interface{}
	res    *backtest.Result
	err    error
}

func writeGridCSV(path string, rows []optResult, gridKeys []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := append([]string{}, gridKeys...)
	header = append(header, "trades", "win_rate", "net_pnl", "profit_factor",
		"max_dd_pct", "sharpe", "sortino", "fees", "slippage")
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		row := make([]string, 0, len(header))
		for _, k := range gridKeys {
			row = append(row, fmt.Sprintf("%v", r.params[k]))
		}
		s := r.res.Summary
		row = append(row,
			strconv.Itoa(s.TotalTrades),
			fmtFloat(s.WinRate),
			fmtFloat(s.NetPnL),
			fmtFloat(s.ProfitFactor),
			fmtFloat(s.MaxDrawdown*100),
			fmtFloat(s.SharpeAnnualised),
			fmtFloat(s.Sortino),
			fmtFloat(s.TotalFees),
			fmtFloat(s.TotalSlippage),
		)
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func printTopRows(rows []optResult, gridKeys []string, n int) {
	if n > len(rows) {
		n = len(rows)
	}
	fmt.Println("\nTop", n, "results:")
	for i := 0; i < n; i++ {
		s := rows[i].res.Summary
		fmt.Printf("  #%-2d  trades=%-4d  win=%5.1f%%  net=%+8.1f  PF=%5.2f  Sharpe=%6.2f  ",
			i+1, s.TotalTrades, s.WinRate, s.NetPnL, s.ProfitFactor, s.SharpeAnnualised)
		for _, k := range gridKeys {
			fmt.Printf("%s=%v ", k, rows[i].params[k])
		}
		fmt.Println()
	}
}

func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
