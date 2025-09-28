package app

// diagnoseTradingSetup diagnoses the trading configuration
func (app *Application) diagnoseTradingSetup() {
	app.logger.System().Info("=== Trading Setup Diagnosis ===")

	// 1. Check trading enabled state (dynamic control)
	if app.IsTradingEnabled() {
		app.logger.System().Info("✓ Trading is ENABLED (dynamic control)")
	} else {
		app.logger.System().Warn("✗ Trading is DISABLED (dynamic control)")
		app.logger.System().Info("  To enable: use Web UI or API /api/trading/start")
	}

	// 2. Check API credentials
	hasAPIKey := app.config.API.Credentials.APIKey != "" && app.config.API.Credentials.APIKey != "${BITFLYER_API_KEY}"
	hasAPISecret := app.config.API.Credentials.APISecret != "" && app.config.API.Credentials.APISecret != "${BITFLYER_API_SECRET}"

	if hasAPIKey && hasAPISecret {
		app.logger.System().Info("✓ API credentials are configured")
	} else {
		app.logger.System().Warn("✗ API credentials are missing or not expanded")
		app.logger.System().Info("  Required environment variables:")
		app.logger.System().Info("    export BITFLYER_API_KEY='your_api_key'")
		app.logger.System().Info("    export BITFLYER_API_SECRET='your_api_secret'")
		app.logger.System().Error("  ⚠️  CRITICAL: Trading requires valid API credentials!")
		app.logger.System().Error("  ⚠️  System may not function correctly without credentials!")
	}

	// 3. Check target symbols
	app.logger.System().WithField("symbols", app.config.Trading.Symbols).Info("Target trading symbols")
	if len(app.config.Trading.Symbols) == 0 {
		app.logger.System().Warn("✗ No trading symbols configured")
	}

	// 4. Check strategy
	app.logger.System().WithField("strategy", app.config.Trading.Strategy.Name).Info("Trading strategy")

	// 5. risk management configuration
	app.logger.System().Info("Risk management settings:")
	app.logger.System().WithField("max_trade_amount_percent", app.config.Trading.RiskManagement.MaxTradeAmountPercent).Info("  Max trade amount per order")
	app.logger.System().WithField("max_total_loss_percent", app.config.Trading.RiskManagement.MaxTotalLossPercent).Info("  Max total loss percent")
	app.logger.System().WithField("min_trade_interval", app.config.Trading.RiskManagement.MinTradeInterval).Info("  Min trade interval")

	// 6. WebSocket connection test
	// Delegate to ConnectionManager
	if app.connManager != nil {
		if app.connManager.IsConnected() {
			app.logger.System().Info("✓ WebSocket connection is active")
		} else {
			app.logger.System().Warn("✗ WebSocket connection is not active")
			app.logger.System().Info("  Market data may not be available")
		}
	}

	// 8. Check strategy initialization state
	if app.serviceRegistry.CurrentStrategy != nil {
		if app.serviceRegistry.CurrentStrategy.IsRunning() {
			app.logger.System().Info("✓ Strategy is initialized and running")
		} else {
			app.logger.System().Warn("✗ Strategy is not running")
		}
	} else {
		app.logger.System().Error("✗ No strategy initialized")
	}

	app.logger.System().Info("=== End Diagnosis ===")
}
