package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmf-san/gogocoin/v1/pkg/engine"
	pkgstrategy "github.com/bmf-san/gogocoin/v1/pkg/strategy"
	pkgscalping "github.com/bmf-san/gogocoin/v1/pkg/strategy/scalping"
)

func main() {
	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		err := engine.Run(ctx,
			engine.WithStrategy("scalping", func() pkgstrategy.Strategy {
				return pkgscalping.NewDefault()
			}),
			engine.WithConfigPath("./configs/config.yaml"),
		)
		if err != nil {
			log.Printf("engine: %v", err)
			cancel()
		}
	}()

	log.Println("gogocoin started. Press Ctrl+C to exit.")
	<-sigChan
	cancel()
}
