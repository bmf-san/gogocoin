package pnl

import (
	"fmt"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/persistence"
	"github.com/bmf-san/gogocoin/internal/logger"
)

const (
	// DefaultTradeLimit is the default limit for recent trades lookup
	DefaultTradeLimit = 100
)

// Calculator calculates profit and loss for trades
type Calculator struct {
	db           domain.TradingRepository //nolint:staticcheck // SA1019: migrating to individual repos in Phase 5
	logger       logger.LoggerInterface
	strategyName string
}

// NewCalculator creates a new PnL calculator
func NewCalculator(db domain.TradingRepository, logger logger.LoggerInterface, strategyName string) *Calculator { //nolint:staticcheck // SA1019: migrating to individual repos in Phase 5
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

	// Use FilledSize when available, fall back to requested Size (e.g. for ACTIVE/unfilled orders)
	effectiveSize := result.FilledSize
	if effectiveSize == 0 {
		effectiveSize = result.Size
	}
	// Use AveragePrice when available, fall back to requested Price
	effectivePrice := result.AveragePrice
	if effectivePrice == 0 {
		effectivePrice = result.Price
	}

	switch result.Side {
	case "BUY":
		// For buys, PnL is just the negative of the fee (cost to enter position)
		pnl = -result.Fee
	case "SELL":
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
		Amount:       effectiveSize,
		Size:         effectiveSize,
		Price:        effectivePrice,
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
	err := persistence.WithTransaction(pc.db, pc.logger, func(tx domain.Transaction) error {

		if result.Side == "BUY" {
			// For BUY orders, create a new open position
			position := &domain.Position{
				Symbol:        result.Symbol,
				Side:          "BUY",
				Size:          effectiveSize,
				UsedSize:      0,
				RemainingSize: effectiveSize,
				EntryPrice:    effectivePrice,
				CurrentPrice:  effectivePrice,
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

	// Use FilledSize when available, fall back to requested Size
	effectiveSize := result.FilledSize
	if effectiveSize == 0 {
		effectiveSize = result.Size
	}
	// Use AveragePrice when available, fall back to requested Price
	effectivePrice := result.AveragePrice
	if effectivePrice == 0 {
		effectivePrice = result.Price
	}

	switch result.Side {
	case "BUY":
		position := &domain.Position{
			Symbol:        result.Symbol,
			Side:          "BUY",
			Size:          effectiveSize,
			UsedSize:      0,
			RemainingSize: effectiveSize,
			EntryPrice:    effectivePrice,
			CurrentPrice:  effectivePrice,
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
	case "SELL":
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
		Amount:       effectiveSize,
		Size:         effectiveSize,
		Price:        effectivePrice,
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

	effectivePrice := result.AveragePrice
	if effectivePrice == 0 {
		effectivePrice = result.Price
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
		position.CurrentPrice = effectivePrice
		position.UpdateStatus()

		positions = append(positions, position)
		remainingSize -= matchSize
	}

	if totalCost > 0 {
		sellRevenue := effectivePrice * result.FilledSize
		return sellRevenue - totalCost - totalFees, positions, nil
	}

	return -result.Fee, positions, nil
}
