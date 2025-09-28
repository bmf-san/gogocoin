package worker

import (
	"context"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// StrategyMonitorWorker monitors strategy health
type StrategyMonitorWorker struct {
	logger          *logger.Logger
	strategyGetter  StrategyGetter
	checkInterval   time.Duration
}

// StrategyGetter defines the interface for getting current strategy
type StrategyGetter interface {
	GetCurrentStrategy() strategy.Strategy
}

// NewStrategyMonitorWorker creates a new strategy monitor worker
func NewStrategyMonitorWorker(logger *logger.Logger, strategyGetter StrategyGetter) *StrategyMonitorWorker {
	return &StrategyMonitorWorker{
		logger:         logger,
		strategyGetter: strategyGetter,
		checkInterval:  5 * time.Minute,
	}
}

// Run starts the strategy monitor worker
func (w *StrategyMonitorWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	w.logger.System().Info("Strategy monitor worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.System().Info("Strategy monitor worker stopped")
			return
		case <-ticker.C:
			w.checkAndResetStrategy()
		}
	}
}

// checkAndResetStrategy is no longer needed as we only have scalping strategy
// This function is kept as a placeholder for future strategy-specific logic if needed
func (w *StrategyMonitorWorker) checkAndResetStrategy() {
	if w.strategyGetter == nil {
		return
	}

	currentStrategy := w.strategyGetter.GetCurrentStrategy()
	if currentStrategy == nil {
		return
	}
	// No strategy-specific checks needed for scalping
}
