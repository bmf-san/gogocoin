package api

import (
	"context"
	"time"
)

// GetHealth implements StrictServerInterface - health check
func (s *Server) GetHealth(ctx context.Context, request GetHealthRequestObject) (GetHealthResponseObject, error) {
	now := time.Now()
	uptime := s.calculateUptime()
	status := "ok"
	version := s.serverConfig.Version
	return GetHealth200JSONResponse{
		Status:    &status,
		Timestamp: &now,
		Uptime:    &uptime,
		Version:   &version,
	}, nil
}

// GetApiStatus implements StrictServerInterface - get system status
func (s *Server) GetApiStatus(ctx context.Context, request GetApiStatusRequestObject) (GetApiStatusResponseObject, error) {
	const (
		APIKeyPlaceholder    = "${BITFLYER_API_KEY}"
		APISecretPlaceholder = "${BITFLYER_API_SECRET}"
	)

	cfg := s.getConfig()
	hasAPIKey := cfg.API.Credentials.APIKey != "" && cfg.API.Credentials.APIKey != APIKeyPlaceholder
	hasAPISecret := cfg.API.Credentials.APISecret != "" && cfg.API.Credentials.APISecret != APISecretPlaceholder

	var statusDisplay string
	if hasAPIKey && hasAPISecret {
		statusDisplay = "trading"
	} else {
		statusDisplay = "trading (insufficient credentials)"
	}

	uptime := s.calculateUptime()

	totalTrades, err := s.getTodayTradesCount()
	if err != nil {
		s.logger.Error("Failed to get today's trades count: " + err.Error())
		totalTrades = 0
	}

	var tradingEnabled bool
	if s.app != nil {
		tradingEnabled = s.app.IsTradingEnabled()
	}

	runningStatus := "running"
	strategyName := cfg.Trading.Strategy.Name
	now := time.Now()
	symbols := cfg.Trading.Symbols

	resp := StatusResponse{
		Status:            &runningStatus,
		CredentialsStatus: &statusDisplay,
		Strategy:          &strategyName,
		LastUpdate:        &now,
		Uptime:            &uptime,
		TotalTrades:       &totalTrades,
		TradingEnabled:    &tradingEnabled,
		MonitoringSymbols: &symbols,
	}

	monitoringPrices := make(map[string]float32)
	latestMarketData, err := s.db.GetLatestMarketDataForSymbols(cfg.Trading.Symbols)
	if err == nil {
		for symbol := range latestMarketData {
			monitoringPrices[symbol] = float32(latestMarketData[symbol].Close)
		}
	}
	if len(monitoringPrices) > 0 {
		resp.MonitoringPrices = &monitoringPrices
	}

	primarySymbol := ""
	if request.Params.Symbol != nil {
		primarySymbol = *request.Params.Symbol
	}
	if primarySymbol == "" {
		if len(cfg.Trading.Symbols) > 0 {
			primarySymbol = cfg.Trading.Symbols[0]
		} else {
			primarySymbol = "BTC_JPY"
		}
	}
	if price, ok := monitoringPrices[primarySymbol]; ok {
		resp.CurrentPrice = &price
	}

	currentSignal := "N/A"
	resp.CurrentSignal = &currentSignal

	return GetApiStatus200JSONResponse(resp), nil
}
