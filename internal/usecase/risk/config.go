package risk

import "time"

// ManagerConfig holds all configuration for the risk Manager.
// This is a usecase-layer struct, free from infrastructure/config dependency.
type ManagerConfig struct {
	// Risk management parameters (from RiskManagementConfig)
	MaxTotalLossPercent        float64
	MaxTradeLossPercent        float64
	MaxDailyLossPercent        float64
	MaxTradeAmountPercent      float64
	MaxDailyTrades             int
	MinTradeInterval           time.Duration
	MaxOpenPositionsPerSymbol  int // 0 = unlimited
	// PnLEpoch discards trade history recorded before this instant when the
	// total-loss limit is evaluated. Zero means "use the whole history".
	//
	// The limit is normally read from the stored performance metrics, which
	// aggregate every trade ever recorded. That is only trustworthy while the
	// recorded PnL is trustworthy: a period of mis-calculated PnL keeps
	// distorting the limit forever, in whichever direction the error ran. The
	// epoch draws a line under such a period without deleting the trades.
	PnLEpoch time.Time

	// Trading parameters (from TradingConfig)
	FeeRate        float64
	InitialBalance float64
}
