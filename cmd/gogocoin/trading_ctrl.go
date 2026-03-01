package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// TradingController manages the trading enabled/disabled state.
// It satisfies both:
//   - adapter/http.ApplicationService (IsTradingEnabled, SetTradingEnabled, …)
//   - adapter/worker.TradingEnabledGetter (IsTradingEnabled)
type TradingController struct {
	mu      sync.RWMutex
	enabled bool
	db      domain.AppStateRepository
	log     logger.LoggerInterface
}

const appStateKeyTradingEnabled = "trading_enabled"

// newTradingController creates a TradingController and restores prior state from db.
func newTradingController(db domain.AppStateRepository, log logger.LoggerInterface) (*TradingController, error) {
	tc := &TradingController{db: db, log: log}

	// Restore persisted state or initialize for first run.
	// GetAppState returns ("", nil) when the key has never been saved.
	val, err := db.GetAppState(appStateKeyTradingEnabled)
	if err != nil {
		log.System().WithError(err).Warn("Failed to read trading state at startup")
	} else if val == "" {
		// Key not yet persisted: save the safe default.
		if saveErr := db.SaveAppState(appStateKeyTradingEnabled, "false"); saveErr != nil {
			log.System().WithError(saveErr).Warn("Failed to initialize trading state at startup")
		}
	} else {
		// Key exists: restore persisted state.
		if val == "true" {
			tc.enabled = true
		}
	}

	return tc, nil
}

// IsTradingEnabled returns whether trading is currently enabled.
func (tc *TradingController) IsTradingEnabled() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.enabled
}

// SetTradingEnabled enables or disables trading and persists the state.
func (tc *TradingController) SetTradingEnabled(enabled bool) error {
	tc.mu.Lock()
	if tc.enabled == enabled {
		tc.mu.Unlock()
		return nil
	}
	tc.enabled = enabled
	tc.mu.Unlock()

	val := "false"
	if enabled {
		val = "true"
	}
	if err := tc.db.SaveAppState(appStateKeyTradingEnabled, val); err != nil {
		tc.log.System().WithError(err).Error("Failed to persist trading state")
		return fmt.Errorf("failed to persist trading state: %w", err)
	}

	if enabled {
		tc.log.System().Info("Trading enabled via API")
	} else {
		tc.log.System().Info("Trading disabled via API")
	}
	return nil
}

// Ensure compile-time interface satisfaction.
var (
	_ interface {
		IsTradingEnabled() bool
		SetTradingEnabled(enabled bool) error
	} = (*TradingController)(nil)
)

// tradingControllerAppServiceAdapter wraps TradingController to satisfy
// adapter/http.ApplicationService (which still uses the old SetTradingEnabled(bool) error signature).
type tradingControllerAppServiceAdapter struct {
	tc                 *TradingController
	getBalance         func(ctx context.Context) ([]domain.Balance, error)
	getCurrentStrategy func() interface{}
}

// errNotImplemented is returned for optional app-service methods not yet wired.
var errNotImplemented = fmt.Errorf("not implemented in bootstrap")
