package cli

import (
	"context"
	"flag"

	"github.com/bmf-san/gogocoin/pkg/backtest"
)

// Run implements `backtest run`.
func Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "configs/backtest.yaml", "path to backtest YAML config")
	scenario := fs.String("scenario", "", "scenario name to run (required)")
	from := fs.String("from", "", "override data.from (YYYY-MM-DD)")
	to := fs.String("to", "", "override data.to (YYYY-MM-DD)")
	out := fs.String("out", "", "output directory for trades.csv / equity.csv / signals.csv")
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
	fromT, err := backtest.ParseDate(*from)
	if err != nil {
		return err
	}
	toT, err := backtest.ParseDate(*to)
	if err != nil {
		return err
	}
	_, err = runScenario(ctx, cfg, *scenario, fromT, toT, nil, outDirOrEmpty(*out), false)
	return err
}
