package risk

import "time"

// ManagerConfig holds all configuration for the risk Manager.
// This is a usecase-layer struct, free from infrastructure/config dependency.
type ManagerConfig struct {
	// Risk management parameters (from RiskManagementConfig)
	MaxTotalLossPercent   float64
	MaxTradeLossPercent   float64
	MaxDailyLossPercent   float64
	MaxTradeAmountPercent float64
	MaxDailyTrades        int
	MinTradeInterval      time.Duration

	// Trading parameters (from TradingConfig)
	FeeRate        float64
	InitialBalance float64
}
