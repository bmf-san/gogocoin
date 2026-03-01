package strategy

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	
)

// MarketSpecService provides market specification information
type MarketSpecService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}

// ScalpingStrategy is a minimal stateless scalping strategy
// - Uses only EMA-based signals (no internal position state)
// - Supports 200-300 JPY minimum capital
// - Safe for restarts (position state managed externally)
// - Works from SELL signals (can close existing positions immediately)
type ScalpingStrategy struct {
	*BaseStrategy

	// Configuration
	emaFastPeriod  int     // Fast EMA period (default: 9)
	emaSlowPeriod  int     // Slow EMA period (default: 21)
	takeProfitPct  float64 // Take profit percentage (default: 0.8%)
	stopLossPct    float64 // Stop loss percentage (default: 0.4%)
	cooldownSec    int     // Cooldown between trades (default: 90 seconds)
	maxDailyTrades int     // Maximum trades per day (default: 3)
	minNotional    float64 // Minimum order amount (default: 200 JPY)
	feeRate        float64 // Transaction fee rate (default: 0.001 = 0.1%)

	// Services
	marketSpecSvc MarketSpecService // Optional market specification service

	// Internal state (minimal, for cooldown only)
	lastTradeTime   time.Time
	dailyTradeCount int
	lastTradeDate   string
	mu              sync.RWMutex // Protects internal state
}

// NewScalping creates a new minimal stateless scalping strategy
func NewScalping(cfg ScalpingParams) *ScalpingStrategy {
	base := NewBaseStrategy(
		"scalping",
		"Minimal stateless scalping strategy using EMA crossover",
		"2.0.0",
	)

	return &ScalpingStrategy{
		BaseStrategy:    base,
		emaFastPeriod:   cfg.EMAFastPeriod,
		emaSlowPeriod:   cfg.EMASlowPeriod,
		takeProfitPct:   cfg.TakeProfitPct,
		stopLossPct:     cfg.StopLossPct,
		cooldownSec:     cfg.CooldownSec,
		maxDailyTrades:  cfg.MaxDailyTrades,
		minNotional:     cfg.MinNotional,
		feeRate:         cfg.FeeRate,
		lastTradeTime:   time.Time{},
		dailyTradeCount: 0,
		lastTradeDate:   "",
	}
}

// Initialize initializes the strategy with configuration
func (s *ScalpingStrategy) Initialize(config map[string]interface{}) error {
	if config == nil {
		return nil
	}

	if emaFast, ok := config["ema_fast_period"].(int); ok {
		s.emaFastPeriod = emaFast
	}
	if emaSlow, ok := config["ema_slow_period"].(int); ok {
		s.emaSlowPeriod = emaSlow
	}
	if tp, ok := config["take_profit_pct"].(float64); ok {
		s.takeProfitPct = tp
	}
	if sl, ok := config["stop_loss_pct"].(float64); ok {
		s.stopLossPct = sl
	}
	if cooldown, ok := config["cooldown_sec"].(int); ok {
		s.cooldownSec = cooldown
	}
	if maxTrades, ok := config["max_daily_trades"].(int); ok {
		s.maxDailyTrades = maxTrades
	}
	if minNotional, ok := config["min_notional"].(float64); ok {
		s.minNotional = minNotional
	}
	if feeRate, ok := config["fee_rate"].(float64); ok {
		s.feeRate = feeRate
	}

	return s.ValidateConfig()
}

// UpdateConfig updates the strategy configuration
func (s *ScalpingStrategy) UpdateConfig(config map[string]interface{}) error {
	return s.Initialize(config)
}

// ValidateConfig validates the strategy configuration
func (s *ScalpingStrategy) ValidateConfig() error {
	if s.emaFastPeriod <= 0 {
		return fmt.Errorf("ema_fast_period must be positive")
	}
	if s.emaSlowPeriod <= 0 {
		return fmt.Errorf("ema_slow_period must be positive")
	}
	if s.emaFastPeriod >= s.emaSlowPeriod {
		return fmt.Errorf("ema_fast_period must be less than ema_slow_period")
	}
	if s.takeProfitPct <= 0 {
		return fmt.Errorf("take_profit_pct must be positive")
	}
	if s.stopLossPct <= 0 {
		return fmt.Errorf("stop_loss_pct must be positive")
	}
	if s.cooldownSec < 0 {
		return fmt.Errorf("cooldown_sec must be non-negative")
	}
	if s.maxDailyTrades <= 0 {
		return fmt.Errorf("max_daily_trades must be positive")
	}
	if s.minNotional <= 0 {
		return fmt.Errorf("min_notional must be positive")
	}
	if s.feeRate < 0 || s.feeRate > 0.1 {
		return fmt.Errorf("fee_rate must be between 0 and 0.1")
	}
	return nil
}

// GenerateSignal generates a trading signal (stateless pure function)
// Input: current price, fast EMA, slow EMA
// Output: BUY/SELL/HOLD signal
func (s *ScalpingStrategy) GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// Check if we have enough history for EMA calculation
	if len(history) < s.emaSlowPeriod {
		return s.CreateSignal(data.Symbol, SignalHold, 0.0, data.Price, 0.0, map[string]interface{}{
			"reason": "insufficient_history",
		}), nil
	}

	// Calculate EMAs
	emaFast := s.calculateEMA(history, s.emaFastPeriod)
	emaSlow := s.calculateEMA(history, s.emaSlowPeriod)

	// Check cooldown
	if s.isInCooldown() {
		return s.CreateSignal(data.Symbol, SignalHold, 0.0, data.Price, 0.0, map[string]interface{}{
			"reason":   "cooldown",
			"ema_fast": emaFast,
			"ema_slow": emaSlow,
		}), nil
	}

	// Check daily trade limit
	if s.isDailyLimitReached() {
		return s.CreateSignal(data.Symbol, SignalHold, 0.0, data.Price, 0.0, map[string]interface{}{
			"reason":   "daily_limit",
			"ema_fast": emaFast,
			"ema_slow": emaSlow,
		}), nil
	}

	// Stateless signal generation
	signal := s.generateStatelessSignal(data.Price, emaFast, emaSlow)

	// Calculate quantity based on minimum notional
	quantity := s.calculateQuantity(data.Symbol, data.Price)

	return s.CreateSignal(
		data.Symbol,
		signal,
		1.0, // Full strength for simplicity
		data.Price,
		quantity,
		map[string]interface{}{
			"ema_fast": emaFast,
			"ema_slow": emaSlow,
		},
	), nil
}

// generateStatelessSignal is the core stateless logic
// BUY: ema_fast > ema_slow AND price > ema_fast
// SELL: ema_fast < ema_slow AND price < ema_fast
// HOLD: otherwise
func (s *ScalpingStrategy) generateStatelessSignal(price, emaFast, emaSlow float64) SignalAction {
	if emaFast > emaSlow && price > emaFast {
		return SignalBuy
	}
	if emaFast < emaSlow && price < emaFast {
		return SignalSell
	}
	return SignalHold
}

// calculateEMA calculates exponential moving average
func (s *ScalpingStrategy) calculateEMA(history []MarketData, period int) float64 {
	if len(history) < period {
		return 0.0
	}

	// Use the most recent data points
	data := history[len(history)-period:]

	// Calculate SMA as the initial EMA value
	sum := 0.0
	for i := range data {
		sum += data[i].Price
	}
	ema := sum / float64(period)

	// Calculate multiplier
	multiplier := 2.0 / float64(period+1)

	// Calculate EMA
	for i := 1; i < len(data); i++ {
		ema = (data[i].Price-ema)*multiplier + ema
	}

	return ema
}

// calculateQuantity calculates order quantity based on minimum notional
func (s *ScalpingStrategy) calculateQuantity(symbol string, price float64) float64 {
	if price <= 0 {
		return 0.0
	}

	// Calculate quantity to meet minimum notional
	// notional = price * quantity
	// quantity = min_notional / price (fee is not included in this calculation)
	// We want: notional >= min_notional
	quantity := s.minNotional / price

	// Round to reasonable precision (8 decimals for crypto)
	// Use Ceil to ensure we never fall below min_notional after rounding
	quantity = math.Ceil(quantity*100000000) / 100000000

	// Validate against bitFlyer minimum order sizes
	// These are exchange-specific minimums
	// Use MarketSpecService if available, otherwise fallback to hardcoded values
	minOrderSize := s.getMinimumOrderSize(symbol)
	if quantity < minOrderSize {
		// Calculate quantity based on minimum order size instead
		quantity = minOrderSize
	}

	// Final check: ensure notional meets minimum after all rounding
	notional := quantity * price
	if notional < s.minNotional {
		// Add one tick (smallest precision) to meet the requirement
		quantity += 1.0 / 100000000
	}

	return quantity
}

// SetMarketSpecService sets the market specification service
func (s *ScalpingStrategy) SetMarketSpecService(svc MarketSpecService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marketSpecSvc = svc
}

// GetMinimumOrderSize returns the minimum order size for a given symbol
// This is exported for validation purposes
// Uses MarketSpecificationService if available, otherwise falls back to hardcoded values
func GetMinimumOrderSize(symbol string) float64 {
	return getMinimumOrderSizeFallback(symbol)
}

// getMinimumOrderSizeFallback returns hardcoded minimum order sizes as fallback
func getMinimumOrderSizeFallback(symbol string) float64 {
	// bitFlyer minimum order sizes as of 2025
	switch symbol {
	case "BTC_JPY":
		return 0.001 // 0.001 BTC
	case "ETH_JPY":
		return 0.01 // 0.01 ETH
	case "XRP_JPY":
		return 1.0 // 1 XRP
	case "XLM_JPY":
		return 10.0 // 10 XLM
	case "MONA_JPY":
		return 1.0 // 1 MONA
	case "BCH_JPY":
		return 0.01 // 0.01 BCH
	default:
		return 0.001 // Conservative default
	}
}

// getMinimumOrderSize returns the minimum order size, using MarketSpecService if available
func (s *ScalpingStrategy) getMinimumOrderSize(symbol string) float64 {
	s.mu.RLock()
	svc := s.marketSpecSvc
	s.mu.RUnlock()

	if svc != nil {
		if minSize, err := svc.GetMinimumOrderSize(symbol); err == nil && minSize > 0 {
			return minSize
		}
	}

	// Fallback to hardcoded values
	return getMinimumOrderSizeFallback(symbol)
}

// isInCooldown checks if we're in cooldown period
func (s *ScalpingStrategy) isInCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastTradeTime.IsZero() {
		return false
	}
	elapsed := time.Since(s.lastTradeTime)
	return elapsed.Seconds() < float64(s.cooldownSec)
}

// isDailyLimitReached checks if daily trade limit is reached
func (s *ScalpingStrategy) isDailyLimitReached() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().Format("2006-01-02")

	// If it's a new day, the limit is not reached (counter will be reset on next RecordTrade)
	if s.lastTradeDate != today {
		return false
	}

	return s.dailyTradeCount >= s.maxDailyTrades
}

// RecordTrade records a trade execution (called externally)
func (s *ScalpingStrategy) RecordTrade() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastTradeTime = time.Now()
	today := s.lastTradeTime.Format("2006-01-02")

	if s.lastTradeDate != today {
		s.dailyTradeCount = 1
		s.lastTradeDate = today
	} else {
		s.dailyTradeCount++
	}
}

// InitializeDailyTradeCount initializes the daily trade counter from database on startup
func (s *ScalpingStrategy) InitializeDailyTradeCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	s.dailyTradeCount = count
	s.lastTradeDate = today
}

// GetTakeProfitPrice calculates take profit price from entry
func (s *ScalpingStrategy) GetTakeProfitPrice(entryPrice float64) float64 {
	return entryPrice * (1.0 + s.takeProfitPct/100.0)
}

// GetStopLossPrice calculates stop loss price from entry
func (s *ScalpingStrategy) GetStopLossPrice(entryPrice float64) float64 {
	return entryPrice * (1.0 - s.stopLossPct/100.0)
}

// Analyze analyzes historical data (not used in stateless approach)
func (s *ScalpingStrategy) Analyze(data []MarketData) (*Signal, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}

	// Use the latest data point
	latest := data[len(data)-1]
	return s.GenerateSignal(context.Background(), &latest, data)
}

// Reset resets the strategy state
func (s *ScalpingStrategy) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTradeTime = time.Time{}
	s.dailyTradeCount = 0
	s.lastTradeDate = ""
	return s.BaseStrategy.Reset()
}

// GetConfig returns the current configuration
func (s *ScalpingStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"ema_fast_period":  s.emaFastPeriod,
		"ema_slow_period":  s.emaSlowPeriod,
		"take_profit_pct":  s.takeProfitPct,
		"stop_loss_pct":    s.stopLossPct,
		"cooldown_sec":     s.cooldownSec,
		"max_daily_trades": s.maxDailyTrades,
		"min_notional":     s.minNotional,
		"fee_rate":         s.feeRate,
	}
}
