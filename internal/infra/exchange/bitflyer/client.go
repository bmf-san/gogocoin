package bitflyer

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/auth"
	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/go-bitflyer-api-client/client/websocket"
	"github.com/bmf-san/gogocoin/internal/logger"
)

// Client is the bitFlyer API client wrapper
type Client struct {
	httpClient *http.ClientWithResponses
	wsClient   *websocket.Client
	logger     logger.LoggerInterface

	// configuration
	config *Config

	// State management
	isConnected     bool
	closed          bool               // set to true by Close(); prevents a racing Reconnect from storing a new wsClient
	heartbeatCancel context.CancelFunc // cancels the heartbeat goroutine on Close/Reconnect
	mu              sync.RWMutex

	// rate limiting
	rateLimiter *RateLimiter

	// retry policy
	retryPolicy *RetryPolicy

	// data validator
	validator *DataValidator
}

// Config is the bitFlyer client configuration
type Config struct {
	APIKey            string
	APISecret         string
	Endpoint          string
	WebSocketEndpoint string
	Timeout           time.Duration
	RetryCount        int
	RequestsPerMinute int
	InitialBalance    float64 // Initial balance
	FeeRate           float64 // Fee rate (e.g., 0.0015 = 0.15%)
}

// NewClient creates a new bitFlyer client
func NewClient(config *Config, log logger.LoggerInterface) (*Client, error) {
	client := &Client{
		config:      config,
		logger:      log,
		rateLimiter: NewRateLimiter(config.RequestsPerMinute, time.Minute),
		retryPolicy: NewRetryPolicy(config.RetryCount, time.Second),
		validator:   NewDataValidator(),
	}

	// Initialize HTTP client
	if err := client.initHTTPClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// Initialize WebSocket client
	if err := client.initWebSocketClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize WebSocket client: %w", err)
	}

	return client, nil
}

// initHTTPClient initializes the HTTP client
func (c *Client) initHTTPClient() error {
	credentials := auth.APICredentials{
		APIKey:    c.config.APIKey,
		APISecret: c.config.APISecret,
	}

	// Wrap the default transport with dateFixingTransport so that
	// timezone-less datetime strings returned by the bitFlyer API (e.g.
	// "2026-03-31T13:08:33") are normalised to RFC3339 UTC ("...Z") before
	// encoding/json tries to unmarshal them into time.Time fields.
	httpTimeout := c.config.Timeout
	if httpTimeout <= 0 {
		httpTimeout = 30 * time.Second
	}
	customHTTPClient := &nethttp.Client{
		Transport: newDateFixingTransport(nil),
		Timeout:   httpTimeout,
	}

	authClient, err := http.NewAuthenticatedClient(
		credentials,
		c.config.Endpoint,
		http.WithCustomHTTPClient(customHTTPClient),
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	c.httpClient = authClient.Client()
	c.logger.API().Info("HTTP client initialized successfully")
	return nil
}

// initWebSocketClient initializes the WebSocket client
func (c *Client) initWebSocketClient() error {
	// Use a bounded context so that a network outage or exchange maintenance
	// window cannot block this call (and therefore the reconnect loop) forever.
	// 30 s is well above the typical WS handshake RTT while still allowing the
	// reconnect worker to apply its own exponential back-off promptly.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.API().WithField("endpoint", c.config.WebSocketEndpoint).Info("Initializing WebSocket client")

	client, err := websocket.NewClient(ctx, c.config.WebSocketEndpoint)
	if err != nil {
		c.logger.API().WithError(err).WithField("endpoint", c.config.WebSocketEndpoint).Error("Failed to create WebSocket client")
		return fmt.Errorf("failed to create websocket client: %w", err)
	}

	c.logger.API().Info("WebSocket client created successfully")

	// Authentication (for private channels)
	if c.config.APIKey != "" && c.config.APISecret != "" {
		c.logger.API().Info("Attempting WebSocket authentication for private channels")
		if err := client.Auth(ctx, c.config.APIKey, c.config.APISecret); err != nil {
			c.logger.API().WithError(err).Warn("WebSocket authentication failed, using public channels only")
		} else {
			c.logger.API().Info("WebSocket authenticated successfully")
		}
	} else {
		c.logger.API().Info("Using public channels only (no credentials)")
	}

	// If Close() was called while we were connecting, discard the new client
	// immediately to prevent: (1) a leaked connection and (2) a later ticker
	// callback trying to send on an already-closed marketDataCh.
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		client.Close(closeCtx)
		return fmt.Errorf("client already closed")
	}
	c.wsClient = client
	c.isConnected = true
	hbCtx, hbCancel := context.WithCancel(context.Background())
	c.heartbeatCancel = hbCancel
	c.mu.Unlock()

	go c.runHeartbeat(hbCtx)

	c.logger.API().Info("WebSocket client initialized successfully")
	return nil
}

// runHeartbeat sends a WebSocket ping every 60 seconds to prevent NAT firewalls
// from silently dropping the idle TCP connection (typical NAT timeout: 2-5 min).
// The ping frame itself resets the NAT timer even if the server does not reply
// with a pong (bitflyer does not send pong frames). Dead connections are detected
// by staleDataTimeout in the market data worker — not here.
func (c *Client) runHeartbeat(ctx context.Context) {
	const interval = 60 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.RLock()
			ws := c.wsClient
			c.mu.RUnlock()
			if ws == nil {
				return
			}
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := ws.Ping(pingCtx)
			cancel()
			if err != nil {
				// bitflyer does not respond to WS ping frames, so a timeout here
				// is expected and does not mean the connection is dead.
				// Continue sending pings to keep the NAT entry alive.
				c.logger.API().WithError(err).Debug("WebSocket heartbeat ping: no pong (expected for bitflyer)")
				continue
			}
			c.logger.API().Debug("WebSocket heartbeat ping OK")
		case <-ctx.Done():
			return
		}
	}
}

// IsConnected returns the connection state
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// SetDisconnected marks the client as disconnected so the worker will reconnect.
// Used by the market data worker when it detects a silently dead WebSocket.
func (c *Client) SetDisconnected() {
	c.mu.Lock()
	c.isConnected = false
	c.mu.Unlock()
}

// Reconnect closes the existing WebSocket connection and establishes a new one.
// Callers should also call MarketDataService.ResetCallbacks() after this.
func (c *Client) Reconnect(ctx context.Context) error {
	// Grab and nil the old client under lock, then Close outside the lock.
	// Holding the lock during Close() (which may block on WS handshake) would
	// prevent concurrent IsConnected() / SetDisconnected() calls from proceeding.
	c.mu.Lock()
	old := c.wsClient
	c.wsClient = nil
	c.isConnected = false
	oldCancel := c.heartbeatCancel
	c.heartbeatCancel = nil
	c.mu.Unlock()

	if oldCancel != nil {
		oldCancel() // stop old heartbeat before closing
	}
	if old != nil {
		old.Close(ctx)
	}

	return c.initWebSocketClient()
}

// Close closes the client
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true // prevent any concurrent Reconnect from storing a new wsClient
	oldCancel := c.heartbeatCancel
	c.heartbeatCancel = nil
	c.mu.Unlock()

	if oldCancel != nil {
		oldCancel() // stop heartbeat before closing connection
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true // prevent any concurrent Reconnect from storing a new wsClient

	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}

	if c.wsClient != nil {
		c.wsClient.Close(ctx)
		c.wsClient = nil // idempotent: prevent double-close
	}

	c.isConnected = false
	c.logger.API().Info("bitFlyer client closed")
	return nil
}

// GetHTTPClient returns the HTTP client
func (c *Client) GetHTTPClient() *http.ClientWithResponses {
	return c.httpClient
}

// WaitForRateLimit waits for rate limiter before making API requests
func (c *Client) WaitForRateLimit(ctx context.Context) error {
	if c.rateLimiter == nil {
		return nil
	}
	return c.rateLimiter.Wait(ctx)
}

// GetWebSocketClient returns the WebSocket client
func (c *Client) GetWebSocketClient() *websocket.Client {
	return c.wsClient
}

// GetFeeRate returns the configured fee rate
func (c *Client) GetFeeRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.FeeRate
}

// MarketInfo represents market information from bitFlyer
// Note: bitFlyer API /v1/markets only provides ProductCode and MarketType.
// Other fields (SizeMin, SizeMax, PriceMin, PriceMax) are set by the application
// based on known market specifications since they are not provided by the API.
type MarketInfo struct {
	ProductCode string  `json:"product_code"`
	MarketType  string  `json:"market_type"`
	Alias       string  `json:"alias"`
	SizeMin     float64 `json:"size_min"`  // Not from API - set by application
	SizeMax     float64 `json:"size_max"`  // Not from API - set by application
	PriceMin    float64 `json:"price_min"` // Not from API - set by application
	PriceMax    float64 `json:"price_max"` // Not from API - set by application
}

// GetMarkets fetches market information from bitFlyer API
func (c *Client) GetMarkets(ctx context.Context) ([]MarketInfo, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	start := time.Now()

	resp, err := c.httpClient.GetV1MarketsWithResponse(ctx)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		c.logger.LogAPICall("GET", "/v1/markets", duration, 0, err)
		return nil, fmt.Errorf("failed to get markets: %w", err)
	}

	c.logger.LogAPICall("GET", "/v1/markets", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	var markets []MarketInfo
	for _, market := range *resp.JSON200 {
		info := MarketInfo{
			ProductCode: safeStringPtr(market.ProductCode),
		}

		if market.MarketType != nil {
			info.MarketType = string(*market.MarketType)
		}

		markets = append(markets, info)
	}

	c.logger.API().WithField("count", len(markets)).Info("Fetched market information")
	return markets, nil
}

// GetMarketInfo fetches market information for a specific symbol
func (c *Client) GetMarketInfo(ctx context.Context, symbol string) (*MarketInfo, error) {
	markets, err := c.GetMarkets(ctx)
	if err != nil {
		return nil, err
	}

	for i := range markets {
		if markets[i].ProductCode == symbol {
			return &markets[i], nil
		}
	}

	return nil, fmt.Errorf("market not found: %s", symbol)
}

func safeStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	requests chan struct{}
	ticker   *time.Ticker
	done     chan struct{} // Signal to stop the goroutine
	stopOnce sync.Once     // Ensures Stop() is safe to call more than once
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerPeriod int, period time.Duration) *RateLimiter {
	// Validate parameters to prevent division by zero
	if requestsPerPeriod <= 0 {
		requestsPerPeriod = 1
	}
	if period <= 0 {
		period = time.Second
	}

	rl := &RateLimiter{
		requests: make(chan struct{}, requestsPerPeriod),
		ticker:   time.NewTicker(period / time.Duration(requestsPerPeriod)),
		done:     make(chan struct{}),
	}

	// Configure initial request allowance
	for i := 0; i < requestsPerPeriod; i++ {
		rl.requests <- struct{}{}
	}

	// Periodically replenish request allowance
	go func() {
		for {
			select {
			case <-rl.ticker.C:
				select {
				case rl.requests <- struct{}{}:
				default:
					// Skip if channel is full
				}
			case <-rl.done:
				return // Stop the goroutine
			}
		}
	}()

	return rl
}

// Wait checks rate limiting and waits if necessary
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.requests:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the rate limiter. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		rl.ticker.Stop()
		close(rl.done) // Signal goroutine to stop
	})
}

// RetryPolicy defines retry policy
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// NewRetryPolicy creates a new retry policy
func NewRetryPolicy(maxRetries int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
	}
}

// ShouldRetry determines whether to retry
func (rp *RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if attempt >= rp.MaxRetries {
		return false
	}

	// Determine retry based on error type
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable()
	}

	// Network errors are retryable
	return isNetworkError(err)
}

// GetDelay calculates the wait time using exponential backoff
func (rp *RetryPolicy) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Cap attempt to prevent overflow (max 2^10 = 1024)
	if attempt > 10 {
		attempt = 10
	}

	delay := rp.BaseDelay * time.Duration(1<<(attempt-1))

	// Cap at 5 minutes maximum (to handle API limits)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}

	return delay
}

// DataValidator provides data validation functionality
// For cryptocurrencies with large price fluctuations, only check for obviously invalid values
type DataValidator struct{}

// NewDataValidator creates a new data validator
func NewDataValidator() *DataValidator {
	return &DataValidator{}
}

// ValidatePrice validates price validity
// Only check for obviously invalid values (zero or negative, extremely large values)
func (dv *DataValidator) ValidatePrice(symbol string, price float64) error {
	if price <= 0 {
		return fmt.Errorf("invalid price: %f (must be positive)", price)
	}

	// Check for extremely large values (obvious data errors)
	// e.g., BTC exceeding 100 million yen, small coins exceeding 1 million yen, etc.
	const maxReasonablePrice = 100000000.0 // 100 million yen
	if price > maxReasonablePrice {
		return fmt.Errorf("price too large for %s: %f (exceeds reasonable maximum %f)",
			symbol, price, maxReasonablePrice)
	}

	return nil
}

// ValidateVolume validates volume validity
// Only check for negative values (volume should be zero or positive)
func (dv *DataValidator) ValidateVolume(symbol string, volume float64) error {
	if volume < 0 {
		return fmt.Errorf("invalid volume: %f (must be non-negative)", volume)
	}

	return nil
}

// APIError represents an API-related error
type APIError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Retryable  bool                   `json:"retryable"`
	Timestamp  time.Time              `json:"timestamp"`
	StatusCode int                    `json:"status_code,omitempty"` // HTTP status code
}

// Error returns the error message
func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] (HTTP %d) %s", e.Code, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// IsRetryable returns whether the error is retryable
func (e *APIError) IsRetryable() bool {
	return e.Retryable
}

// IsAuthenticationError checks if the error is an authentication error
func (e *APIError) IsAuthenticationError() bool {
	return e.StatusCode == 401 || e.Code == "UNAUTHORIZED" || e.Code == "AUTHENTICATION_FAILED"
}

// isNetworkError determines whether it is a network error
func isNetworkError(err error) bool {
	// Simple implementation: determine based on error message
	errMsg := err.Error()
	networkErrors := []string{
		"connection refused",
		"timeout",
		"network is unreachable",
		"no such host",
		"connection reset",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errMsg, netErr) {
			return true
		}
	}

	return false
}

// WithRetry executes a function with retry logic using the configured retry policy
func (c *Client) WithRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check context before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Calculate exponential backoff delay
			delay := c.retryPolicy.GetDelay(attempt)
			c.logger.API().
				WithField("operation", operation).
				WithField("attempt", attempt).
				WithField("delay_ms", delay.Milliseconds()).
				WithError(lastErr).
				Warn("Retrying API call after error")

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Execute the operation
		err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				c.logger.API().
					WithField("operation", operation).
					WithField("attempt", attempt).
					Info("API call succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !c.retryPolicy.ShouldRetry(attempt, err) {
			c.logger.API().
				WithField("operation", operation).
				WithField("attempt", attempt).
				WithError(err).
				Debug("Error is not retryable or max retries reached")
			return err
		}
	}

	return lastErr
}
