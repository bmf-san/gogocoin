package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmf-san/gogocoin/v1/internal/app"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func main() {
	// Parse command line arguments
	var (
		configPath = flag.String("config", "./configs/config.yaml", "Path to config file")
		mode       = flag.String("mode", "", "Execution mode (paper, live)")
		logLevel   = flag.String("log-level", "", "Log level (debug, info, warn, error)")
		initDB     = flag.Bool("init-db", false, "Initialize database")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	// Show version information
	if *version {
		fmt.Println("gogocoin v1.0.0")
		fmt.Println("A cryptocurrency trading bot for bitFlyer exchange")
		return
	}

	// Load configuration file
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration file: %v", err)
	}

	// Override configuration with command line arguments
	if *mode == "" {
		log.Fatalf("Please specify execution mode: -mode paper or -mode live")
	}

	// Validate mode
	if *mode != "paper" && *mode != "live" && *mode != "dev" {
		log.Fatalf("Invalid execution mode: %s (please specify paper, live, or dev)", *mode)
	}

	cfg.Mode = *mode

	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
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

	// Initialize application
	application, err := app.New(cfg, appLogger)
	if err != nil {
		appLogger.LogError("system", "app_initialization", err, map[string]interface{}{
			"config_path": *configPath,
		})
		// Use return to execute defer statements
		appLogger.System().WithError(err).Error("Failed to initialize application")
		return
	}
	defer func() {
		if err := application.Close(); err != nil {
			appLogger.System().WithError(err).Error("Failed to close application")
		}
	}()

	// Database initialization mode
	if *initDB {
		appLogger.System().Info("Initializing database...")
		if err := application.InitializeDatabase(); err != nil {
			appLogger.LogError("system", "database_initialization", err, nil)
			appLogger.System().WithError(err).Error("Failed to initialize database")
			return
		}
		appLogger.System().Info("Database initialization completed")
		return
	}

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Application startup log
	appLogger.LogStartup("v1.0.0", cfg.Mode, map[string]interface{}{
		"strategy":    cfg.Trading.Strategy.Name,
		"symbols":     cfg.Trading.Symbols,
		"paper_mode":  cfg.Mode == "paper",
		"web_enabled": true, // Always enabled
		"web_port":    cfg.API.Port,
	})

	// Run application (Run() includes Start() functionality)
	appLogger.System().Info("Starting gogocoin...")
	go func() {
		if err := application.Run(ctx); err != nil {
			appLogger.LogError("system", "app_run", err, nil)
			appLogger.System().WithError(err).Error("Failed to run application")
			cancel() // Cancel context on error
		}
	}()

	// Wait for signal
	appLogger.System().Info("gogocoin started successfully. Press Ctrl+C to exit.")
	<-sigChan

	// Shutdown process
	appLogger.LogShutdown("user_interrupt")
	appLogger.System().Info("Shutting down gogocoin...")

	// Stop application
	if err := application.Stop(); err != nil {
		appLogger.LogError("system", "app_stop", err, nil)
		log.Printf("Error occurred while stopping application: %v", err)
	}

	appLogger.System().Info("gogocoin shut down successfully")
}
