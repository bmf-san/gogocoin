package bitflyer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/auth"
	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/go-bitflyer-api-client/client/websocket"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// Client is the bitFlyer API client wrapper
type Client struct {
	httpClient *http.ClientWithResponses
	wsClient   *websocket.Client
	logger     *logger.Logger

	// configuration
	config *Config

	// State management
	isConnected bool
	mu          sync.RWMutex

	// rate limiting
	rateLimiter *RateLimiter

	// retry policy
	retryPolicy *RetryPolicy

	// dataバリデーター
	validator *DataValidator
}

// Config isbitFlyerclientofconfiguration
type Config struct {
	APIKey            string
	APISecret         string
	Endpoint          string
	WebSocketEndpoint string
	Timeout           time.Duration
	RetryCount        int
	RequestsPerMinute int
	PaperTrading      bool
	InitialBalance    float64 // papertrade用初期balance
	FeeRate           float64 // papertrade用手数料率（例: 0.0015 = 0.15%）
}

// NewClient is新しいbitFlyerclientcreates
func NewClient(config *Config, log *logger.Logger) (*Client, error) {
	client := &Client{
		config:      config,
		logger:      log,
		rateLimiter: NewRateLimiter(config.RequestsPerMinute, time.Minute),
		retryPolicy: NewRetryPolicy(config.RetryCount, time.Second),
		validator:   NewDataValidator(),
	}

	// HTTPclientof初期化
	if err := client.initHTTPClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// WebSocketclientof初期化（papertradewithもmarket datais必要）
	if err := client.initWebSocketClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize WebSocket client: %w", err)
	}

	return client, nil
}

// initHTTPClient isHTTPclientinitializes
func (c *Client) initHTTPClient() error {
	credentials := auth.APICredentials{
		APIKey:    c.config.APIKey,
		APISecret: c.config.APISecret,
	}

	client, err := http.NewAuthenticatedClient(credentials, c.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to create authenticated client: %w", err)
	}

	c.httpClient = client.Client()
	c.logger.API().Info("HTTP client initialized successfully")
	return nil
}

// initWebSocketClient isWebSocketclientinitializes
func (c *Client) initWebSocketClient() error {
	ctx := context.Background()

	c.logger.API().WithField("endpoint", c.config.WebSocketEndpoint).Info("Initializing WebSocket client")

	client, err := websocket.NewClient(ctx, c.config.WebSocketEndpoint)
	if err != nil {
		c.logger.API().WithError(err).WithField("endpoint", c.config.WebSocketEndpoint).Error("Failed to create WebSocket client")
		return fmt.Errorf("failed to create websocket client: %w", err)
	}

	c.logger.API().Info("WebSocket client created successfully")

	// 認証（プライベートchannel用）
	if c.config.APIKey != "" && c.config.APISecret != "" && !c.config.PaperTrading {
		c.logger.API().Info("Attempting WebSocket authentication for private channels")
		if err := client.Auth(ctx, c.config.APIKey, c.config.APISecret); err != nil {
			c.logger.API().WithError(err).Warn("WebSocket authentication failed, using public channels only")
		} else {
			c.logger.API().Info("WebSocket authenticated successfully")
		}
	} else {
		c.logger.API().Info("Using public channels only (paper trading mode or no credentials)")
	}

	c.wsClient = client
	c.mu.Lock()
	c.isConnected = true
	c.mu.Unlock()

	c.logger.API().Info("WebSocket client initialized successfully")
	return nil
}

// IsConnected isconnection状態returns
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// Close isclient" "クローズする
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}

	if c.wsClient != nil {
		c.wsClient.Close(ctx)
	}

	c.isConnected = false
	c.logger.API().Info("bitFlyer client closed")
	return nil
}

// GetHTTPClient isHTTPclientreturns
func (c *Client) GetHTTPClient() *http.ClientWithResponses {
	return c.httpClient
}

// GetWebSocketClient isWebSocketclientreturns
func (c *Client) GetWebSocketClient() *websocket.Client {
	return c.wsClient
}

// RateLimiter israte limiting機能" "提供する
type RateLimiter struct {
	requests chan struct{}
	ticker   *time.Ticker
}

// NewRateLimiter is新しいrate limiting器creates
func NewRateLimiter(requestsPerPeriod int, period time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(chan struct{}, requestsPerPeriod),
		ticker:   time.NewTicker(period / time.Duration(requestsPerPeriod)),
	}

	// 初期リクエスト許可数" "configuration
	for i := 0; i < requestsPerPeriod; i++ {
		rl.requests <- struct{}{}
	}

	// 定期的にリクエスト許可" "補充
	go func() {
		for range rl.ticker.C {
			select {
			case rl.requests <- struct{}{}:
			default:
				// channelが満杯of場合isスキップ
			}
		}
	}()

	return rl
}

// Wait israte limiting" "チェックし、必要に応じて待機する
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.requests:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop israte limiting器stops
func (rl *RateLimiter) Stop() {
	rl.ticker.Stop()
}

// RetryPolicy isretry policy" "定義する
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// NewRetryPolicy is新しいretry policycreates
func NewRetryPolicy(maxRetries int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
	}
}

// ShouldRetry isリトライすべきかどうか" "判定する
func (rp *RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if attempt >= rp.MaxRetries {
		return false
	}

	// errorof種類に基づいてリトライ" "判定
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable()
	}

	// ネットワークerrorisリトライ
	return isNetworkError(err)
}

// GetDelay is指数バックオフによる待機時間calculates
func (rp *RetryPolicy) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := rp.BaseDelay * time.Duration(1<<(attempt-1))

	// 最大60秒まwith
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}

	return delay
}

// DataValidator provides data validation functionality" "提供する
// 暗号通貨isprice変動が大きいため、明らかに異常な値ofみ" "チェックする
type DataValidator struct{}

// NewDataValidator is新しいdataバリdataーcreates
func NewDataValidator() *DataValidator {
	return &DataValidator{}
}

// ValidatePrice ispriceof妥当性validates
// 明らかに異常な値（0以下、極端に大きい値）ofみ" "チェック
func (dv *DataValidator) ValidatePrice(symbol string, price float64) error {
	if price <= 0 {
		return fmt.Errorf("invalid price: %f (must be positive)", price)
	}

	// 極端に大きい値" "チェック（明らかなdataerror）
	// 例: BTCが1億円" "超える、小額コインが100万円" "超えるなど
	const maxReasonablePrice = 100000000.0 // 1億円
	if price > maxReasonablePrice {
		return fmt.Errorf("price too large for %s: %f (exceeds reasonable maximum %f)",
			symbol, price, maxReasonablePrice)
	}

	return nil
}

// ValidateVolume isvolumeof妥当性validates
// 負of値ofみ" "チェック（volumeis0以上withあるべき）
func (dv *DataValidator) ValidateVolume(symbol string, volume float64) error {
	if volume < 0 {
		return fmt.Errorf("invalid volume: %f (must be non-negative)", volume)
	}

	return nil
}

// APIError isAPI関連oferrorrepresents
type APIError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
	Timestamp time.Time              `json:"timestamp"`
}

// Error iserrorメッセージreturns
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// IsRetryable isリトライ可能かどうかreturns
func (e *APIError) IsRetryable() bool {
	return e.Retryable
}

// isNetworkError isネットワークerrorかどうか" "判定する
func isNetworkError(err error) bool {
	// 簡単な実装：errorメッセージに基づく判定
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
