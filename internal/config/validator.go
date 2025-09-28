package config

import (
	"fmt"

	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// Validator validates configuration settings
type Validator struct {
	logger *logger.Logger
}

// NewValidator creates a new configuration validator
func NewValidator(logger *logger.Logger) *Validator {
	return &Validator{
		logger: logger,
	}
}

// ValidateTradingConfig validates the entire trading configuration
func (v *Validator) ValidateTradingConfig(cfg *Config) error {
	// Get minimum order sizes for configured symbols
	for _, symbol := range cfg.Trading.Symbols {
		if err := v.ValidateSymbol(cfg, symbol); err != nil {
			return err
		}
	}

	return nil
}

// ValidateSymbol validates a specific trading symbol configuration
func (v *Validator) ValidateSymbol(cfg *Config, symbol string) error {
	minSize := getMinimumOrderSize(symbol)
	if minSize == 0 {
		if v.logger != nil {
			v.logger.System().WithField("symbol", symbol).Warn("Unknown symbol, skipping validation")
		}
		return nil
	}

	// Estimate current price (use rough estimates for validation)
	var estimatedPrice float64
	switch symbol {
	case "BTC_JPY":
		estimatedPrice = 4000000 // 4M JPY/BTC
	case "ETH_JPY":
		estimatedPrice = 500000 // 500K JPY/ETH
	case "XRP_JPY":
		estimatedPrice = 100 // 100 JPY/XRP
	case "XLM_JPY":
		estimatedPrice = 50 // 50 JPY/XLM
	case "MONA_JPY":
		estimatedPrice = 200 // 200 JPY/MONA
	case "BCH_JPY":
		estimatedPrice = 50000 // 50K JPY/BCH
	default:
		if v.logger != nil {
			v.logger.System().WithField("symbol", symbol).Warn("No price estimate available, skipping validation")
		}
		return nil
	}

	// Calculate minimum notional required
	minNotional := minSize * estimatedPrice

	// Get min_notional from strategy params
	strategyParams, err := cfg.GetStrategyParams("scalping")
	if err != nil {
		if v.logger != nil {
			v.logger.System().Warn("Failed to get strategy params, using default 200 JPY")
		}
		return nil
	}

	scalpingParams, ok := strategyParams.(ScalpingParams)
	if !ok {
		if v.logger != nil {
			v.logger.System().Warn("Invalid scalping params type, using default 200 JPY")
		}
		return nil
	}

	configMinNotionalFloat := scalpingParams.MinNotional
	if configMinNotionalFloat == 0 {
		configMinNotionalFloat = 200
	}

	if configMinNotionalFloat < minNotional {
		return fmt.Errorf(
			"⚠️  CONFIGURATION ERROR for %s:\n"+
				"   Minimum order size: %.8f %s\n"+
				"   Estimated minimum cost: %.0f JPY (at ~%.0f JPY per unit)\n"+
				"   Your config min_notional: %.0f JPY\n"+
				"   \n"+
				"   ❌ Your orders will be REJECTED by the exchange!\n"+
				"   \n"+
				"   Solutions:\n"+
				"   1. Increase min_notional to at least %.0f JPY in config.yaml\n"+
				"   2. Use a cheaper symbol like XRP_JPY (min ~100 JPY) or XLM_JPY (min ~500 JPY)\n"+
				"   3. Remove %s from trading.symbols if you don't have enough capital",
			symbol,
			minSize,
			extractCurrency(symbol),
			minNotional,
			estimatedPrice,
			configMinNotionalFloat,
			minNotional*1.1, // Add 10% buffer
			symbol,
		)
	}

	if v.logger != nil {
		v.logger.System().
			WithField("symbol", symbol).
			WithField("min_size", minSize).
			WithField("min_notional", minNotional).
			WithField("config_min", configMinNotionalFloat).
			WithField("validation", "PASSED").
			Info("✓ Trading configuration validated for symbol")
	}

	return nil
}

// ValidateRiskManagement validates risk management settings
func (v *Validator) ValidateRiskManagement(rm *RiskManagementConfig) error {
	if rm.MaxTradeAmountPercent <= 0 || rm.MaxTradeAmountPercent > 100 {
		return fmt.Errorf("max_trade_amount_percent must be between 0 and 100, got %.2f", rm.MaxTradeAmountPercent)
	}

	if rm.MaxTotalLossPercent <= 0 || rm.MaxTotalLossPercent > 100 {
		return fmt.Errorf("max_total_loss_percent must be between 0 and 100, got %.2f", rm.MaxTotalLossPercent)
	}

	if rm.MaxDailyTrades <= 0 {
		return fmt.Errorf("max_daily_trades must be positive, got %d", rm.MaxDailyTrades)
	}

	return nil
}

// extractCurrency extracts currency from symbol (e.g., "BTC_JPY" -> "BTC")
func extractCurrency(symbol string) string {
	for i := len(symbol) - 1; i >= 0; i-- {
		if symbol[i] == '_' {
			return symbol[:i]
		}
	}
	return symbol
}

// getMinimumOrderSize returns minimum order size for a trading symbol
func getMinimumOrderSize(symbol string) float64 {
	minimumSizes := map[string]float64{
		"BTC_JPY":  0.001,
		"ETH_JPY":  0.01,
		"XRP_JPY":  1.0,
		"XLM_JPY":  10.0,
		"MONA_JPY": 1.0,
		"BCH_JPY":  0.01,
	}
	return minimumSizes[symbol]
}
