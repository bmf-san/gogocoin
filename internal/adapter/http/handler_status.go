package api

import (
	"context"
	"time"
)

// GetHealth implements StrictServerInterface - ヘルスチェック
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

// GetApiStatus implements StrictServerInterface - システム状態の取得
func (s *Server) GetApiStatus(ctx context.Context, request GetApiStatusRequestObject) (GetApiStatusResponseObject, error) {
	const (
		APIKeyPlaceholder    = "${BITFLYER_API_KEY}"
		APISecretPlaceholder = "${BITFLYER_API_SECRET}"
	)

	hasAPIKey := s.config.API.Credentials.APIKey != "" && s.config.API.Credentials.APIKey != APIKeyPlaceholder
	hasAPISecret := s.config.API.Credentials.APISecret != "" && s.config.API.Credentials.APISecret != APISecretPlaceholder

	var statusDisplay string
	if hasAPIKey && hasAPISecret {
		statusDisplay = "trading"
	} else {
		statusDisplay = "trading (insufficient credentials)"
	}

	uptime := s.calculateUptime()

	totalTrades, err := s.getTotalTradesCount()
	if err != nil {
		s.logger.Error("Failed to get total trades count: " + err.Error())
		totalTrades = 0
	}

	var tradingEnabled bool
	if s.app != nil {
		tradingEnabled = s.app.IsTradingEnabled()
	}

	runningStatus := "running"
	strategyName := s.config.Trading.Strategy.Name
	now := time.Now()
	symbols := s.config.Trading.Symbols

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
	latestMarketData, err := s.db.GetLatestMarketDataForSymbols(s.config.Trading.Symbols)
	if err == nil {
		for symbol, data := range latestMarketData {
			monitoringPrices[symbol] = float32(data.Close)
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
		if len(s.config.Trading.Symbols) > 0 {
			primarySymbol = s.config.Trading.Symbols[0]
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
