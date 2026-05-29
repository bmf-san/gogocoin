package cli

// Package cli implements the backtest CLI subcommands.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bmf-san/gogocoin/pkg/backtest"
	pkgstrategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// buildDatasource constructs a Datasource from cfg overlaid with optional
// from/to overrides (used by walk-forward). If cfg.BarPeriod is set, the
// raw datasource is wrapped in a ResamplingDatasource.
func buildDatasource(cfg backtest.DataConfig, fromOverride, toOverride time.Time) (backtest.Datasource, error) {
	from, err := backtest.ParseDate(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("data.from: %w", err)
	}
	to, err := backtest.ParseDate(cfg.To)
	if err != nil {
		return nil, fmt.Errorf("data.to: %w", err)
	}
	if !fromOverride.IsZero() {
		from = fromOverride
	}
	if !toOverride.IsZero() {
		to = toOverride
	}
	period, err := backtest.ParseBarPeriod(cfg.BarPeriod)
	if err != nil {
		return nil, fmt.Errorf("data.bar_period: %w", err)
	}
	var inner backtest.Datasource
	switch cfg.Source {
	case "", "sqlite":
		inner, err = backtest.NewSQLiteDatasource(backtest.SQLiteDatasourceOptions{
			Path:   cfg.Path,
			Symbol: cfg.Symbol,
			From:   from,
			To:     to,
		})
	case "csv":
		inner, err = backtest.NewCSVDatasource(backtest.CSVDatasourceOptions{
			Path:   cfg.Path,
			Symbol: cfg.Symbol,
			From:   from,
			To:     to,
		})
	default:
		return nil, fmt.Errorf("unknown data.source %q", cfg.Source)
	}
	if err != nil {
		return nil, err
	}
	if period > 0 {
		return backtest.NewResamplingDatasource(inner, period)
	}
	return inner, nil
}

// buildStrategy creates an initialized Strategy from a scenario.
func buildStrategy(name string, params map[string]interface{}) (pkgstrategy.Strategy, error) {
	s, err := pkgstrategy.Create(name)
	if err != nil {
		return nil, fmt.Errorf("strategy %q: %w", name, err)
	}
	if params != nil {
		if err := s.Initialize(params); err != nil {
			return nil, fmt.Errorf("initialize: %w", err)
		}
	}
	if err := s.Reset(); err != nil {
		return nil, fmt.Errorf("reset: %w", err)
	}
	return s, nil
}

// runScenario is the shared inner loop for `run` and `compare`.
func runScenario(ctx context.Context, cfg *backtest.Config, scenarioName string,
	from, to time.Time, paramsOverride map[string]interface{},
	outDir string, quiet bool,
) (*backtest.Result, error) {
	sc, ok := cfg.Scenarios[scenarioName]
	if !ok {
		return nil, fmt.Errorf("scenario %q not found", scenarioName)
	}
	params := mergeParams(sc.Params, paramsOverride)
	strat, err := buildStrategy(sc.Strategy, params)
	if err != nil {
		return nil, err
	}
	ds, err := buildDatasource(sc.DataOverride.Apply(cfg.Data), from, to)
	if err != nil {
		return nil, err
	}
	defer ds.Close()
	res, err := backtest.NewEngine(backtest.EngineConfig{
		Strategy:     strat,
		Datasource:   ds,
		Simulator:    cfg.Simulator.ToSimulatorConfig(),
		HistoryLimit: sc.HistoryLimit,
	}).Run(ctx)
	if err != nil {
		return nil, err
	}
	if outDir != "" {
		if err := backtest.WriteCSVReports(outDir, res); err != nil {
			return nil, fmt.Errorf("write reports: %w", err)
		}
	}
	if !quiet {
		backtest.PrintSummary(os.Stdout, res)
	}
	return res, nil
}

func mergeParams(base, overlay map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// outDirOrEmpty resolves a possibly-relative output dir.
func outDirOrEmpty(s string) string {
	if s == "" {
		return ""
	}
	abs, err := filepath.Abs(s)
	if err != nil {
		return s
	}
	return abs
}
