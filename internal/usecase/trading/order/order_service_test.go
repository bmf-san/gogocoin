package order

import (
	"testing"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func TestBuildOrderRequest(t *testing.T) {
	log := logger.NewNopLogger()

	service := NewOrderService(nil, log, "BTC_JPY")

	tests := []struct {
		name     string
		order    *domain.OrderRequest
		validate func(*testing.T, http.PostV1MeSendchildorderJSONRequestBody)
	}{
		{
			name: "LIMIT BUY order",
			order: &domain.OrderRequest{
				Symbol:       "BTC_JPY",
				Side:         "BUY",
				Type:         "LIMIT",
				Size:         0.01,
				Price:        10000000,
				TimeInForce:  "GTC",
				MinuteToExpire: 60,
			},
			validate: func(t *testing.T, req http.PostV1MeSendchildorderJSONRequestBody) {
				if req.ProductCode != "BTC_JPY" {
					t.Errorf("Expected ProductCode BTC_JPY, got %s", req.ProductCode)
				}
				if req.Side != http.NewOrderRequestSideBUY {
					t.Errorf("Expected Side BUY, got %v", req.Side)
				}
				if req.ChildOrderType != http.NewOrderRequestChildOrderTypeLIMIT {
					t.Errorf("Expected ChildOrderType LIMIT, got %v", req.ChildOrderType)
				}
				if req.Size != 0.01 {
					t.Errorf("Expected Size 0.01, got %f", req.Size)
				}
				if req.Price == nil || *req.Price != 10000000 {
					t.Error("Expected Price 10000000")
				}
				if req.TimeInForce == nil || *req.TimeInForce != http.NewOrderRequestTimeInForceGTC {
					t.Error("Expected TimeInForce GTC")
				}
				if req.MinuteToExpire == nil || *req.MinuteToExpire != 60 {
					t.Error("Expected MinuteToExpire 60")
				}
			},
		},
		{
			name: "MARKET SELL order",
			order: &domain.OrderRequest{
				Symbol: "ETH_JPY",
				Side:   "SELL",
				Type:   "MARKET",
				Size:   0.1,
			},
			validate: func(t *testing.T, req http.PostV1MeSendchildorderJSONRequestBody) {
				if req.ProductCode != "ETH_JPY" {
					t.Errorf("Expected ProductCode ETH_JPY, got %s", req.ProductCode)
				}
				if req.Side != http.NewOrderRequestSideSELL {
					t.Errorf("Expected Side SELL, got %v", req.Side)
				}
				if req.ChildOrderType != http.NewOrderRequestChildOrderTypeMARKET {
					t.Errorf("Expected ChildOrderType MARKET, got %v", req.ChildOrderType)
				}
				if req.Price != nil {
					t.Error("Expected nil Price for MARKET order")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := service.buildOrderRequest(tt.order)
			tt.validate(t, req)
		})
	}
}

func TestBuildOrderResult(t *testing.T) {
	log := logger.NewNopLogger()

	service := NewOrderService(nil, log, "BTC_JPY")

	orderID := "JOR20150101-144122-011111"
	order := &domain.OrderRequest{
		Symbol: "BTC_JPY",
		Side:   "BUY",
		Type:   "LIMIT",
		Size:   0.01,
		Price:  10000000,
	}

	resp := &http.PostV1MeSendchildorderResponse{
		JSON200: &http.ChildOrderResult{
			ChildOrderAcceptanceId: &orderID,
		},
	}

	feeRate := 0.001
	result := service.buildOrderResult(order, resp, feeRate)

	if result.OrderID != orderID {
		t.Errorf("Expected OrderID %s, got %s", orderID, result.OrderID)
	}
	if result.Symbol != "BTC_JPY" {
		t.Errorf("Expected Symbol BTC_JPY, got %s", result.Symbol)
	}
	if result.Side != "BUY" {
		t.Errorf("Expected Side BUY, got %s", result.Side)
	}
	if result.Type != "LIMIT" {
		t.Errorf("Expected Type LIMIT, got %s", result.Type)
	}
	if result.Size != 0.01 {
		t.Errorf("Expected Size 0.01, got %f", result.Size)
	}
	if result.Price != 10000000 {
		t.Errorf("Expected Price 10000000, got %f", result.Price)
	}
	if result.Status != "ACTIVE" {
		t.Errorf("Expected Status ACTIVE, got %s", result.Status)
	}
	if result.FilledSize != 0 {
		t.Errorf("Expected FilledSize 0, got %f", result.FilledSize)
	}
	if result.RemainingSize != 0.01 {
		t.Errorf("Expected RemainingSize 0.01, got %f", result.RemainingSize)
	}
	// Check fee calculation
	expectedFee := 0.01 * 10000000 * 0.001 // size * price * feeRate = 100
	if result.Fee != expectedFee {
		t.Errorf("Expected Fee %f, got %f", expectedFee, result.Fee)
	}
}

func TestSafeString(t *testing.T) {
	tests := []struct {
		name     string
		ptr      *string
		expected string
	}{
		{
			name:     "Non-nil pointer",
			ptr:      stringPtr("test"),
			expected: "test",
		},
		{
			name:     "Nil pointer",
			ptr:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeString(tt.ptr)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSafeFloat32(t *testing.T) {
	tests := []struct {
		name     string
		ptr      *float32
		expected float32
	}{
		{
			name:     "Non-nil pointer",
			ptr:      float32Ptr(123.45),
			expected: 123.45,
		},
		{
			name:     "Nil pointer",
			ptr:      nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeFloat32(tt.ptr)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func float32Ptr(f float32) *float32 {
	return &f
}
