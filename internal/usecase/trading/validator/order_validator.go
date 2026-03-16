package validator

import (
	"fmt"
	"math"
	"strings"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
)

const (
	// Minimum order sizes for each symbol
	minOrderSizeBTC     = 0.001
	minOrderSizeETH     = 0.01
	minOrderSizeXRP     = 1.0
	minOrderSizeXLM     = 10.0
	minOrderSizeMONA    = 0.1
	minOrderSizeBCH     = 0.001
	minOrderSizeDefault = 0.001
)

// MarketSpecService provides market specification information
type MarketSpecService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}

// OrderValidator validates order requests
type OrderValidator struct {
	marketSpecSvc MarketSpecService
	logger        logger.LoggerInterface
}

// NewOrderValidator creates a new OrderValidator
func NewOrderValidator(marketSpecSvc MarketSpecService, logger logger.LoggerInterface) *OrderValidator {
	return &OrderValidator{
		marketSpecSvc: marketSpecSvc,
		logger:        logger,
	}
}

// ValidateOrder validates the order request
func (v *OrderValidator) ValidateOrder(order *domain.OrderRequest) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if order.Side != "BUY" && order.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}

	// Check for NaN or Inf in numeric fields
	if math.IsNaN(order.Size) || math.IsInf(order.Size, 0) {
		return fmt.Errorf("invalid order size: NaN or infinite")
	}
	if math.IsNaN(order.Price) || math.IsInf(order.Price, 0) {
		return fmt.Errorf("invalid order price: NaN or infinite")
	}

	if order.Size <= 0 {
		// Provide helpful error message with minimum size information
		minSize := v.GetMinimumOrderSize(order.Symbol)
		var estimatedCost float64
		if order.Price > 0 {
			estimatedCost = minSize * order.Price
		}

		return fmt.Errorf(
			"❌ Order size must be positive (got %.8f)\n"+
				"   Symbol: %s\n"+
				"   Minimum order size: %.8f %s\n"+
				"   Estimated minimum cost: ~%.0f JPY\n"+
				"   \n"+
				"   💡 Solutions:\n"+
				"   1. Increase min_notional in config.yaml to at least %.0f JPY\n"+
				"   2. Use XRP_JPY (min ~100 JPY) or MONA_JPY (min ~20 JPY) for smaller trades\n"+
				"   3. Ensure you have sufficient balance",
			order.Size,
			order.Symbol,
			minSize,
			getCurrencyFromSymbol(order.Symbol),
			estimatedCost,
			estimatedCost*1.1,
		)
	}
	if order.Type == "LIMIT" && order.Price <= 0 {
		return fmt.Errorf("price must be positive for LIMIT orders")
	}
	return nil
}

// GetMinimumOrderSize returns the minimum order size for a symbol
func (v *OrderValidator) GetMinimumOrderSize(symbol string) float64 {
	// Try to use market specification service if available
	if v.marketSpecSvc != nil {
		minSize, err := v.marketSpecSvc.GetMinimumOrderSize(symbol)
		if err == nil {
			return minSize
		}
		// Log warning but continue with fallback
		v.logger.Trading().WithField("symbol", symbol).WithError(err).Warn("Failed to get minimum order size from service, using fallback")
	}

	// Fallback to hardcoded values if service not available
	switch symbol {
	case "BTC_JPY":
		return minOrderSizeBTC
	case "ETH_JPY":
		return minOrderSizeETH
	case "XRP_JPY":
		return minOrderSizeXRP
	case "XLM_JPY":
		return minOrderSizeXLM
	case "MONA_JPY":
		return minOrderSizeMONA
	case "BCH_JPY":
		return minOrderSizeBCH
	default:
		return minOrderSizeDefault
	}
}

// CheckBalance checks if balance is sufficient for the order
func (v *OrderValidator) CheckBalance(order *domain.OrderRequest, balances []domain.Balance, feeRate float64) error {
	// Build balance lookup map once for O(1) access instead of O(n) linear search
	balanceMap := make(map[string]*domain.Balance, len(balances))
	for i := range balances {
		balanceMap[balances[i].Currency] = &balances[i]
	}

	if order.Side == "BUY" {
		// Buy order: check JPY balance including fees
		jpyBalance := balanceMap["JPY"]
		if jpyBalance == nil {
			return fmt.Errorf("JPY balance not found")
		}

		// For MARKET orders, we cannot calculate exact required amount
		// since price is not known. Skip detailed balance check and rely on API validation.
		if order.Type == "MARKET" {
			// Just check that we have some JPY balance
			if jpyBalance.Available <= 0 {
				return fmt.Errorf("no JPY balance available")
			}
			v.logger.Trading().WithField("available_jpy", jpyBalance.Available).
				WithField("order_size", order.Size).
				Info("MARKET buy order: balance check skipped (price unknown)")
			return nil
		}

		// For LIMIT orders, calculate required amount including estimated fee
		notional := order.Size * order.Price
		requiredAmount := notional * (1 + feeRate)

		if jpyBalance.Available < requiredAmount {
			return fmt.Errorf("%w: required %f (including fee), available %f JPY",
				domain.ErrInsufficientBalance, requiredAmount, jpyBalance.Available)
		}
	} else {
		// Sell order: check currency balance
		currency := getCurrencyFromSymbol(order.Symbol)

		currencyBalance := balanceMap[currency]
		if currencyBalance == nil {
			return fmt.Errorf("%s balance not found", currency)
		}

		if currencyBalance.Available < order.Size {
			return fmt.Errorf("%w: required %f, available %f %s",
				domain.ErrInsufficientBalance, order.Size, currencyBalance.Available, currency)
		}
	}

	return nil
}

// getCurrencyFromSymbol extracts the base currency from a symbol
// Example: BTC_JPY -> BTC, MONA_JPY -> MONA
func getCurrencyFromSymbol(symbol string) string {
	if idx := strings.Index(symbol, "_"); idx > 0 {
		return symbol[:idx]
	}
	return symbol
}
