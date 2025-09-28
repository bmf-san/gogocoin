package paper

import (
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
)

// PnLCalculator calculates PnL (Profit and Loss)
type PnLCalculator struct {
	feeRate float64
	logger  *logger.Logger
}

// NewPnLCalculator creates a new PnL calculator
func NewPnLCalculator(feeRate float64, log *logger.Logger) *PnLCalculator {
	if feeRate <= 0 {
		feeRate = 0.0015 // Default: 0.15%
	}

	return &PnLCalculator{
		feeRate: feeRate,
		logger:  log,
	}
}

// CalculateFee calculates fee
func (pc *PnLCalculator) CalculateFee(size, price float64) float64 {
	return size * price * pc.feeRate
}

// CalculatePnL calculates PnL for SELL orders
// References the previous BUY order to calculate profit/loss
func (pc *PnLCalculator) CalculatePnL(sellOrder *domain.OrderResult, db trading.DatabaseSaver) float64 {
	if db == nil {
		return 0
	}

	// Get recent BUY trades with the same symbol
	trades, err := db.GetRecentTrades(100)
	if err != nil {
		pc.logger.Trading().WithError(err).Error("Failed to get recent trades for PnL calculation")
		return 0
	}

	// Find the most recent BUY trade
	var buyPrice float64
	var buyFee float64
	found := false

	for i := range trades {
		trade := &trades[i]
		// Find BUY trade with same symbol (before SELL trade)
		if trade.Symbol == sellOrder.Symbol && trade.Side == "BUY" &&
			trade.ExecutedAt.Before(sellOrder.CreatedAt) {
			buyPrice = trade.Price
			buyFee = trade.Fee
			found = true
			break
		}
	}

	if !found {
		pc.logger.Trading().
			WithField("symbol", sellOrder.Symbol).
			Warn("No matching BUY trade found for PnL calculation")
		return 0
	}

	// Calculate PnL
	// PnL = (sell price - buy price) × quantity - (buy fee + sell fee)
	sellPrice := sellOrder.AveragePrice
	if sellPrice <= 0 {
		sellPrice = sellOrder.Price
	}
	size := sellOrder.FilledSize
	if size <= 0 {
		size = sellOrder.Size
	}
	sellFee := sellOrder.Fee

	priceDiff := (sellPrice - buyPrice) * size
	totalFee := buyFee + sellFee
	pnl := priceDiff - totalFee

	pc.logger.Trading().
		WithField("symbol", sellOrder.Symbol).
		WithField("buy_price", buyPrice).
		WithField("sell_price", sellPrice).
		WithField("size", size).
		WithField("buy_fee", buyFee).
		WithField("sell_fee", sellFee).
		WithField("price_diff", priceDiff).
		WithField("total_fee", totalFee).
		WithField("pnl", pnl).
		Info("Calculated paper trade PnL")

	return pnl
}
