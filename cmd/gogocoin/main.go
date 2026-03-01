package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func main() {
	// Load configuration file
	configPath := "./configs/config.yaml"
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration file: %v", err)
	}

	// Initialize logger
	loggerConfig := logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAgeDays: cfg.Logging.MaxAgeDays,
		Categories: cfg.Logging.Categories,
	}
	appLogger, err := logger.New(&loggerConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := appLogger.Close(); err != nil {
			log.Printf("Failed to close logger: %v", err)
		}
	}()

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Application startup log
	appLogger.LogStartup("v1.0.0", map[string]interface{}{
		"strategy":    cfg.Trading.Strategy.Name,
		"symbols":     cfg.Trading.Symbols,
		"web_enabled": true,
		"web_port":    cfg.UI.Port,
	})

	// Run application
	go func() {
		if err := Run(ctx, cfg, appLogger); err != nil {
			appLogger.LogError("system", "app_run", err, nil)
			appLogger.System().WithError(err).Error("Application failed")
			cancel()
		}
	}()

	// Wait for signal
	log.Println("gogocoin started. Press Ctrl+C to exit.")
	<-sigChan

	// Shutdown process
	appLogger.LogShutdown("user_interrupt")
	appLogger.System().Info("Shutting down gogocoin...")
	cancel()

	appLogger.System().Info("gogocoin shut down successfully")
}
