package cli

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bmf-san/gogocoin/pkg/backtest"
)

// wfWindow holds train + test window results for one walk-forward step.
type wfWindow struct {
	TrainFrom, TrainTo time.Time
	TestFrom, TestTo   time.Time
	BestParams         map[string]interface{}
	Train              *backtest.Result
	Test               *backtest.Result
}

// WalkForward implements `backtest walkforward`.
//
// Splits the configured date range into rolling [train, test] windows.
// On each train window we run a quick grid search to pick the best params,
// then evaluate them on the immediately-following test window. The output
// CSV records test-window stats — these are the only honest metrics.
func WalkForward(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("walkforward", flag.ContinueOnError)
	configPath := fs.String("config", "configs/backtest.yaml", "path to backtest YAML config")
	scenario := fs.String("scenario", "", "scenario name (must define a `grid` block)")
	trainDays := fs.Int("train-days", 60, "size of training window")
	testDays := fs.Int("test-days", 14, "size of testing window")
	step := fs.Int("step", 14, "step size between windows (days)")
	out := fs.String("out", "out/walkforward", "output directory")
	sortBy := fs.String("sort", "sharpe", "in-sample selection key (sharpe|net_pnl|win_rate|profit_factor)")
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
		return fmt.Errorf("scenario %q needs a grid block", *scenario)
	}
	from, err := backtest.ParseDate(cfg.Data.From)
	if err != nil || from.IsZero() {
		return fmt.Errorf("data.from required for walk-forward")
	}
	to, err := backtest.ParseDate(cfg.Data.To)
	if err != nil || to.IsZero() {
		return fmt.Errorf("data.to required for walk-forward")
	}

	combos := expandGrid(sc.Grid, sc.Fixed)
	gridKeys := sortedKeys(sc.Grid)

	type windowResult = wfWindow
	var windows []windowResult

	cursor := from
	for {
		trainTo := cursor.AddDate(0, 0, *trainDays)
		testTo := trainTo.AddDate(0, 0, *testDays)
		if testTo.After(to) {
			break
		}
		fmt.Fprintf(os.Stderr, "window: train %s..%s  test %s..%s\n",
			cursor.Format("2006-01-02"), trainTo.Format("2006-01-02"),
			trainTo.Format("2006-01-02"), testTo.Format("2006-01-02"))

		// Grid search on training window.
		var best *backtest.Result
		var bestParams map[string]interface{}
		for _, params := range combos {
			r, err := runScenario(ctx, cfg, *scenario, cursor, trainTo, params, "", true)
			if err != nil {
				continue
			}
			if best == nil || cmpKey(r, *sortBy) > cmpKey(best, *sortBy) {
				best = r
				bestParams = params
			}
		}
		if best == nil {
			fmt.Fprintln(os.Stderr, "  no successful train run; skipping")
			cursor = cursor.AddDate(0, 0, *step)
			continue
		}
		// Evaluate best params on test window.
		test, err := runScenario(ctx, cfg, *scenario, trainTo, testTo, bestParams, "", true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  test failed: %v\n", err)
			cursor = cursor.AddDate(0, 0, *step)
			continue
		}
		windows = append(windows, windowResult{
			TrainFrom: cursor, TrainTo: trainTo,
			TestFrom: trainTo, TestTo: testTo,
			BestParams: bestParams,
			Train:      best, Test: test,
		})
		cursor = cursor.AddDate(0, 0, *step)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	csvPath := filepath.Join(*out, "windows.csv")
	if err := writeWalkForwardCSV(csvPath, windows, gridKeys); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d windows)\n", csvPath, len(windows))
	if len(windows) == 0 {
		return nil
	}

	// Aggregate test-window metrics.
	var totalNet, totalTrades float64
	var winsSum, lossesSum int
	for _, w := range windows {
		totalNet += w.Test.Summary.NetPnL
		totalTrades += float64(w.Test.Summary.TotalTrades)
		winsSum += w.Test.Summary.Wins
		lossesSum += w.Test.Summary.Losses
	}
	winRate := 0.0
	if winsSum+lossesSum > 0 {
		winRate = float64(winsSum) / float64(winsSum+lossesSum) * 100
	}
	fmt.Printf("\n[ Walk-forward aggregate (out-of-sample only) ]\n")
	fmt.Printf("  windows:    %d\n", len(windows))
	fmt.Printf("  trades:     %.0f\n", totalTrades)
	fmt.Printf("  win rate:   %.2f %%\n", winRate)
	fmt.Printf("  total P&L:  %+.2f JPY\n", totalNet)
	return nil
}

func writeWalkForwardCSV(path string, windows []wfWindow, gridKeys []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{"train_from", "train_to", "test_from", "test_to"}
	for _, k := range gridKeys {
		header = append(header, "best_"+k)
	}
	header = append(header,
		"train_trades", "train_win_rate", "train_net_pnl", "train_sharpe",
		"test_trades", "test_win_rate", "test_net_pnl", "test_sharpe")
	if err := w.Write(header); err != nil {
		return err
	}
	for _, ww := range windows {
		row := []string{
			ww.TrainFrom.Format("2006-01-02"),
			ww.TrainTo.Format("2006-01-02"),
			ww.TestFrom.Format("2006-01-02"),
			ww.TestTo.Format("2006-01-02"),
		}
		for _, k := range gridKeys {
			row = append(row, fmt.Sprintf("%v", ww.BestParams[k]))
		}
		row = append(row,
			strconv.Itoa(ww.Train.Summary.TotalTrades),
			fmtFloat(ww.Train.Summary.WinRate),
			fmtFloat(ww.Train.Summary.NetPnL),
			fmtFloat(ww.Train.Summary.SharpeAnnualised),
			strconv.Itoa(ww.Test.Summary.TotalTrades),
			fmtFloat(ww.Test.Summary.WinRate),
			fmtFloat(ww.Test.Summary.NetPnL),
			fmtFloat(ww.Test.Summary.SharpeAnnualised),
		)
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
