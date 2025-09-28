package validator

import (
	"testing"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func TestValidateOrder(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	validator := NewOrderValidator(nil, log)

	tests := []struct {
		name        string
		order       *domain.OrderRequest
		expectError bool
	}{
		{
			name:        "Valid BUY order",
			order:       &domain.OrderRequest{Symbol: "BTC_JPY", Side: "BUY", Size: 0.01, Type: "MARKET"},
			expectError: false,
		},
		{
			name:        "Valid SELL order",
			order:       &domain.OrderRequest{Symbol: "ETH_JPY", Side: "SELL", Size: 0.1, Type: "MARKET"},
			expectError: false,
		},
		{
			name:        "Valid LIMIT order",
			order:       &domain.OrderRequest{Symbol: "XRP_JPY", Side: "BUY", Size: 10.0, Type: "LIMIT", Price: 150.0},
			expectError: false,
		},
		{
			name:        "Missing symbol",
			order:       &domain.OrderRequest{Symbol: "", Side: "BUY", Size: 0.01},
			expectError: true,
		},
		{
			name:        "Invalid side",
			order:       &domain.OrderRequest{Symbol: "BTC_JPY", Side: "INVALID", Size: 0.01},
			expectError: true,
		},
		{
			name:        "Zero size",
			order:       &domain.OrderRequest{Symbol: "BTC_JPY", Side: "BUY", Size: 0},
			expectError: true,
		},
		{
			name:        "Negative size",
			order:       &domain.OrderRequest{Symbol: "BTC_JPY", Side: "BUY", Size: -0.01},
			expectError: true,
		},
		{
			name:        "LIMIT order without price",
			order:       &domain.OrderRequest{Symbol: "BTC_JPY", Side: "BUY", Size: 0.01, Type: "LIMIT", Price: 0},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateOrder(tt.order)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGetMinimumOrderSize(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	validator := NewOrderValidator(nil, log)

	tests := []struct {
		symbol   string
		expected float64
	}{
		{"BTC_JPY", 0.001},
		{"ETH_JPY", 0.01},
		{"XRP_JPY", 1.0},
		{"XLM_JPY", 10.0},
		{"MONA_JPY", 0.1},
		{"BCH_JPY", 0.001},
		{"UNKNOWN_JPY", 0.001}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			result := validator.GetMinimumOrderSize(tt.symbol)
			if result != tt.expected {
				t.Errorf("GetMinimumOrderSize(%s) = %f, want %f", tt.symbol, result, tt.expected)
			}
		})
	}
}

func TestCheckBalance(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	validator := NewOrderValidator(nil, log)

	tests := []struct {
		name        string
		order       *domain.OrderRequest
		balances    []domain.Balance
		feeRate     float64
		expectError bool
	}{
		{
			name: "Sufficient JPY for BUY LIMIT order",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   0.01,
				Price:  10000000,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 150000},
			},
			feeRate:     0.001,
			expectError: false,
		},
		{
			name: "Insufficient JPY for BUY LIMIT order",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   0.01,
				Price:  10000000,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 50000},
			},
			feeRate:     0.001,
			expectError: true,
		},
		{
			name: "Sufficient currency for SELL order",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "SELL",
				Type:   "MARKET",
				Size:   0.01,
			},
			balances: []domain.Balance{
				{Currency: "BTC", Available: 0.1},
			},
			feeRate:     0.001,
			expectError: false,
		},
		{
			name: "Insufficient currency for SELL order",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "SELL",
				Type:   "MARKET",
				Size:   0.01,
			},
			balances: []domain.Balance{
				{Currency: "BTC", Available: 0.005},
			},
			feeRate:     0.001,
			expectError: true,
		},
		{
			name: "MARKET BUY order with JPY balance",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "BUY",
				Type:   "MARKET",
				Size:   0.01,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 100000},
			},
			feeRate:     0.001,
			expectError: false,
		},
		{
			name: "MARKET BUY order with no JPY balance",
			order: &domain.OrderRequest{
				Symbol: "BTC_JPY",
				Side:   "BUY",
				Type:   "MARKET",
				Size:   0.01,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 0},
			},
			feeRate:     0.001,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.CheckBalance(tt.order, tt.balances, tt.feeRate)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGetCurrencyFromSymbol(t *testing.T) {
	tests := []struct {
		symbol   string
		expected string
	}{
		{"BTC_JPY", "BTC"},
		{"ETH_JPY", "ETH"},
		{"XRP_JPY", "XRP"},
		{"BTC", "BTC"},
		{"BT", "BT"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			result := getCurrencyFromSymbol(tt.symbol)
			if result != tt.expected {
				t.Errorf("getCurrencyFromSymbol(%s) = %s, want %s", tt.symbol, result, tt.expected)
			}
		})
	}
}
