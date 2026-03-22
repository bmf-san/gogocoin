package worker

import (
	"context"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// Worker defines the standard interface for all background workers
// All workers should implement this interface for consistent lifecycle management
type Worker interface {
	// Run starts the worker (blocking until context is canceled)
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

// PositionReader fetches open buy positions for a symbol, used by stop-loss logic.
type PositionReader interface {
	GetOpenPositions(symbol string, side string) ([]domain.Position, error)
}

