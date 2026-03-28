package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/bmf-san/gogocoin/pkg/engine"
    _ "github.com/bmf-san/gogocoin/example/strategy/scalping"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    done := make(chan struct{})
    go func() {
        defer close(done)
        err := engine.Run(ctx,
            engine.WithConfigPath("./configs/config.yaml"),
        )
        if err != nil {
            log.Printf("engine: %v", err)
            cancel()
        }
    }()

    log.Println("gogocoin example started. Press Ctrl+C to exit.")
    <-sigChan
    cancel()
    <-done
}
