package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/internal/config"
	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/logger"
)

// Server is the API server for Web UI
type Server struct {
	configMu     sync.RWMutex   // guards config pointer and its mutable fields
	config       *config.Config
	db           DatabaseService
	logger       logger.LoggerInterface
	host         string
	port         int
	serverConfig *ServerConfig
	startTime    time.Time
	app          ApplicationService
	httpServer   *http.Server
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, db DatabaseService, logger logger.LoggerInterface) *Server {
	return NewServerWithConfig(cfg, db, logger, DefaultServerConfig())
}

// NewServerWithConfig creates a new API server with custom server config
func NewServerWithConfig(cfg *config.Config, db DatabaseService, logger logger.LoggerInterface, serverCfg *ServerConfig) *Server {
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

// getConfig returns the current config under a read lock.
// Always use this instead of accessing s.config directly from handlers.
func (s *Server) getConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

// Start starts the API server
func (s *Server) Start() error {
	// Create new ServeMux
	mux := http.NewServeMux()

	// Register API routes via oapi-codegen generated strict handler
	HandlerWithOptions(NewStrictHandler(s, nil), StdHTTPServerOptions{
		BaseRouter: mux,
	})

	// Serve static files (non-API paths)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 for API paths not handled above
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Serve static files
		http.FileServer(http.Dir(s.serverConfig.WebRoot)).ServeHTTP(w, r)
	})

	s.logger.UI().Info(fmt.Sprintf("Starting web server on %s:%d", s.host, s.port))

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

// getTodayTradesCount gets the number of trades executed today (JST)
func (s *Server) getTodayTradesCount() (int, error) {
	return s.db.GetTodayTradesCount()
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
