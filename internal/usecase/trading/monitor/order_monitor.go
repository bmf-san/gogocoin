package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/logger"
)

const (
	// OrderMonitoringInterval is the interval for checking order status
	// Kept at 15s to avoid flooding the rate limiter when many orders are concurrent
	OrderMonitoringInterval = 15 * time.Second
	// OrderMonitoringTimeout is the maximum time to wait for order completion
	OrderMonitoringTimeout = 90 * time.Second
	// orderCallTimeout is the per-API-call timeout within the monitoring loop
	orderCallTimeout = 20 * time.Second
)

// OrderMonitor monitors order execution and status
type OrderMonitor struct {
	logger           logger.LoggerInterface
	orderGetter      OrderGetter
	balanceUpdater   BalanceUpdater
	pnlCalculator    PnLCalculator
	onOrderCompleted func(*domain.OrderResult)
	monitorInterval  time.Duration
	monitorTimeout   time.Duration
}

// OrderGetter defines the interface for getting orders
type OrderGetter interface {
	GetOrders(ctx context.Context) ([]*domain.OrderResult, error)
}

// BalanceUpdater defines the interface for updating balance
type BalanceUpdater interface {
	UpdateBalanceToDB(ctx context.Context)
	InvalidateBalanceCache()
}

// PnLCalculator defines the interface for calculating PnL
type PnLCalculator interface {
	CalculateAndSave(result *domain.OrderResult) (float64, error)
}

// NewOrderMonitor creates a new order monitor
func NewOrderMonitor(
	logger logger.LoggerInterface,
	orderGetter OrderGetter,
	balanceUpdater BalanceUpdater,
	pnlCalculator PnLCalculator,
) *OrderMonitor {
	return &OrderMonitor{
		logger:          logger,
		orderGetter:     orderGetter,
		balanceUpdater:  balanceUpdater,
		pnlCalculator:   pnlCalculator,
		monitorInterval: OrderMonitoringInterval,
		monitorTimeout:  OrderMonitoringTimeout,
	}
}

// SetOnOrderCompleted sets the callback function for completed orders
func (om *OrderMonitor) SetOnOrderCompleted(fn func(*domain.OrderResult)) {
	om.onOrderCompleted = fn
}

// MonitorExecution monitors order execution until completion or timeout.
// It operates on a local copy of result to avoid data races with the caller.
func (om *OrderMonitor) MonitorExecution(ctx context.Context, result *domain.OrderResult) {
	// Copy the caller's result so all internal mutations are isolated and race-free.
	local := *result
	// Write the final state back to the caller's result when we return so that
	// callers can inspect the outcome after MonitorExecution finishes.
	defer func() { *result = local }()

	ticker := time.NewTicker(om.monitorInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(om.monitorTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			if om.logger != nil {
				om.logger.Trading().WithField("order_id", local.OrderID).Info("Order monitoring canceled by context")
			}
			// Try to get final order status before returning
			if err := om.saveFinalOrderState(ctx, &local); err != nil && om.logger != nil {
				om.logger.Trading().WithError(err).WithField("order_id", local.OrderID).
					Error("Failed to save final order state after context cancellation")
			}
			return
		case <-timeout.C:
			if om.logger != nil {
				om.logger.Trading().WithField("order_id", local.OrderID).Warn("Order monitoring timeout - checking final status")
			}
			// Check and save final order status before timeout
			if err := om.saveFinalOrderState(ctx, &local); err != nil && om.logger != nil {
				om.logger.Trading().WithError(err).WithField("order_id", local.OrderID).
					Error("Failed to save final order state after timeout")
			}
			return
		case <-ticker.C:
			if om.checkOrderStatus(ctx, &local) {
				return
			}
		}
	}
}

// saveFinalOrderState checks and saves the final order state (for timeout/cancel scenarios)
// Returns error if database save fails
func (om *OrderMonitor) saveFinalOrderState(_ context.Context, result *domain.OrderResult) error {
	// Use a fresh short-lived context: the parent ctx may already be expired/canceled
	finalCtx, cancel := context.WithTimeout(context.Background(), orderCallTimeout)
	defer cancel()
	// Try to get current order status one last time
	orders, err := om.orderGetter.GetOrders(finalCtx)
	if err != nil {
		if om.logger != nil {
			om.logger.Trading().WithField("order_id", result.OrderID).
				WithError(err).Warn("Failed to get final order status - saving partial state")
		}
		// Save whatever we have (even if order fetch failed)
		return om.saveTradeToDB(result)
	}

	// Find and update order status
	found := false
	for _, order := range orders {
		if order.OrderID == result.OrderID {
			result.Status = order.Status
			result.FilledSize = order.FilledSize
			result.RemainingSize = order.RemainingSize
			result.AveragePrice = order.AveragePrice
			result.TotalCommission = order.TotalCommission
			result.Fee = order.Fee
			result.UpdatedAt = time.Now()

			if om.logger != nil {
				om.logger.Trading().WithField("order_id", result.OrderID).
					WithField("status", result.Status).
					WithField("filled_size", result.FilledSize).
					Info("Retrieved final order status")
			}
			found = true
			break
		}
	}

	// If order not found, do not save incomplete data to prevent data inconsistency
	if !found {
		if om.logger != nil {
			om.logger.Trading().WithField("order_id", result.OrderID).
				Error("Order not found in final status check - possible cache inconsistency, not saving to prevent incomplete data")
		}
		// Return error to indicate save was skipped
		return fmt.Errorf("order %s not found in final status check", result.OrderID)
	}

	// Save to database only if order status was successfully retrieved
	return om.saveTradeToDB(result)
}

// checkOrderStatus checks order status and returns true if completed
func (om *OrderMonitor) checkOrderStatus(ctx context.Context, result *domain.OrderResult) bool {
	// Use a per-call context so a rate-limit delay doesn't burn the overall monitoring budget
	callCtx, cancel := context.WithTimeout(ctx, orderCallTimeout)
	defer cancel()
	orders, err := om.orderGetter.GetOrders(callCtx)
	if err != nil {
		// Check if the error is an authentication error using proper error type checking
		var apiErr *bitflyer.APIError
		if errors.As(err, &apiErr) && apiErr.IsAuthenticationError() {
			if om.logger != nil {
				om.logger.Trading().WithField("order_id", result.OrderID).
					WithField("error_code", apiErr.Code).
					WithField("status_code", apiErr.StatusCode).
					Error("CRITICAL: Authentication failed during order monitoring - order status unknown, requires manual verification")
			}

			// Mark as UNKNOWN status and save to database for manual review
			result.Status = "UNKNOWN_AUTH_ERROR"
			result.UpdatedAt = time.Now()

			// Save the trade with unknown status for audit trail
			if err := om.saveTradeToDB(result); err != nil && om.logger != nil {
				om.logger.Trading().WithError(err).WithField("order_id", result.OrderID).
					Error("Failed to save trade with unknown auth status")
			}

			// DO NOT call completion callback - order status is uncertain
			return true // Stop monitoring (cannot proceed without auth)
		}

		// For other errors, log and continue monitoring
		if om.logger != nil {
			om.logger.Trading().WithError(err).Error("Failed to get order status")
		}
		return false
	}

	for _, order := range orders {
		if order.OrderID == result.OrderID {
			result.Status = order.Status
			result.FilledSize = order.FilledSize
			result.RemainingSize = order.RemainingSize
			result.AveragePrice = order.AveragePrice
			result.TotalCommission = order.TotalCommission
			result.Fee = order.Fee
			result.UpdatedAt = time.Now()

			// isTerminalStatus returns true for statuses where further polling
			// would be pointless (order is no longer active).
			switch order.Status {
			case "COMPLETED":
				if om.logger != nil {
					om.logger.Trading().WithField("order_id", result.OrderID).Info("Order completed")
				}

				// Save trade to database first (before balance update)
				if err := om.saveTradeToDB(result); err != nil && om.logger != nil {
					om.logger.Trading().WithError(err).WithField("order_id", result.OrderID).
						Error("Failed to save completed trade to database")
				}

				// Invalidate balance cache to ensure next balance check gets fresh data
				// reflecting the completed trade
				if om.balanceUpdater != nil {
					om.balanceUpdater.InvalidateBalanceCache()
				}

				// Update balance to database with fresh data
				if om.balanceUpdater != nil {
					om.balanceUpdater.UpdateBalanceToDB(ctx)
				}

				// Call completion callback
				if om.onOrderCompleted != nil {
					om.onOrderCompleted(result)
				}

				return true

			case "CANCELED", "EXPIRED", "REJECTED":
				if om.logger != nil {
					om.logger.Trading().
						WithField("order_id", result.OrderID).
						WithField("status", order.Status).
						Warn("Order reached terminal status without completing - saving and stopping monitor")
				}

				// Save the terminal-status trade (no balance update needed)
				if err := om.saveTradeToDB(result); err != nil && om.logger != nil {
					om.logger.Trading().WithError(err).WithField("order_id", result.OrderID).
						Error("Failed to save terminal-status trade to database")
				}

				return true
			}
		}
	}

	return false
}

// saveTradeToDB saves trade to database with PnL calculation
func (om *OrderMonitor) saveTradeToDB(result *domain.OrderResult) error {
	if om.pnlCalculator != nil {
		if _, err := om.pnlCalculator.CalculateAndSave(result); err != nil {
			return fmt.Errorf("failed to calculate and save trade: %w", err)
		}
	}
	return nil
}
