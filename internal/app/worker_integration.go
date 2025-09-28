package app

import (
	"context"

	"github.com/bmf-san/gogocoin/v1/internal/worker"
)

// InitializeWorkerManager creates and configures the worker manager
// This is an example of how to integrate WorkerManager into the Application
func (app *Application) InitializeWorkerManager() (*worker.WorkerManager, error) {
	wm := worker.NewWorkerManager(app.logger)

	// Register workers
	// Example: Market data worker
	// marketDataWorker := worker.NewMarketDataWorker(...)
	// if err := wm.Register("market_data", marketDataWorker); err != nil {
	//     return nil, err
	// }

	// Example: Signal worker
	// signalWorker := worker.NewSignalWorker(...)
	// if err := wm.Register("signal", signalWorker); err != nil {
	//     return nil, err
	// }

	// Example: Strategy worker
	// strategyWorker := worker.NewStrategyWorker(...)
	// if err := wm.Register("strategy", strategyWorker); err != nil {
	//     return nil, err
	// }

	return wm, nil
}

// RunWithWorkerManager starts the application with WorkerManager
// This is an alternative to the current Run() method that uses WorkerManager
func (app *Application) RunWithWorkerManager(ctx context.Context) error {
	// Initialize worker manager
	wm, err := app.InitializeWorkerManager()
	if err != nil {
		return err
	}

	// Start all workers
	if err := wm.StartAll(ctx); err != nil {
		return err
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop all workers gracefully
	return wm.StopAll()
}

// GetWorkerHealth returns health status of all workers (if using WorkerManager)
// This can be called from API endpoints for monitoring
func (app *Application) GetWorkerHealth(wm *worker.WorkerManager) map[string]worker.HealthStatus {
	if wm == nil {
		return nil
	}
	return wm.HealthCheck()
}
