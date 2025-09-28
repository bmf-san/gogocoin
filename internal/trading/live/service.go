package live

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
)

// Trader is the live trading service
// It executes trades using the actual bitFlyer API
type Trader struct {
	client       *bitflyer.Client
	logger       *logger.Logger
	db           trading.DatabaseSaver
	strategyName string
	mu           sync.RWMutex
}

// NewTrader creates a new live trading service
func NewTrader(client *bitflyer.Client, log *logger.Logger) *Trader {
	return &Trader{
		client: client,
		logger: log,
		mu:     sync.RWMutex{},
	}
}

// PlaceOrder executes an order (live trading)
func (t *Trader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate input
	if err := t.validateOrder(order); err != nil {
		return nil, fmt.Errorf("invalid order: %w", err)
	}

	httpClient := t.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	// Check balance
	balances, err := t.getBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	if err := t.checkBalance(order, balances); err != nil {
		return nil, fmt.Errorf("insufficient balance: %w", err)
	}

	start := time.Now()

	// Convert order type
	var childOrderType http.NewOrderRequestChildOrderType
	switch order.Type {
	case "LIMIT":
		childOrderType = http.NewOrderRequestChildOrderTypeLIMIT
	case "MARKET":
		childOrderType = http.NewOrderRequestChildOrderTypeMARKET
	default:
		childOrderType = http.NewOrderRequestChildOrderTypeMARKET
	}

	// Convert order side
	var side http.NewOrderRequestSide
	switch order.Side {
	case "BUY":
		side = http.NewOrderRequestSideBUY
	case "SELL":
		side = http.NewOrderRequestSideSELL
	default:
		side = http.NewOrderRequestSideBUY
	}

	// Build order request
	requestBody := http.PostV1MeSendchildorderJSONRequestBody{
		ProductCode:    order.Symbol,
		ChildOrderType: childOrderType,
		Side:           side,
		Size:           float32(order.Size),
	}

	if order.Type == "LIMIT" {
		price := float32(order.Price)
		requestBody.Price = &price
	}

	if order.TimeInForce != "" {
		var timeInForce http.NewOrderRequestTimeInForce
		switch order.TimeInForce {
		case "GTC":
			timeInForce = http.NewOrderRequestTimeInForceGTC
		case "IOC":
			timeInForce = http.NewOrderRequestTimeInForceIOC
		case "FOK":
			timeInForce = http.NewOrderRequestTimeInForceFOK
		default:
			timeInForce = http.NewOrderRequestTimeInForceIOC
		}
		requestBody.TimeInForce = &timeInForce
	}

	if order.MinuteToExpire > 0 {
		minuteToExpire := int(order.MinuteToExpire)
		requestBody.MinuteToExpire = &minuteToExpire
	}

	// Call API
	resp, err := httpClient.PostV1MeSendchildorderWithResponse(ctx, requestBody)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		t.logger.LogAPICall("POST", "/v1/me/sendchildorder", duration, 0, err)
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	t.logger.LogAPICall("POST", "/v1/me/sendchildorder", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	// Build order result
	var orderID string
	if resp.JSON200.ChildOrderAcceptanceId != nil {
		orderID = *resp.JSON200.ChildOrderAcceptanceId
	}

	result := &domain.OrderResult{
		OrderID:       orderID,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          order.Type,
		Size:          order.Size,
		Price:         order.Price,
		Status:        "ACTIVE",
		FilledSize:    0,
		RemainingSize: order.Size,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	t.logger.LogTrade("ORDER_PLACED", order.Symbol, order.Price, order.Size, map[string]interface{}{
		"order_id": result.OrderID,
		"side":     order.Side,
		"type":     order.Type,
	})

	// Execute order confirmation and status update asynchronously
	go t.monitorOrderExecution(ctx, result)

	return result, nil
}

// CancelOrder cancels an order
func (t *Trader) CancelOrder(ctx context.Context, orderID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	httpClient := t.client.GetHTTPClient()
	if httpClient == nil {
		return fmt.Errorf("HTTP client not initialized")
	}

	start := time.Now()

	// Cancel request
	requestBody := http.PostV1MeCancelchildorderJSONRequestBody{
		ChildOrderAcceptanceId: &orderID,
	}

	resp, err := httpClient.PostV1MeCancelchildorderWithResponse(ctx, requestBody)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		t.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, 0, err)
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	t.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	t.logger.Trading().WithField("order_id", orderID).Info("Order canceled successfully")
	return nil
}

// GetBalance retrieves balance information
func (t *Trader) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.getBalance(ctx)
}

// GetOrders retrieves the list of orders
func (t *Trader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	httpClient := t.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	start := time.Now()

	// Get last 100 orders
	count := http.Count(100)
	params := http.GetV1MeGetchildordersParams{
		Count: &count,
	}

	resp, err := httpClient.GetV1MeGetchildordersWithResponse(ctx, &params)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		t.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, 0, err)
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	t.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	var orders []*domain.OrderResult
	for i := range *resp.JSON200 {
		order := &(*resp.JSON200)[i]

		var side, orderType, status string
		if order.Side != nil {
			side = string(*order.Side)
		}
		if order.ChildOrderType != nil {
			orderType = string(*order.ChildOrderType)
		}
		if order.ChildOrderState != nil {
			status = string(*order.ChildOrderState)
		}

		var createdAt, updatedAt time.Time
		if order.ChildOrderDate != nil {
			createdAt = *order.ChildOrderDate
			updatedAt = *order.ChildOrderDate
		}

		orderResult := &domain.OrderResult{
			OrderID:       safeString(order.ChildOrderAcceptanceId),
			Symbol:        safeString(order.ProductCode),
			Side:          side,
			Type:          orderType,
			Size:          float64(safeFloat32(order.Size)),
			Price:         float64(safeFloat32(order.Price)),
			Status:        status,
			FilledSize:    float64(safeFloat32(order.ExecutedSize)),
			RemainingSize: float64(safeFloat32(order.OutstandingSize)),
			AveragePrice:  float64(safeFloat32(order.AveragePrice)),
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}

		orders = append(orders, orderResult)
	}

	t.logger.Trading().Info("Retrieved live orders", "count", len(orders))
	return orders, nil
}

// SetDatabase sets the database saver interface
func (t *Trader) SetDatabase(db trading.DatabaseSaver) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.db = db
}

// SetStrategyName sets the strategy name
func (t *Trader) SetStrategyName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.strategyName = name
}

// validateOrder validates the order
func (t *Trader) validateOrder(order *domain.OrderRequest) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if order.Side != "BUY" && order.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}
	if order.Size <= 0 {
		return fmt.Errorf("size must be positive: %f", order.Size)
	}
	if order.Type == "LIMIT" && order.Price <= 0 {
		return fmt.Errorf("price must be positive for LIMIT orders")
	}
	return nil
}

// getBalance retrieves balance (internal use)
func (t *Trader) getBalance(ctx context.Context) ([]domain.Balance, error) {
	httpClient := t.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	start := time.Now()

	resp, err := httpClient.GetV1MeGetbalanceWithResponse(ctx)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		t.logger.LogAPICall("GET", "/v1/me/getbalance", duration, 0, err)
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	t.logger.LogAPICall("GET", "/v1/me/getbalance", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	var balances []domain.Balance
	for _, bal := range *resp.JSON200 {
		balance := domain.Balance{
			Currency:  safeString(bal.CurrencyCode),
			Amount:    float64(safeFloat32(bal.Amount)),
			Available: float64(safeFloat32(bal.Available)),
			Timestamp: time.Now(),
		}
		balances = append(balances, balance)

		// Debug log
		t.logger.Trading().
			WithField("currency", balance.Currency).
			WithField("amount", balance.Amount).
			WithField("available", balance.Available).
			Info("Live balance retrieved from bitFlyer API")
	}

	t.logger.Trading().
		WithField("total_currencies", len(balances)).
		Info("All live balances retrieved successfully")

	return balances, nil
}

// checkBalance checks if balance is sufficient
func (t *Trader) checkBalance(order *domain.OrderRequest, balances []domain.Balance) error {
	if order.Side == "BUY" {
		// Buy order: check JPY balance
		var jpyBalance *domain.Balance
		for i := range balances {
			if balances[i].Currency == "JPY" {
				jpyBalance = &balances[i]
				break
			}
		}
		if jpyBalance == nil {
			return fmt.Errorf("JPY balance not found")
		}

		requiredAmount := order.Size * order.Price
		if jpyBalance.Available < requiredAmount {
			return fmt.Errorf("insufficient JPY balance: required %f, available %f",
				requiredAmount, jpyBalance.Available)
		}
	} else {
		// Sell order: check currency balance
		currency := getCurrencyFromSymbol(order.Symbol)

		var currencyBalance *domain.Balance
		for i := range balances {
			if balances[i].Currency == currency {
				currencyBalance = &balances[i]
				break
			}
		}
		if currencyBalance == nil {
			return fmt.Errorf("%s balance not found", currency)
		}

		if currencyBalance.Available < order.Size {
			return fmt.Errorf("insufficient %s balance: required %f, available %f",
				currency, order.Size, currencyBalance.Available)
		}
	}

	return nil
}

// monitorOrderExecution monitors order execution
func (t *Trader) monitorOrderExecution(ctx context.Context, result *domain.OrderResult) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	maxWaitTime := 5 * time.Minute
	timeout := time.After(maxWaitTime)

	for {
		select {
		case <-ctx.Done():
			t.logger.Trading().WithField("order_id", result.OrderID).Info("Order monitoring canceled")
			return
		case <-timeout:
			t.logger.Trading().WithField("order_id", result.OrderID).Warn("Order monitoring timeout")
			return
		case <-ticker.C:
			if t.checkOrderStatus(ctx, result) {
				return
			}
		}
	}
}

// checkOrderStatus checks order status and returns true if completed
func (t *Trader) checkOrderStatus(ctx context.Context, result *domain.OrderResult) bool {
	orders, err := t.GetOrders(ctx)
	if err != nil {
		// If authentication fails (401), stop monitoring to avoid spam
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
			t.logger.Trading().WithField("order_id", result.OrderID).
				Error("Authentication failed - stopping order monitoring. Please configure valid API credentials.")
			return true // Stop monitoring
		}
		t.logger.Trading().WithError(err).Error("Failed to get order status")
		return false
	}

	for _, order := range orders {
		if order.OrderID == result.OrderID {
			result.Status = order.Status
			result.FilledSize = order.FilledSize
			result.RemainingSize = order.RemainingSize
			result.AveragePrice = order.AveragePrice
			result.UpdatedAt = time.Now()

			if order.Status == "COMPLETED" {
				t.logger.Trading().WithField("order_id", result.OrderID).Info("Order completed")

				// Update balance to database
				t.updateBalanceToDB(ctx)

				// Save trade to database
				t.saveTradeToDB(result)

				return true
			}
		}
	}

	return false
}

// saveTradeToDB saves trade to database
func (t *Trader) saveTradeToDB(result *domain.OrderResult) {
	if t.db == nil {
		return
	}

	strategyName := t.strategyName
	if strategyName == "" {
		strategyName = "unknown"
	}

	trade := &domain.Trade{
		Symbol:       result.Symbol,
		Side:         result.Side,
		Type:         result.Type,
		Size:         result.FilledSize,
		Price:        result.AveragePrice,
		Status:       result.Status,
		Fee:          result.Fee,
		ExecutedAt:   result.UpdatedAt,
		CreatedAt:    result.CreatedAt,
		OrderID:      result.OrderID,
		StrategyName: strategyName,
		PnL:          0, // PnL is not manually calculated in live mode
	}

	if err := t.db.SaveTrade(trade); err != nil {
		t.logger.Trading().WithError(err).Error("Failed to save trade to database")
	}
}

// updateBalanceToDB updates balance to database
func (t *Trader) updateBalanceToDB(ctx context.Context) {
	if t.db == nil {
		return
	}

	balances, err := t.getBalance(ctx)
	if err != nil {
		t.logger.Trading().WithError(err).Error("Failed to get balance for database update")
		return
	}

	for _, bal := range balances {
		if err := t.db.SaveBalance(bal); err != nil {
			t.logger.Trading().WithError(err).
				WithField("currency", bal.Currency).
				Error("Failed to save balance to database")
		}
	}
}

// Helper functions
func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func safeFloat32(ptr *float32) float32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func getCurrencyFromSymbol(symbol string) string {
	// Example: BTC_JPY -> BTC
	if len(symbol) >= 3 {
		return symbol[:3]
	}
	return symbol
}
