package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// DatabaseService defines the database operations needed by the API server.
// This interface follows the Consumer-Driven Contracts pattern, where each layer
// defines only the methods it needs. This decouples the API layer from the
// concrete database implementation and allows for independent evolution of layers.
type DatabaseService interface {
	// Balance operations
	GetLatestBalances() ([]domain.Balance, error)

	// Trade operations
	GetRecentTrades(limit int) ([]domain.Trade, error)
	GetTradesCount() (int, error)

	// Position operations
	GetActivePositions() ([]domain.Position, error)
	GetActivePositionsCount() (int, error)

	// Market data operations
	GetLatestMarketData(symbol string, limit int) ([]domain.MarketData, error)
	GetLatestMarketDataForSymbols(symbols []string) (map[string]domain.MarketData, error)

	// Performance metrics
	GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error)

	// Log operations
	GetRecentLogsWithFilters(limit int, level, category string) ([]domain.LogEntry, error)
}

// Balance represents balance for API
type Balance struct {
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
	Available float64 `json:"available"`
}

// ApplicationService is the application service interface
type ApplicationService interface {
	GetBalances(ctx context.Context) ([]domain.Balance, error)
	GetCurrentStrategy() strategy.Strategy
	IsTradingEnabled() bool
	SetTradingEnabled(enabled bool) error
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	ConfigPath string
	WebRoot    string
	Version    string
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ConfigPath: "configs/config.yaml",
		WebRoot:    "web",
		Version:    "v1.0.0",
	}
}

// Server is the API server for Web UI
type Server struct {
	config       *config.Config
	db           DatabaseService
	logger       *logger.Logger
	host         string
	port         int
	serverConfig *ServerConfig
	startTime    time.Time
	app          ApplicationService
	httpServer   *http.Server
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, db DatabaseService, logger *logger.Logger) *Server {
	return NewServerWithConfig(cfg, db, logger, DefaultServerConfig())
}

// NewServerWithConfig creates a new API server with custom server config
func NewServerWithConfig(cfg *config.Config, db DatabaseService, logger *logger.Logger, serverCfg *ServerConfig) *Server {
	// Get port from configuration, default to 8080
	port := cfg.UI.Port
	if port == 0 {
		port = 8080
	}

	// Get host from configuration, default to localhost
	host := cfg.UI.Host
	if host == "" {
		host = "localhost"
	}

	return &Server{
		config:       cfg,
		db:           db,
		logger:       logger,
		host:         host,
		port:         port,
		serverConfig: serverCfg,
		startTime:    time.Now(),
		app:          nil, // Set via SetApplication()
	}
}

// SetApplication sets the application service
func (s *Server) SetApplication(app ApplicationService) {
	s.app = app
}

// Start starts the API server
func (s *Server) Start() error {
	// Create new ServeMux
	mux := http.NewServeMux()

	// API endpoints (register first)
	mux.HandleFunc("/health", s.handleHealth) // Health check endpoint for monitoring
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/balance", s.handleBalance)
	// Positions endpoint removed - not used in spot trading simulation
	mux.HandleFunc("/api/trades", s.handleTrades)
	mux.HandleFunc("/api/orders", s.handleOrders)
	mux.HandleFunc("/api/performance", s.handlePerformance)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/strategy/reset", s.handleStrategyReset)
	mux.HandleFunc("/api/trading/start", s.handleTradingStart)
	mux.HandleFunc("/api/trading/stop", s.handleTradingStop)
	mux.HandleFunc("/api/trading/status", s.handleTradingStatus)

	// Serve static files (non-API paths)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 for API paths
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Serve static files
		http.FileServer(http.Dir(s.serverConfig.WebRoot)).ServeHTTP(w, r)
	})

	s.logger.Info(fmt.Sprintf("Starting web server on %s:%d", s.host, s.port))

	// Configure timeout for security
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the API server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.System().Info("Shutting down API server gracefully...")

	// Use http.Server.Shutdown for graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.System().WithError(err).Error("API server shutdown error")
		return fmt.Errorf("failed to shutdown API server: %w", err)
	}

	s.logger.System().Info("API server shut down successfully")
	return nil
}

// StatusResponse represents status response
type StatusResponse struct {
	Status            string             `json:"status"`
	CredentialsStatus string             `json:"credentials_status"`
	Strategy          string             `json:"strategy"`
	LastUpdate        time.Time          `json:"last_update"`
	Uptime            string             `json:"uptime"`
	TotalTrades       int                `json:"total_trades"`
	ActiveOrders      int                `json:"active_orders"`
	TradingEnabled    bool               `json:"trading_enabled"`
	MonitoringSymbols []string           `json:"monitoring_symbols"`
	MonitoringPrices  map[string]float64 `json:"monitoring_prices,omitempty"`
	CurrentPrice      float64            `json:"current_price,omitempty"`
	PriceChange       float64            `json:"price_change,omitempty"`
	CurrentSignal     string             `json:"current_signal,omitempty"`
	CooldownRemaining int                `json:"cooldown_remaining,omitempty"`
	EMAFast           float64            `json:"ema_fast,omitempty"`
	EMASlow           float64            `json:"ema_slow,omitempty"`
}

// TradingStatusResponse represents trading status response
type TradingStatusResponse struct {
	Enabled   bool      `json:"enabled"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version"`
}

// handleHealth is a simple health check endpoint for monitoring/orchestration
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    s.calculateUptime(),
		Version:   s.serverConfig.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		s.logger.Error("Failed to encode health response: " + err.Error())
	}
}

// handleStatus returns system status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// API credentials check - use constants for placeholder values
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
		statusDisplay = "trading (⚠️insufficient credentials)"
	}

	// Calculate uptime dynamically
	uptime := s.calculateUptime()

	// Get actual trade count and active orders
	totalTrades, err := s.getTotalTradesCount()
	if err != nil {
		s.logger.Error("Failed to get total trades count: " + err.Error())
		totalTrades = 0
	}

	activeOrders, err := s.getActiveOrdersCount()
	if err != nil {
		s.logger.Error("Failed to get active orders count: " + err.Error())
		activeOrders = 0
	}

	var tradingEnabled bool
	if s.app != nil {
		tradingEnabled = s.app.IsTradingEnabled()
	}

	status := StatusResponse{
		Status:            "running",
		CredentialsStatus: statusDisplay,
		Strategy:          s.config.Trading.Strategy.Name,
		LastUpdate:        time.Now(),
		Uptime:            uptime,
		TotalTrades:       totalTrades,
		ActiveOrders:      activeOrders,
		TradingEnabled:    tradingEnabled,
		MonitoringSymbols: s.config.Trading.Symbols,
	}

	// Get latest prices for all monitoring symbols (batch query to avoid N+1 problem)
	monitoringPrices := make(map[string]float64)
	latestMarketData, err := s.db.GetLatestMarketDataForSymbols(s.config.Trading.Symbols)
	if err == nil {
		for symbol, data := range latestMarketData {
			monitoringPrices[symbol] = data.Close
		}
	}
	if len(monitoringPrices) > 0 {
		status.MonitoringPrices = monitoringPrices
	}

	// Get symbol from query parameter, default to first configured symbol
	primarySymbol := r.URL.Query().Get("symbol")
	if primarySymbol == "" {
		if len(s.config.Trading.Symbols) > 0 {
			primarySymbol = s.config.Trading.Symbols[0]
		} else {
			primarySymbol = "BTC_JPY"
		}
	}

	// Set current_price from monitoring prices
	if price, ok := monitoringPrices[primarySymbol]; ok {
		status.CurrentPrice = price
	}

	// Note: EMA calculation removed from status endpoint for performance
	// EMA values are calculated in the strategy layer, not needed in UI status
	status.CurrentSignal = "N/A" // Strategy signals are logged, not displayed in real-time

	s.writeJSON(w, status)
}

// BalanceResponse represents balance information response
type BalanceResponse struct {
	Currency   string    `json:"currency"`
	Available  float64   `json:"available"`
	Amount     float64   `json:"amount"`
	LastUpdate time.Time `json:"last_update"`
}

// handleBalance returns balance information
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get actual balance from application
	if s.app != nil {
		balances, err := s.app.GetBalances(r.Context())
		if err != nil {
			s.logger.Error("Failed to get balances from application: " + err.Error())

			// Any API error (rate limit, 403, network issue, etc.): fallback to cached DB data
			cachedBalances, dbErr := s.db.GetLatestBalances()
			if dbErr == nil && len(cachedBalances) > 0 {
				if isRateLimitError(err) {
					s.logger.Info("Rate limit encountered, falling back to cached balance data")
				} else {
					s.logger.Info("API error encountered, falling back to cached balance data")
				}
				s.sortBalances(cachedBalances)
				s.writeJSON(w, cachedBalances)
				return
			}

			// No cached data available
			http.Error(w, "Failed to get balances", http.StatusInternalServerError)
			return
		}
		if len(balances) > 0 {
			// Convert Balance struct to BalanceResponse
			var responses []domain.Balance
			for _, balance := range balances {
				responses = append(responses, domain.Balance{
					Currency:  balance.Currency,
					Amount:    balance.Amount,
					Available: balance.Available,
					Timestamp: time.Now(),
				})
			}
			// Sort balances
			s.sortBalances(responses)
			s.writeJSON(w, responses)
			return
		}
		// Return empty array even when balances is empty
		s.writeJSON(w, []domain.Balance{})
		return
	}

	// Get from database only when application is not initialized
	balances, err := s.db.GetLatestBalances()
	if err != nil {
		s.logger.Error("Failed to get balances from database: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return sample data when data is empty
	if len(balances) == 0 {
		balances = s.generateSampleBalances()
	}

	// Sort balances
	s.sortBalances(balances)

	s.writeJSON(w, balances)
}

// TradeResponse represents trade history response
type TradeResponse struct {
	ID          int       `json:"id"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Size        float64   `json:"size"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee"`
	Timestamp   time.Time `json:"timestamp"`
	Strategy    string    `json:"strategy"`
	PnL         float64   `json:"pnl"`
}

// handleTrades returns trading history
func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate limit parameter (1-1000)
	limit, err := validateIntParam(r.URL.Query().Get("limit"), 50, 1, 1000)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid limit parameter: %v", err), http.StatusBadRequest)
		return
	}

	trades, err := s.db.GetRecentTrades(limit)
	if err != nil {
		s.logger.Error("Failed to get trades: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.System().WithField("count", len(trades)).Info("Returning trades from API")

	// Return actual trading data (empty array if none)
	s.writeJSON(w, trades)
}

// PerformanceResponse represents performance response
type PerformanceResponse struct {
	TotalPnL    float64   `json:"total_pnl"`
	DailyPnL    float64   `json:"daily_pnl"`
	WeeklyPnL   float64   `json:"weekly_pnl"`
	MonthlyPnL  float64   `json:"monthly_pnl"`
	WinRate     float64   `json:"win_rate"`
	MaxDrawdown float64   `json:"max_drawdown"`
	SharpeRatio float64   `json:"sharpe_ratio"`
	LastUpdate  time.Time `json:"last_update"`
}

// handlePerformance returns performance information
func (s *Server) handlePerformance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	performance, err := s.db.GetPerformanceMetrics(30) // Last 30 days
	if err != nil {
		s.logger.Error("Failed to get performance: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.System().WithField("count", len(performance)).Info("Returning performance metrics from API")

	// Return actual performance data (empty array if none)
	s.writeJSON(w, performance)
}

// ConfigUpdateRequest represents config update request
type ConfigUpdateRequest struct {
	Strategy struct {
		Name string `json:"name"`
	} `json:"strategy"`
	Risk struct {
		StopLoss   float64 `json:"stop_loss"`
		TakeProfit float64 `json:"take_profit"`
	} `json:"risk"`
}

// handleConfig returns and updates configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.logger.Info("Returning config",
			"strategy", s.config.Trading.Strategy.Name)
		s.writeJSON(w, s.config)
	case http.MethodPost:
		s.handleConfigUpdate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConfigUpdate updates configuration
func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to prevent DoS (1MB max)
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)

	var req ConfigUpdateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Reject requests with unknown fields
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate strategy name if provided
	if req.Strategy.Name != "" {
		allowedStrategies := []string{"scalping", "simple"}
		if err := validateStringParam(req.Strategy.Name, allowedStrategies); err != nil {
			http.Error(w, fmt.Sprintf("Invalid strategy name: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Validate risk parameters if provided
	if req.Risk.StopLoss < 0 || req.Risk.StopLoss > 100 {
		http.Error(w, "Invalid stop_loss: must be between 0 and 100", http.StatusBadRequest)
		return
	}
	if req.Risk.TakeProfit < 0 || req.Risk.TakeProfit > 100 {
		http.Error(w, "Invalid take_profit: must be between 0 and 100", http.StatusBadRequest)
		return
	}

	s.logger.Info("Updating configuration",
		"strategy", req.Strategy.Name)

	// Update configuration (mode cannot be changed during runtime)
	if req.Strategy.Name != "" {
		s.config.Trading.Strategy.Name = req.Strategy.Name
	}
	// Note: StopLoss and TakeProfit are now managed in strategy parameters, not risk_management

	// Save to configuration file
	if err := s.saveConfigToFile(); err != nil {
		s.logger.Error("Failed to save config to file: " + err.Error())
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Configuration updated successfully")

	// Reload from config file to ensure consistency
	configPath := s.serverConfig.ConfigPath
	reloadedConfig, err := config.Load(configPath)
	if err != nil {
		s.logger.Error("Failed to reload config from file: " + err.Error())
		// Return in-memory configuration even on error
	} else {
		s.config = reloadedConfig // Update with reloaded configuration
		s.logger.Info("Configuration reloaded from file successfully")
	}

	// Return updated configuration
	response := map[string]interface{}{
		"status":  "success",
		"message": "Configuration saved",
		"config":  s.config,
	}

	s.writeJSON(w, response)
}

// saveConfigToFile saves configuration to file
func (s *Server) saveConfigToFile() error {
	// Get configuration file path (usually configs/config.yaml)
	configPath := s.serverConfig.ConfigPath

	// Clone config to avoid modifying the in-memory config.
	// Restore env var placeholders for credentials so plaintext secrets are
	// never written to disk.
	cfgCopy := *s.config
	apiCopy := cfgCopy.API
	apiCopy.Credentials = config.CredentialsConfig{
		APIKey:    "${BITFLYER_API_KEY}",
		APISecret: "${BITFLYER_API_SECRET}",
	}
	cfgCopy.API = apiCopy

	// Convert configuration to YAML
	data, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Create directory if it does not exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write to file with restricted permissions (0600 - owner read/write only)
	// Configuration files may contain sensitive information (API keys)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LogEntry represents log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
}

// handleLogs returns log information
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate limit parameter (1-1000)
	limit, err := validateIntParam(r.URL.Query().Get("limit"), 100, 1, 1000)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid limit parameter: %v", err), http.StatusBadRequest)
		return
	}

	// Validate level filter
	levelFilter := r.URL.Query().Get("level")
	allowedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	if err := validateStringParam(levelFilter, allowedLevels); err != nil {
		http.Error(w, fmt.Sprintf("Invalid level parameter: %v", err), http.StatusBadRequest)
		return
	}

	// Validate category filter
	categoryFilter := r.URL.Query().Get("category")
	allowedCategories := []string{"system", "api", "strategy", "trading", "data", "ui"}
	if err := validateStringParam(categoryFilter, allowedCategories); err != nil {
		http.Error(w, fmt.Sprintf("Invalid category parameter: %v", err), http.StatusBadRequest)
		return
	}

	logs, err := s.db.GetRecentLogsWithFilters(limit, levelFilter, categoryFilter)
	if err != nil {
		s.logger.Error("Failed to get logs: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, logs)
}

// calculateUptime calculates uptime and returns as string
func (s *Server) calculateUptime() string {
	duration := time.Since(s.startTime)

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// OrderResponse represents order response
type OrderResponse struct {
	OrderID    string    `json:"order_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Type       string    `json:"type"`
	Size       float64   `json:"size"`
	Price      float64   `json:"price"`
	Status     string    `json:"status"`
	ExecutedAt time.Time `json:"executed_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleOrders returns order information
func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate limit parameter (1-100)
	limit, err := validateIntParam(r.URL.Query().Get("limit"), 20, 1, 100)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid limit parameter: %v", err), http.StatusBadRequest)
		return
	}

	// Get order information from recent trades
	trades, err := s.db.GetRecentTrades(limit)
	if err != nil {
		s.logger.Error("Failed to get recent trades for orders: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert Trade to OrderResponse
	// Initialize with empty slice to ensure JSON returns [] instead of null
	orders := make([]OrderResponse, 0)
	for i := range trades {
		trade := &trades[i]
		if trade.OrderID != "" { // Only trades with order_id
			orders = append(orders, OrderResponse{
				OrderID:    trade.OrderID,
				Symbol:     trade.Symbol,
				Side:       trade.Side,
				Type:       trade.Type,
				Size:       trade.Size,
				Price:      trade.Price,
				Status:     trade.Status,
				ExecutedAt: trade.ExecutedAt,
				CreatedAt:  trade.CreatedAt,
			})
		}
	}

	s.logger.System().WithField("count", len(orders)).Info("Returning orders from API")

	s.writeJSON(w, orders)
}

// sortBalances sorts balances by currency (JPY first, then alphabetical order)
func (s *Server) sortBalances(balances []domain.Balance) {
	// Define currency display order
	currencyOrder := map[string]int{
		"JPY":  1,
		"BTC":  2,
		"ETH":  3,
		"XRP":  4,
		"XLM":  5,
		"MONA": 6,
	}

	sort.Slice(balances, func(i, j int) bool {
		orderI, existsI := currencyOrder[balances[i].Currency]
		orderJ, existsJ := currencyOrder[balances[j].Currency]

		// If both are defined currencies, use defined order
		if existsI && existsJ {
			return orderI < orderJ
		}
		// If only i is defined, put i first
		if existsI {
			return true
		}
		// If only j is defined, put j first
		if existsJ {
			return false
		}
		// If both are undefined, use alphabetical order
		return balances[i].Currency < balances[j].Currency
	})
}

// generateSampleBalances generates sample balance data
func (s *Server) generateSampleBalances() []domain.Balance {
	now := time.Now()
	return []domain.Balance{
		{
			ID:        1,
			Currency:  "JPY",
			Available: 500000.0,
			Amount:    1000000.0,
			Timestamp: now,
		},
		{
			ID:        2,
			Currency:  "BTC",
			Available: 0.025,
			Amount:    0.05,
			Timestamp: now,
		},
		{
			ID:        3,
			Currency:  "ETH",
			Available: 0.5,
			Amount:    1.0,
			Timestamp: now,
		},
	}
}

// getTotalTradesCount gets total number of trades
func (s *Server) getTotalTradesCount() (int, error) {
	return s.db.GetTradesCount()
}

// getActiveOrdersCount gets number of active orders
func (s *Server) getActiveOrdersCount() (int, error) {
	return s.db.GetActivePositionsCount()
}

// handleStrategyReset resets the strategy
func (s *Server) handleStrategyReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Strategy reset requested via API")

	// Get current strategy from application and reset
	if s.app == nil || s.app.GetCurrentStrategy() == nil {
		http.Error(w, "No strategy available to reset", http.StatusServiceUnavailable)
		return
	}

	strategy := s.app.GetCurrentStrategy()
	if err := strategy.Reset(); err != nil {
		s.logger.Error("Failed to reset strategy: " + err.Error())
		http.Error(w, "Failed to reset strategy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("Strategy reset successfully via API")
	s.writeJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "Strategy reset successfully",
	})
}

// handleTradingStart starts trading
func (s *Server) handleTradingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Trading start requested via API")

	if s.app == nil {
		http.Error(w, "Application not initialized", http.StatusServiceUnavailable)
		return
	}

	if err := s.app.SetTradingEnabled(true); err != nil {
		s.logger.Error("Failed to start trading: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		s.writeJSON(w, TradingStatusResponse{
			Enabled:   false,
			Status:    "error",
			Message:   "Failed to start trading: " + err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.logger.Info("Trading started successfully via API")
	s.writeJSON(w, TradingStatusResponse{
		Enabled:   true,
		Status:    "success",
		Message:   "Trading started successfully",
		Timestamp: time.Now(),
	})
}

// handleTradingStop stops trading
func (s *Server) handleTradingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Trading stop requested via API")

	if s.app == nil {
		http.Error(w, "Application not initialized", http.StatusServiceUnavailable)
		return
	}

	if err := s.app.SetTradingEnabled(false); err != nil {
		s.logger.Error("Failed to stop trading: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		s.writeJSON(w, TradingStatusResponse{
			Enabled:   true,
			Status:    "error",
			Message:   "Failed to stop trading: " + err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.logger.Info("Trading stopped successfully via API")
	s.writeJSON(w, TradingStatusResponse{
		Enabled:   false,
		Status:    "success",
		Message:   "Trading stopped successfully",
		Timestamp: time.Now(),
	})
}

// handleTradingStatus returns trading status
func (s *Server) handleTradingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.app == nil {
		http.Error(w, "Application not initialized", http.StatusServiceUnavailable)
		return
	}

	enabled := s.app.IsTradingEnabled()
	status := "stopped"
	message := "Trading is currently stopped"

	if enabled {
		status = "running"
		message = "Trading is currently active"
	}

	s.writeJSON(w, TradingStatusResponse{
		Enabled:   enabled,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// isRateLimitError checks if an error is a rate limit error using sentinel errors
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's the sentinel error using errors.Is()
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		return true
	}

	// Check if it's a wrapped domain.Error with rate limit type
	var domainErr *domain.Error
	if errors.As(err, &domainErr) && domainErr.Type == domain.ErrTypeRateLimit {
		return true
	}

	// Check bitFlyer package errors
	if errors.Is(err, bitflyer.ErrAPIRateLimitExceeded) {
		return true
	}

	// Check bitFlyer APIError for rate limit status
	if bitflyer.IsRateLimitError(err) {
		return true
	}

	// Fallback to string matching for external errors (e.g., HTTP 429)
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429")
}

// writeJSON writes JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// validateIntParam validates integer query parameter with min/max bounds
func validateIntParam(paramStr string, defaultVal, min, max int) (int, error) {
	if paramStr == "" {
		return defaultVal, nil
	}

	val, err := strconv.Atoi(paramStr)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value: %w", err)
	}

	if val < min {
		return 0, fmt.Errorf("value %d is below minimum %d", val, min)
	}

	if val > max {
		return 0, fmt.Errorf("value %d exceeds maximum %d", val, max)
	}

	return val, nil
}

// validateStringParam validates string parameter against allowed values
func validateStringParam(param string, allowedValues []string) error {
	if param == "" {
		return nil // Empty is allowed (optional parameter)
	}

	for _, allowed := range allowedValues {
		if param == allowed {
			return nil
		}
	}

	return fmt.Errorf("invalid value '%s', must be one of: %v", param, allowedValues)
}
