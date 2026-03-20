package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/bmf-san/gogocoin/internal/config"
	"github.com/bmf-san/gogocoin/internal/domain"
)

// customConfigResponse returns the full application config as JSON,
// satisfying GetApiConfigResponseObject.
type customConfigResponse struct {
	cfg *config.Config
}

func (r customConfigResponse) VisitGetApiConfigResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(r.cfg)
}

// customConfigUpdateResponse returns the full application config in the POST /api/config response.
type customConfigUpdateResponse struct {
	cfg     *config.Config
	message string
}

func (r customConfigUpdateResponse) VisitPostApiConfigResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": r.message,
		"config":  r.cfg,
	})
}

// maskedConfig returns a copy of cfg with API credentials replaced by placeholders.
func maskedConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}
	copy := *cfg
	copy.API.Credentials.APIKey = "***"
	copy.API.Credentials.APISecret = "***"
	return &copy
}

// GetApiBalance implements StrictServerInterface - get balance
func (s *Server) GetApiBalance(ctx context.Context, request GetApiBalanceRequestObject) (GetApiBalanceResponseObject, error) {
	if s.app != nil {
		balances, err := s.app.GetBalances(ctx)
		if err != nil {
			s.logger.Error("Failed to get balances from application: " + err.Error())
			cachedBalances, dbErr := s.db.GetLatestBalances()
			if dbErr == nil && len(cachedBalances) > 0 {
				if isRateLimitError(err) {
					s.logger.UI().Info("Rate limit encountered, falling back to cached balance data")
				} else {
					s.logger.UI().Info("API error encountered, falling back to cached balance data")
				}
				s.sortBalances(cachedBalances)
				return GetApiBalance200JSONResponse(domainBalancesToAPI(cachedBalances)), nil
			}
			msg := "Failed to get balances"
			return GetApiBalance500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
		}
		if len(balances) > 0 {
			var domainBalances []domain.Balance
			for _, balance := range balances {
				domainBalances = append(domainBalances, domain.Balance{
					Currency:  balance.Currency,
					Amount:    balance.Amount,
					Available: balance.Available,
				})
			}
			s.sortBalances(domainBalances)
			return GetApiBalance200JSONResponse(domainBalancesToAPI(domainBalances)), nil
		}
		return GetApiBalance200JSONResponse{}, nil
	}

	// Fallback: DB only
	balances, err := s.db.GetLatestBalances()
	if err != nil {
		s.logger.Error("Failed to get balances from database: " + err.Error())
		msg := "Internal server error"
		return GetApiBalance500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
	}
	if len(balances) == 0 {
		balances = s.generateSampleBalances()
	}
	s.sortBalances(balances)
	return GetApiBalance200JSONResponse(domainBalancesToAPI(balances)), nil
}

// GetApiTrades implements StrictServerInterface - get trade history
func (s *Server) GetApiTrades(ctx context.Context, request GetApiTradesRequestObject) (GetApiTradesResponseObject, error) {
	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	trades, err := s.db.GetRecentTrades(limit)
	if err != nil {
		s.logger.Error("Failed to get trades: " + err.Error())
		msg := "Internal server error"
		return GetApiTrades500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
	}

	s.logger.System().WithField("count", len(trades)).Info("Returning trades from API")
	return GetApiTrades200JSONResponse(domainTradesToAPI(trades)), nil
}

// GetApiPerformance implements StrictServerInterface - get performance metrics
func (s *Server) GetApiPerformance(ctx context.Context, request GetApiPerformanceRequestObject) (GetApiPerformanceResponseObject, error) {
	metrics, err := s.db.GetPerformanceMetrics(30)
	if err != nil {
		s.logger.Error("Failed to get performance: " + err.Error())
		msg := "Internal server error"
		return GetApiPerformance500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
	}

	s.logger.System().WithField("count", len(metrics)).Info("Returning performance metrics from API")
	return GetApiPerformance200JSONResponse(domainMetricsToAPI(metrics)), nil
}

// GetApiConfig implements StrictServerInterface - get current config
func (s *Server) GetApiConfig(ctx context.Context, request GetApiConfigRequestObject) (GetApiConfigResponseObject, error) {
	cfg := s.getConfig()
	s.logger.UI().WithField("strategy", cfg.Trading.Strategy.Name).Info("Returning config")
	return customConfigResponse{cfg: maskedConfig(cfg)}, nil
}

// PostApiConfig implements StrictServerInterface - update config
func (s *Server) PostApiConfig(ctx context.Context, request PostApiConfigRequestObject) (PostApiConfigResponseObject, error) {
	if request.Body == nil {
		msg := "Request body is required"
		return PostApiConfig400JSONResponse{BadRequestJSONResponse{Message: &msg}}, nil
	}
	req := request.Body

	// Build a local copy of the current config to mutate and save.
	// This avoids holding the lock during disk I/O and prevents a data race
	// on s.config against concurrent read-only handlers.
	s.configMu.RLock()
	cfgSnapshot := *s.config // shallow copy is safe for value-type fields
	s.configMu.RUnlock()

	if req.Strategy != nil && req.Strategy.Name != nil {
		name := string(*req.Strategy.Name)
		allowedStrategies := []string{"scalping", "simple"}
		if err := validateStringParam(name, allowedStrategies); err != nil {
			msg := fmt.Sprintf("Invalid strategy name: %v", err)
			return PostApiConfig400JSONResponse{BadRequestJSONResponse{Message: &msg}}, nil
		}
		cfgSnapshot.Trading.Strategy.Name = name
	}

	if req.Risk != nil {
		if req.Risk.StopLoss != nil && (*req.Risk.StopLoss < 0 || *req.Risk.StopLoss > 100) {
			msg := "Invalid stop_loss: must be between 0 and 100"
			return PostApiConfig400JSONResponse{BadRequestJSONResponse{Message: &msg}}, nil
		}
		if req.Risk.TakeProfit != nil && (*req.Risk.TakeProfit < 0 || *req.Risk.TakeProfit > 100) {
			msg := "Invalid take_profit: must be between 0 and 100"
			return PostApiConfig400JSONResponse{BadRequestJSONResponse{Message: &msg}}, nil
		}
	}

	s.logger.UI().WithField("strategy", cfgSnapshot.Trading.Strategy.Name).Info("Updating configuration")

	if err := s.saveConfigToFile(&cfgSnapshot); err != nil {
		s.logger.Error("Failed to save config to file: " + err.Error())
		msg := "Failed to save configuration"
		return PostApiConfig500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
	}

	s.logger.UI().Info("Configuration updated successfully")

	configPath := s.serverConfig.ConfigPath
	reloadedConfig, err := config.Load(configPath)
	if err != nil {
		s.logger.Error("Failed to reload config from file: " + err.Error())
	} else {
		s.configMu.Lock()
		s.config = reloadedConfig
		s.configMu.Unlock()
		s.logger.UI().Info("Configuration reloaded from file successfully")
	}

	return customConfigUpdateResponse{cfg: maskedConfig(s.getConfig()), message: "Configuration saved"}, nil
}

// GetApiLogs implements StrictServerInterface - get logs
func (s *Server) GetApiLogs(ctx context.Context, request GetApiLogsRequestObject) (GetApiLogsResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	levelFilter := ""
	if request.Params.Level != nil {
		levelFilter = string(*request.Params.Level)
	}

	categoryFilter := ""
	if request.Params.Category != nil {
		categoryFilter = string(*request.Params.Category)
	}

	logs, err := s.db.GetRecentLogsWithFilters(limit, levelFilter, categoryFilter)
	if err != nil {
		s.logger.Error("Failed to get logs: " + err.Error())
		msg := "Internal server error"
		return GetApiLogs500JSONResponse{InternalServerErrorJSONResponse{Message: &msg}}, nil
	}

	return GetApiLogs200JSONResponse(domainLogsToAPI(logs)), nil
}

// GetApiOrders implements StrictServerInterface - get order history
func (s *Server) GetApiOrders(ctx context.Context, request GetApiOrdersRequestObject) (GetApiOrdersResponseObject, error) {
	// Orders are not stored in DB directly; return empty list
	return GetApiOrders200JSONResponse{}, nil
}

// saveConfigToFile saves configuration to file.
// API credentials are replaced with environment variable placeholders
// so that secrets are never written to disk in plain text.
// cfg is the desired state to persist; it must be a caller-owned copy,
// not the shared s.config pointer (to avoid holding the lock during disk I/O).
func (s *Server) saveConfigToFile(cfg *config.Config) error {
	configPath := s.serverConfig.ConfigPath

	cfgCopy := *cfg
	apiCopy := cfgCopy.API
	apiCopy.Credentials = config.CredentialsConfig{
		APIKey:    "${BITFLYER_API_KEY}",
		APISecret: "${BITFLYER_API_SECRET}",
	}
	cfgCopy.API = apiCopy

	data, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 0600: owner read/write only — config may contain sensitive info
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// domainBalancesToAPI converts []domain.Balance to []Balance (generated API type).
func domainBalancesToAPI(balances []domain.Balance) []Balance {
	result := make([]Balance, len(balances))
	for i, b := range balances {
		id := b.ID
		currency := b.Currency
		amount := float32(b.Amount)
		available := float32(b.Available)
		ts := b.Timestamp
		result[i] = Balance{
			Id:        &id,
			Currency:  &currency,
			Amount:    &amount,
			Available: &available,
			Timestamp: &ts,
		}
	}
	return result
}

// domainTradesToAPI converts []domain.Trade to []Trade (generated API type).
func domainTradesToAPI(trades []domain.Trade) []Trade {
	result := make([]Trade, len(trades))
	for i, t := range trades {
		id := t.ID
		symbol := t.Symbol
		productCode := t.ProductCode
		side := TradeSide(t.Side)
		tradeType := t.Type
		amount := float32(t.Amount)
		size := float32(t.Size)
		price := float32(t.Price)
		fee := float32(t.Fee)
		status := TradeStatus(t.Status)
		orderID := t.OrderID
		strategyName := t.StrategyName
		pnl := float32(t.PnL)
		createdAt := t.CreatedAt
		executedAt := t.ExecutedAt
		updatedAt := t.UpdatedAt
		result[i] = Trade{
			Id:           &id,
			Symbol:       &symbol,
			ProductCode:  &productCode,
			Side:         &side,
			Type:         &tradeType,
			Amount:       &amount,
			Size:         &size,
			Price:        &price,
			Fee:          &fee,
			Status:       &status,
			OrderId:      &orderID,
			StrategyName: &strategyName,
			Pnl:          &pnl,
			CreatedAt:    &createdAt,
			ExecutedAt:   &executedAt,
			UpdatedAt:    &updatedAt,
		}
	}
	return result
}

// domainMetricsToAPI converts []domain.PerformanceMetric to []PerformanceMetric (generated API type).
func domainMetricsToAPI(metrics []domain.PerformanceMetric) []PerformanceMetric {
	result := make([]PerformanceMetric, len(metrics))
	for i, m := range metrics {
		date := m.Date
		totalReturn := float32(m.TotalReturn)
		dailyReturn := float32(m.DailyReturn)
		winRate := float32(m.WinRate)
		maxDrawdown := float32(m.MaxDrawdown)
		sharpeRatio := float32(m.SharpeRatio)
		profitFactor := float32(m.ProfitFactor)
		totalTrades := m.TotalTrades
		winningTrades := m.WinningTrades
		losingTrades := m.LosingTrades
		avgWin := float32(m.AverageWin)
		avgLoss := float32(m.AverageLoss)
		largestWin := float32(m.LargestWin)
		largestLoss := float32(m.LargestLoss)
		consWins := m.ConsecutiveWins
		consLoss := m.ConsecutiveLoss
		totalPnl := float32(m.TotalPnL)
		result[i] = PerformanceMetric{
			Date:            &date,
			TotalReturn:     &totalReturn,
			DailyReturn:     &dailyReturn,
			WinRate:         &winRate,
			MaxDrawdown:     &maxDrawdown,
			SharpeRatio:     &sharpeRatio,
			ProfitFactor:    &profitFactor,
			TotalTrades:     &totalTrades,
			WinningTrades:   &winningTrades,
			LosingTrades:    &losingTrades,
			AverageWin:      &avgWin,
			AverageLoss:     &avgLoss,
			LargestWin:      &largestWin,
			LargestLoss:     &largestLoss,
			ConsecutiveWins: &consWins,
			ConsecutiveLoss: &consLoss,
			TotalPnl:        &totalPnl,
		}
	}
	return result
}

// domainLogsToAPI converts []domain.LogEntry to []LogEntry (generated API type).
func domainLogsToAPI(logs []domain.LogEntry) []LogEntry {
	result := make([]LogEntry, len(logs))
	for i, l := range logs {
		level := LogEntryLevel(l.Level)
		category := LogEntryCategory(l.Category)
		message := l.Message
		ts := l.Timestamp
		result[i] = LogEntry{
			Level:     &level,
			Category:  &category,
			Message:   &message,
			Timestamp: &ts,
		}
		if len(l.Fields) > 0 {
			fields := map[string]interface{}(l.Fields)
			result[i].Fields = &fields
		}
	}
	return result
}
