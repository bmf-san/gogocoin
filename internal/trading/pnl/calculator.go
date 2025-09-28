package pnl

import (
	"fmt"

	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

const (
	// DefaultTradeLimit is the default limit for recent trades lookup
	DefaultTradeLimit = 100
)

// Calculator calculates profit and loss for trades
type Calculator struct {
	db           domain.TradingRepository
	logger       *logger.Logger
	strategyName string
}

// NewCalculator creates a new PnL calculator
func NewCalculator(db domain.TradingRepository, logger *logger.Logger, strategyName string) *Calculator {
	return &Calculator{
		db:           db,
		logger:       logger,
		strategyName: strategyName,
	}
}

// SetStrategyName sets the strategy name for trades
func (pc *Calculator) SetStrategyName(name string) {
	pc.strategyName = name
}

// CalculateAndSave calculates PnL and saves trade to database.
// Uses transaction to ensure atomicity of position and trade saves.
// Returns an error if the trade could not be persisted.
func (pc *Calculator) CalculateAndSave(result *domain.OrderResult) (float64, error) {
	if pc.db == nil {
		return 0, nil
	}

	strategyName := pc.strategyName
	if strategyName == "" {
		strategyName = "unknown"
	}

	// For SELL: read open positions before transaction (read-only, safe outside tx).
	// The computed positions-to-update are then applied atomically inside the transaction
	// together with the trade save, ensuring no partial-write inconsistency.
	pnl := 0.0
	var positionsToUpdate []*domain.Position

	if result.Side == "BUY" {
		// For buys, PnL is just the negative of the fee (cost to enter position)
		pnl = -result.Fee
	} else if result.Side == "SELL" {
		// Compute PnL and collect positions that need updating (no writes yet)
		var sellErr error
		pnl, positionsToUpdate, sellErr = pc.prepareSellData(result)
		if sellErr != nil {
			return 0, fmt.Errorf("failed to prepare sell data: %w", sellErr)
		}
	}

	trade := &domain.Trade{
		Symbol:       result.Symbol,
		ProductCode:  result.Symbol, // always same as symbol for bitFlyer
		Side:         result.Side,
		Type:         result.Type,
		Amount:       result.FilledSize, // filled quantity (consistent with non-tx SaveTrade)
		Size:         result.FilledSize,
		Price:        result.AveragePrice,
		Status:       result.Status,
		Fee:          result.Fee,
		ExecutedAt:   result.UpdatedAt,
		CreatedAt:    result.CreatedAt,
		OrderID:      result.OrderID,
		StrategyName: strategyName,
		PnL:          pnl,
	}

	// Use WithTransaction helper for automatic rollback/commit management.
	// BUY: SavePosition + SaveTrade are atomic.
	// SELL: UpdatePosition(s) + SaveTrade are atomic — if SaveTrade fails the
	//       position rows are NOT modified, so the DB stays consistent.
	err := database.WithTransaction(pc.db, pc.logger, func(tx domain.Transaction) error {

		if result.Side == "BUY" {
			// For BUY orders, create a new open position
			position := &domain.Position{
				Symbol:        result.Symbol,
				Side:          "BUY",
				Size:          result.FilledSize,
				UsedSize:      0,
				RemainingSize: result.FilledSize,
				EntryPrice:    result.AveragePrice,
				CurrentPrice:  result.AveragePrice,
				Status:        "OPEN",
				OrderID:       result.OrderID,
				CreatedAt:     result.CreatedAt,
				UpdatedAt:     result.UpdatedAt,
			}
			if err := tx.SavePosition(position); err != nil {
				return fmt.Errorf("failed to save position: %w", err)
			}
		}

		// For SELL: apply the pre-computed position updates inside the transaction
		for _, pos := range positionsToUpdate {
			if err := tx.UpdatePosition(pos); err != nil {
				return fmt.Errorf("failed to update position %s: %w", pos.OrderID, err)
			}
		}

		if err := tx.SaveTrade(trade); err != nil {
			return fmt.Errorf("failed to save trade: %w", err)
		}

		return nil
	})

	if err != nil {
		if pc.logger != nil {
			pc.logger.Trading().WithError(err).Error("Transaction failed, falling back to non-transactional save")
		}
		// Fallback to non-transactional save
		return pc.calculateAndSaveWithoutTx(result, strategyName)
	}

	return pnl, nil
}

// calculateAndSaveWithoutTx is a fallback method for non-transactional save.
// Returns the calculated PnL and any error that prevented the trade from being persisted.
func (pc *Calculator) calculateAndSaveWithoutTx(result *domain.OrderResult, strategyName string) (float64, error) {
	pnl := 0.0

	if result.Side == "BUY" {
		position := &domain.Position{
			Symbol:        result.Symbol,
			Side:          "BUY",
			Size:          result.FilledSize,
			UsedSize:      0,
			RemainingSize: result.FilledSize,
			EntryPrice:    result.AveragePrice,
			CurrentPrice:  result.AveragePrice,
			Status:        "OPEN",
			OrderID:       result.OrderID,
			CreatedAt:     result.CreatedAt,
			UpdatedAt:     result.UpdatedAt,
		}

		if err := pc.db.SavePosition(position); err != nil {
			if pc.logger != nil {
				pc.logger.Trading().WithError(err).Error("Failed to save position to database")
			}
		}

		pnl = -result.Fee
	} else if result.Side == "SELL" {
		var positionsToUpdate []*domain.Position
		var sellErr error
		pnl, positionsToUpdate, sellErr = pc.prepareSellData(result)
		if sellErr == nil {
			for _, pos := range positionsToUpdate {
				if err := pc.db.UpdatePosition(pos); err != nil && pc.logger != nil {
					pc.logger.Trading().WithError(err).WithField("order_id", pos.OrderID).
						Error("Failed to update position (fallback path)")
				}
			}
		}
	}

	trade := &domain.Trade{
		Symbol:       result.Symbol,
		ProductCode:  result.Symbol,
		Side:         result.Side,
		Type:         result.Type,
		Amount:       result.FilledSize,
		Size:         result.FilledSize,
		Price:        result.AveragePrice,
		Status:       result.Status,
		Fee:          result.Fee,
		ExecutedAt:   result.UpdatedAt,
		CreatedAt:    result.CreatedAt,
		OrderID:      result.OrderID,
		StrategyName: strategyName,
		PnL:          pnl,
	}

	if err := pc.db.SaveTrade(trade); err != nil {
		if pc.logger != nil {
			pc.logger.Trading().WithError(err).Error("Failed to save trade to database")
		}
		return pnl, fmt.Errorf("failed to save trade: %w", err)
	}

	return pnl, nil
}

// prepareSellData calculates PnL for a SELL order using FIFO position matching.
// It returns the computed PnL and the slice of positions that need to be updated,
// but does NOT write anything to the database — callers must apply the updates
// inside a transaction to ensure atomicity with the trade record save.
func (pc *Calculator) prepareSellData(result *domain.OrderResult) (pnl float64, positions []*domain.Position, err error) {
	openPositions, err := pc.db.GetOpenPositions(result.Symbol, "BUY")
	if err != nil {
		if pc.logger != nil {
			pc.logger.Trading().WithError(err).Error("Failed to get open positions for SELL")
		}
		return -result.Fee, nil, fmt.Errorf("failed to get open positions: %w", err)
	}

	remainingSize := result.FilledSize
	totalCost := 0.0
	totalFees := result.Fee

	for i := range openPositions {
		if remainingSize <= 0 {
			break
		}

		position := &openPositions[i]
		matchSize := remainingSize
		if position.RemainingSize < matchSize {
			matchSize = position.RemainingSize
		}

		totalCost += position.EntryPrice * matchSize

		// Mutate the in-memory copy only — no DB writes here
		position.UsedSize += matchSize
		position.RemainingSize -= matchSize
		position.CurrentPrice = result.AveragePrice
		position.UpdateStatus()

		positions = append(positions, position)
		remainingSize -= matchSize
	}

	if totalCost > 0 {
		sellRevenue := result.AveragePrice * result.FilledSize
		return sellRevenue - totalCost - totalFees, positions, nil
	}

	return -result.Fee, positions, nil
}
