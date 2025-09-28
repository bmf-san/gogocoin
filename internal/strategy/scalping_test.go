package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/config"
)

func TestNewScalping(t *testing.T) {
	t.Parallel()

	cfg := config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	}

	strategy := NewScalping(cfg)

	if strategy == nil {
		t.Fatal("NewScalping returned nil")
	}
	if strategy.Name() != "scalping" {
		t.Errorf("Expected name 'scalping', got '%s'", strategy.Name())
	}
	if strategy.emaFastPeriod != 9 {
		t.Errorf("Expected emaFastPeriod 9, got %d", strategy.emaFastPeriod)
	}
	if strategy.emaSlowPeriod != 21 {
		t.Errorf("Expected emaSlowPeriod 21, got %d", strategy.emaSlowPeriod)
	}
}

func TestScalpingStrategy_ValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    config.ScalpingParams
		wantError bool
	}{
		{
			name: "valid_config",
			config: config.ScalpingParams{
				EMAFastPeriod:  9,
				EMASlowPeriod:  21,
				TakeProfitPct:  0.8,
				StopLossPct:    0.4,
				CooldownSec:    90,
				MaxDailyTrades: 3,
				MinNotional:    200,
				FeeRate:        0.001,
			},
			wantError: false,
		},
		{
			name: "invalid_ema_fast_period",
			config: config.ScalpingParams{
				EMAFastPeriod:  0,
				EMASlowPeriod:  21,
				TakeProfitPct:  0.8,
				StopLossPct:    0.4,
				CooldownSec:    90,
				MaxDailyTrades: 3,
				MinNotional:    200,
				FeeRate:        0.001,
			},
			wantError: true,
		},
		{
			name: "ema_fast_greater_than_slow",
			config: config.ScalpingParams{
				EMAFastPeriod:  21,
				EMASlowPeriod:  9,
				TakeProfitPct:  0.8,
				StopLossPct:    0.4,
				CooldownSec:    90,
				MaxDailyTrades: 3,
				MinNotional:    200,
				FeeRate:        0.001,
			},
			wantError: true,
		},
		{
			name: "negative_take_profit",
			config: config.ScalpingParams{
				EMAFastPeriod:  9,
				EMASlowPeriod:  21,
				TakeProfitPct:  -0.8,
				StopLossPct:    0.4,
				CooldownSec:    90,
				MaxDailyTrades: 3,
				MinNotional:    200,
				FeeRate:        0.001,
			},
			wantError: true,
		},
		{
			name: "invalid_fee_rate",
			config: config.ScalpingParams{
				EMAFastPeriod:  9,
				EMASlowPeriod:  21,
				TakeProfitPct:  0.8,
				StopLossPct:    0.4,
				CooldownSec:    90,
				MaxDailyTrades: 3,
				MinNotional:    200,
				FeeRate:        0.5,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewScalping(tt.config)
			err := strategy.ValidateConfig()

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestScalpingStrategy_GenerateStatelessSignal(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	tests := []struct {
		name     string
		price    float64
		emaFast  float64
		emaSlow  float64
		expected SignalAction
	}{
		{
			name:     "buy_signal",
			price:    10500.0,
			emaFast:  10400.0,
			emaSlow:  10300.0,
			expected: SignalBuy,
		},
		{
			name:     "sell_signal",
			price:    10100.0,
			emaFast:  10200.0,
			emaSlow:  10300.0,
			expected: SignalSell,
		},
		{
			name:     "hold_signal_price_below_ema_fast",
			price:    10100.0,
			emaFast:  10200.0,
			emaSlow:  10100.0,
			expected: SignalHold,
		},
		{
			name:     "hold_signal_price_above_ema_fast_but_ema_cross",
			price:    10300.0,
			emaFast:  10200.0,
			emaSlow:  10250.0,
			expected: SignalHold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal := strategy.generateStatelessSignal(tt.price, tt.emaFast, tt.emaSlow)

			if signal != tt.expected {
				t.Errorf("Expected signal %s, got %s", tt.expected, signal)
			}
		})
	}
}

func TestScalpingStrategy_CalculateEMA(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	// Create test data
	history := make([]MarketData, 21)
	for i := range history {
		history[i] = MarketData{
			Price: float64(10000 + i*10),
		}
	}

	ema := strategy.calculateEMA(history, 9)

	if ema <= 0 {
		t.Errorf("Expected positive EMA, got %f", ema)
	}

	// EMA should be somewhere near the recent prices
	if ema < 10000 || ema > 11000 {
		t.Errorf("EMA out of expected range: %f", ema)
	}
}

func TestScalpingStrategy_CalculateQuantity(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	tests := []struct {
		name     string
		symbol   string
		price    float64
		expected float64 // Approximate expected quantity
	}{
		{
			name:     "btc_price",
			symbol:   "BTC_JPY",
			price:    10000000.0, // 10M JPY per BTC
			expected: 0.001,      // bitFlyer minimum for BTC
		},
		{
			name:     "eth_price",
			symbol:   "ETH_JPY",
			price:    400000.0, // 400K JPY per ETH
			expected: 0.01,     // bitFlyer minimum for ETH
		},
		{
			name:     "xrp_price",
			symbol:   "XRP_JPY",
			price:    100.0, // 100 JPY per XRP
			expected: 2.0,   // 200 / 100 = 2 XRP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty := strategy.calculateQuantity(tt.symbol, tt.price)

			if qty <= 0 {
				t.Errorf("Expected positive quantity, got %f", qty)
			}

			// Check if quantity meets minimum notional
			notional := qty * tt.price
			if notional < strategy.minNotional {
				t.Errorf("Quantity %f at price %f gives notional %f, less than min %f",
					qty, tt.price, notional, strategy.minNotional)
			}
		})
	}
}

func TestScalpingStrategy_CooldownAndDailyLimit(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    2, // 2 seconds for testing
		MaxDailyTrades: 2,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	// Initially not in cooldown
	if strategy.isInCooldown() {
		t.Error("Should not be in cooldown initially")
	}

	// Initially not at daily limit
	if strategy.isDailyLimitReached() {
		t.Error("Should not be at daily limit initially")
	}

	// Record first trade
	strategy.RecordTrade()

	// Should be in cooldown
	if !strategy.isInCooldown() {
		t.Error("Should be in cooldown after trade")
	}

	// Should not be at daily limit (1/2 trades)
	if strategy.isDailyLimitReached() {
		t.Error("Should not be at daily limit after first trade")
	}

	// Wait for cooldown to expire
	time.Sleep(3 * time.Second)

	// Should not be in cooldown anymore
	if strategy.isInCooldown() {
		t.Error("Should not be in cooldown after waiting")
	}

	// Record second trade
	strategy.RecordTrade()

	// Should be at daily limit (2/2 trades)
	if !strategy.isDailyLimitReached() {
		t.Error("Should be at daily limit after second trade")
	}
}

func TestScalpingStrategy_GetTakeProfitStopLoss(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	entryPrice := 10000.0

	tp := strategy.GetTakeProfitPrice(entryPrice)
	sl := strategy.GetStopLossPrice(entryPrice)

	expectedTP := 10000.0 * 1.008 // +0.8%
	expectedSL := 10000.0 * 0.996 // -0.4%

	if tp != expectedTP {
		t.Errorf("Expected TP %f, got %f", expectedTP, tp)
	}
	if sl != expectedSL {
		t.Errorf("Expected SL %f, got %f", expectedSL, sl)
	}
}

func TestScalpingStrategy_GenerateSignalWithInsufficientHistory(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	// Not enough history for EMA calculation
	history := []MarketData{
		{Price: 10000.0},
		{Price: 10010.0},
	}

	data := &MarketData{
		Symbol: "BTC_JPY",
		Price:  10020.0,
	}

	signal, err := strategy.GenerateSignal(context.Background(), data, history)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if signal.Action != SignalHold {
		t.Errorf("Expected HOLD signal with insufficient history, got %s", signal.Action)
	}

	if signal.Metadata["reason"] != "insufficient_history" {
		t.Error("Expected reason to be 'insufficient_history'")
	}
}

func TestScalpingStrategy_Reset(t *testing.T) {
	t.Parallel()

	strategy := NewScalping(config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	})

	// Record some trades
	strategy.RecordTrade()
	strategy.RecordTrade()

	// Reset
	err := strategy.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Check if state is reset
	if !strategy.lastTradeTime.IsZero() {
		t.Error("lastTradeTime should be zero after reset")
	}
	if strategy.dailyTradeCount != 0 {
		t.Error("dailyTradeCount should be 0 after reset")
	}
	if strategy.lastTradeDate != "" {
		t.Error("lastTradeDate should be empty after reset")
	}
}

func TestScalpingStrategy_GetConfig(t *testing.T) {
	t.Parallel()

	config := config.ScalpingParams{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  0.8,
		StopLossPct:    0.4,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		MinNotional:    200,
		FeeRate:        0.001,
	}

	strategy := NewScalping(config)
	retrievedConfig := strategy.GetConfig()

	if retrievedConfig["ema_fast_period"] != 9 {
		t.Error("Config ema_fast_period mismatch")
	}
	if retrievedConfig["ema_slow_period"] != 21 {
		t.Error("Config ema_slow_period mismatch")
	}
	if retrievedConfig["take_profit_pct"] != 0.8 {
		t.Error("Config take_profit_pct mismatch")
	}
	if retrievedConfig["stop_loss_pct"] != 0.4 {
		t.Error("Config stop_loss_pct mismatch")
	}
}
