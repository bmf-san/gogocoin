// Command backtest is the gogocoin CLI for offline strategy validation.
//
// Subcommands:
//   - run         Single scenario backtest.
//   - optimize    Grid search over scenario.grid → CSV ranking.
//   - walkforward In-sample / out-of-sample analysis.
//   - compare     Run multiple scenarios and write a comparison CSV.
//
// All subcommands read configs/backtest.yaml by default.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmf-san/gogocoin/pkg/backtest/cli"

	// Register strategies via blank imports.
	_ "github.com/bmf-san/gogocoin/pkg/strategy/scalping"
)

func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) < 2 {
		usage()
		return 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sigCh; cancel() }()

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "run":
		err = cli.Run(ctx, args)
	case "optimize":
		err = cli.Optimize(ctx, args)
	case "walkforward":
		err = cli.WalkForward(ctx, args)
	case "compare":
		err = cli.Compare(ctx, args)
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		return 2
	}
	if err == nil {
		return 0
	}
	if err == flag.ErrHelp {
		return 0
	}
	_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func usage() {
	fmt.Fprintln(os.Stderr, `backtest — offline strategy validation

Usage:
  backtest <command> [flags]

Commands:
  run         Run a single scenario.
  optimize    Grid search over a scenario's grid block.
  walkforward Walk-forward analysis (train/test rolling windows).
  compare     Run multiple scenarios and produce a comparison CSV.

Run "backtest <command> -h" for command-specific flags.`)
}
