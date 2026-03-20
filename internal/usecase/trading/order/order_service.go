package order

import (
	"context"
	"fmt"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/logger"
)

const (
	// DefaultOrderLimit defines the default number of orders to retrieve
	DefaultOrderLimit = 100
)

// OrderService manages order execution via the bitFlyer API
type OrderService struct {
	client *bitflyer.Client
	logger logger.LoggerInterface
	symbol string
}

// NewOrderService creates a new OrderService
func NewOrderService(client *bitflyer.Client, logger logger.LoggerInterface, symbol string) *OrderService {
	return &OrderService{
		client: client,
		logger: logger,
		symbol: symbol,
	}
}

// PlaceOrder executes an order via the API
func (s *OrderService) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	httpClient := s.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	// Build order request
	requestBody := s.buildOrderRequest(order)

	// Wait for rate limiter before making API request
	if err := s.client.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Call API with retry logic
	var resp *http.PostV1MeSendchildorderResponse

	retryErr := s.client.WithRetry(ctx, "PlaceOrder", func() error {
		callStart := time.Now()
		var err error
		resp, err = httpClient.PostV1MeSendchildorderWithResponse(ctx, requestBody)
		callDuration := time.Since(callStart).Milliseconds()

		if err != nil {
			s.logger.LogAPICall("POST", "/v1/me/sendchildorder", callDuration, 0, err)
			return err
		}

		s.logger.LogAPICall("POST", "/v1/me/sendchildorder", callDuration, resp.HTTPResponse.StatusCode, nil)

		if resp.HTTPResponse.StatusCode != 200 {
			return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
		}

		if resp.JSON200 == nil {
			return fmt.Errorf("empty response body")
		}

		return nil
	})

	if retryErr != nil {
		return nil, fmt.Errorf("failed to place order after retries: %w", retryErr)
	}

	// Build order result
	feeRate := s.client.GetFeeRate()
	result := s.buildOrderResult(order, resp, feeRate)
	return result, nil
}

// CancelOrder cancels an order
func (s *OrderService) CancelOrder(ctx context.Context, orderID string) error {
	// Wait for rate limiter before making API request
	if err := s.client.WaitForRateLimit(ctx); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	httpClient := s.client.GetHTTPClient()
	if httpClient == nil {
		return fmt.Errorf("HTTP client not initialized")
	}

	// Cancel request
	requestBody := http.PostV1MeCancelchildorderJSONRequestBody{
		ChildOrderAcceptanceId: &orderID,
	}

	// Call API with retry logic
	retryErr := s.client.WithRetry(ctx, "CancelOrder", func() error {
		start := time.Now()
		resp, err := httpClient.PostV1MeCancelchildorderWithResponse(ctx, requestBody)
		duration := time.Since(start).Milliseconds()

		if err != nil {
			s.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, 0, err)
			return err
		}

		s.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, resp.HTTPResponse.StatusCode, nil)

		if resp.HTTPResponse.StatusCode != 200 {
			return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
		}

		return nil
	})

	if retryErr != nil {
		return fmt.Errorf("failed to cancel order after retries: %w", retryErr)
	}

	s.logger.Trading().WithField("order_id", orderID).Info("Order canceled successfully")
	return nil
}

// GetOrders retrieves the list of orders for the default symbol (s.symbol).
func (s *OrderService) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return s.getOrdersForSymbol(ctx, s.symbol)
}

// GetOrdersBySymbol retrieves orders for an explicit symbol.
// Use this instead of GetOrders when monitoring orders placed for symbols
// other than the trader's default symbol (Symbols[0]).
func (s *OrderService) GetOrdersBySymbol(ctx context.Context, symbol string) ([]*domain.OrderResult, error) {
	return s.getOrdersForSymbol(ctx, symbol)
}

func (s *OrderService) getOrdersForSymbol(ctx context.Context, symbol string) ([]*domain.OrderResult, error) {
	// Wait for rate limiter before making API request
	if err := s.client.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	httpClient := s.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	// Get last orders
	count := http.Count(DefaultOrderLimit)
	params := http.GetV1MeGetchildordersParams{
		ProductCode: symbol,
		Count:       &count,
	}

	// Call API with retry logic
	var resp *http.GetV1MeGetchildordersResponse

	retryErr := s.client.WithRetry(ctx, "GetOrders", func() error {
		callStart := time.Now()
		var err error
		resp, err = httpClient.GetV1MeGetchildordersWithResponse(ctx, &params)
		callDuration := time.Since(callStart).Milliseconds()

		if err != nil {
			s.logger.LogAPICall("GET", "/v1/me/getchildorders", callDuration, 0, err)
			return err
		}

		s.logger.LogAPICall("GET", "/v1/me/getchildorders", callDuration, resp.HTTPResponse.StatusCode, nil)

		if resp.HTTPResponse.StatusCode != 200 {
			return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
		}

		if resp.JSON200 == nil {
			return fmt.Errorf("empty response body")
		}

		return nil
	})

	if retryErr != nil {
		return nil, fmt.Errorf("failed to get orders after retries: %w", retryErr)
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

		// Extract commission/fee from API response
		var commission float64
		if order.TotalCommission != nil {
			commission = float64(*order.TotalCommission)
		}

		orderResult := &domain.OrderResult{
			OrderID:         safeString(order.ChildOrderAcceptanceId),
			Symbol:          safeString(order.ProductCode),
			Side:            side,
			Type:            orderType,
			Size:            float64(safeFloat32(order.Size)),
			Price:           float64(safeFloat32(order.Price)),
			Status:          status,
			FilledSize:      float64(safeFloat32(order.ExecutedSize)),
			RemainingSize:   float64(safeFloat32(order.OutstandingSize)),
			AveragePrice:    float64(safeFloat32(order.AveragePrice)),
			TotalCommission: commission,
			Fee:             commission,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}

		orders = append(orders, orderResult)
	}

	s.logger.Trading().Info("Retrieved orders", "count", len(orders))
	return orders, nil
}

// buildOrderRequest builds an API request body from a domain order request
func (s *OrderService) buildOrderRequest(order *domain.OrderRequest) http.PostV1MeSendchildorderJSONRequestBody {
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

	return requestBody
}

// buildOrderResult builds an OrderResult from API response and original request
func (s *OrderService) buildOrderResult(order *domain.OrderRequest, resp *http.PostV1MeSendchildorderResponse, feeRate float64) *domain.OrderResult {
	var orderID string
	if resp.JSON200.ChildOrderAcceptanceId != nil {
		orderID = *resp.JSON200.ChildOrderAcceptanceId
	}

	// Calculate estimated fee
	estimatedFee := order.Size * order.Price * feeRate

	return &domain.OrderResult{
		OrderID:       orderID,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          order.Type,
		Size:          order.Size,
		Price:         order.Price,
		Status:        "ACTIVE",
		FilledSize:    0,
		RemainingSize: order.Size,
		Fee:           estimatedFee,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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
