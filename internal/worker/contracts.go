package worker

import (
	"context"
	"time"
)

// Worker defines the standard interface for all background workers
// All workers should implement this interface for consistent lifecycle management
type Worker interface {
	// Run starts the worker (blocking until context is cancelled)
	Run(ctx context.Context) error

	// Name returns the worker name for logging and identification
	Name() string
}

// HealthChecker defines health checking capability for workers
type HealthChecker interface {
	// Health returns the current health status
	Health() HealthStatus
}

// HealthStatus represents worker health state
type HealthStatus struct {
	Healthy      bool
	Message      string
	LastCheck    time.Time
	ErrorCount   int
	UptimeMillis int64
}

// Stoppable defines graceful stop capability
type Stoppable interface {
	// Stop gracefully stops the worker
	Stop() error
}

// WorkerWithHealth combines Worker with health checking
type WorkerWithHealth interface {
	Worker
	HealthChecker
}

// WorkerWithStop combines Worker with graceful stop
type WorkerWithStop interface {
	Worker
	Stoppable
}

// FullWorker combines all worker capabilities
type FullWorker interface {
	Worker
	HealthChecker
	Stoppable
}
