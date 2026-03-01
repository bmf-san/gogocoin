package config

import (
	"testing"
)

func TestValidateRiskManagement(t *testing.T) {
	validator := NewValidator(nil)

	tests := []struct {
		name        string
		rm          *RiskManagementConfig
		expectError bool
	}{
		{
			name: "Valid config",
			rm: &RiskManagementConfig{
				MaxTradeAmountPercent: 10.0,
				MaxTotalLossPercent:   50.0,
				MaxDailyTrades:        10,
				MinTradeInterval:      "5m",
			},
			expectError: false,
		},
		{
			name: "Invalid max trade amount - too high",
			rm: &RiskManagementConfig{
				MaxTradeAmountPercent: 150.0,
				MaxTotalLossPercent:   50.0,
				MaxDailyTrades:        10,
			},
			expectError: true,
		},
		{
			name: "Invalid max trade amount - zero",
			rm: &RiskManagementConfig{
				MaxTradeAmountPercent: 0,
				MaxTotalLossPercent:   50.0,
				MaxDailyTrades:        10,
			},
			expectError: true,
		},
		{
			name: "Invalid max total loss - too high",
			rm: &RiskManagementConfig{
				MaxTradeAmountPercent: 10.0,
				MaxTotalLossPercent:   150.0,
				MaxDailyTrades:        10,
			},
			expectError: true,
		},
		{
			name: "Invalid max daily trades - zero",
			rm: &RiskManagementConfig{
				MaxTradeAmountPercent: 10.0,
				MaxTotalLossPercent:   50.0,
				MaxDailyTrades:        0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRiskManagement(tt.rm)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateRiskManagement() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestValidateSymbol_UnknownSymbol(t *testing.T) {
	validator := NewValidator(nil)

	cfg := &Config{
		Trading: TradingConfig{
			Symbols: []string{"UNKNOWN_JPY"},
			Strategy: StrategyConfig{
				Name: "scalping",
			},
		},
	}

	// Should not error for unknown symbol (just skips validation)
	err := validator.ValidateSymbol(cfg, "UNKNOWN_JPY")
	if err != nil {
		t.Errorf("ValidateSymbol() for unknown symbol should not error, got: %v", err)
	}
}

func TestExtractCurrency(t *testing.T) {
	tests := []struct {
		symbol   string
		expected string
	}{
		{"BTC_JPY", "BTC"},
		{"ETH_JPY", "ETH"},
		{"XRP_JPY", "XRP"},
		{"BTC", "BTC"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			result := extractCurrency(tt.symbol)
			if result != tt.expected {
				t.Errorf("extractCurrency(%s) = %s, want %s", tt.symbol, result, tt.expected)
			}
		})
	}
}

func TestValidateTradingConfig(t *testing.T) {
	validator := NewValidator(nil)

	cfg := &Config{
		Trading: TradingConfig{
			Symbols: []string{"UNKNOWN_JPY"}, // Use unknown symbol to skip actual validation
			Strategy: StrategyConfig{
				Name: "scalping",
			},
		},
	}

	err := validator.ValidateTradingConfig(cfg)
	if err != nil {
		t.Errorf("ValidateTradingConfig() failed: %v", err)
	}
}
