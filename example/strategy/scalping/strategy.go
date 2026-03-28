package scalping

import (
	"context"
	"fmt"
	"sync"
	"time"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// Strategy is an EMA crossover strategy with optional RSI and trend filters,
// cooldown, and daily trade limits. It demonstrates how to implement a
// production-grade strategy on top of the gogocoin framework.
type Strategy struct {
	*strategy.BaseStrategy

	emaFastPeriod  int
	emaSlowPeriod  int
	trendEMAPeriod int
	rsiPeriod      int
	rsiOverbought  float64
	rsiOversold    float64
	takeProfitPct  float64
	stopLossPct    float64
	cooldownSec    int
	maxDailyTrades int
	orderNotional  float64

	mu              sync.RWMutex
	lastTradeTime   time.Time
	dailyTradeCount int
	lastTradeDate   string
}

// New creates a Strategy from Params.
func New(p Params) *Strategy {
	rsiOverbought := p.RSIOverbought
	if rsiOverbought == 0 {
		rsiOverbought = 70.0
	}
	rsiOversold := p.RSIOversold
	if rsiOversold == 0 {
		rsiOversold = 30.0
	}
	return &Strategy{
		BaseStrategy:   strategy.NewBaseStrategy("scalping", "EMA crossover with RSI and trend filters", "1.0.0"),
		emaFastPeriod:  p.EMAFastPeriod,
		emaSlowPeriod:  p.EMASlowPeriod,
		trendEMAPeriod: p.TrendEMAPeriod,
		rsiPeriod:      p.RSIPeriod,
		rsiOverbought:  rsiOverbought,
		rsiOversold:    rsiOversold,
		takeProfitPct:  p.TakeProfitPct,
		stopLossPct:    p.StopLossPct,
		cooldownSec:    p.CooldownSec,
		maxDailyTrades: p.MaxDailyTrades,
		orderNotional:  p.OrderNotional,
	}
}

// NewDefault returns a Strategy with conservative defaults.
func NewDefault() *Strategy {
	return New(Params{
		EMAFastPeriod:  9,
		EMASlowPeriod:  21,
		TakeProfitPct:  1.5,
		StopLossPct:    1.0,
		CooldownSec:    90,
		MaxDailyTrades: 3,
		OrderNotional:  1000.0,
	})
}

// Initialize applies config from a map[string]interface{} (YAML strategy_params block).
func (s *Strategy) Initialize(config map[string]interface{}) error {
	if config == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := config["ema_fast_period"].(int); ok {
		s.emaFastPeriod = v
	}
	if v, ok := config["ema_slow_period"].(int); ok {
		s.emaSlowPeriod = v
	}
	if v, ok := config["trend_ema_period"].(int); ok {
		s.trendEMAPeriod = v
	}
	if v, ok := config["rsi_period"].(int); ok {
		s.rsiPeriod = v
	}
	if v, ok := config["rsi_overbought"].(float64); ok {
		s.rsiOverbought = v
	}
	if v, ok := config["rsi_oversold"].(float64); ok {
		s.rsiOversold = v
	}
	if v, ok := config["take_profit_pct"].(float64); ok {
		s.takeProfitPct = v
	}
	if v, ok := config["stop_loss_pct"].(float64); ok {
		s.stopLossPct = v
	}
	if v, ok := config["cooldown_sec"].(int); ok {
		s.cooldownSec = v
	}
	if v, ok := config["max_daily_trades"].(int); ok {
		s.maxDailyTrades = v
	}
	if v, ok := config["order_notional"].(float64); ok {
		s.orderNotional = v
	}
	return s.validate()
}

// UpdateConfig is an alias for Initialize.
func (s *Strategy) UpdateConfig(config map[string]interface{}) error {
	return s.Initialize(config)
}

// GetConfig returns the current configuration as a map.
func (s *Strategy) GetConfig() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"ema_fast_period":  s.emaFastPeriod,
		"ema_slow_period":  s.emaSlowPeriod,
		"trend_ema_period": s.trendEMAPeriod,
		"rsi_period":       s.rsiPeriod,
		"rsi_overbought":   s.rsiOverbought,
		"rsi_oversold":     s.rsiOversold,
		"take_profit_pct":  s.takeProfitPct,
		"stop_loss_pct":    s.stopLossPct,
		"cooldown_sec":     s.cooldownSec,
		"max_daily_trades": s.maxDailyTrades,
		"order_notional":   s.orderNotional,
	}
}

// GenerateSignal emits a BUY/SELL/HOLD signal via EMA crossover with optional
// trend EMA and RSI filters, cooldown, and daily trade limit.
func (s *Strategy) GenerateSignal(_ context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	s.mu.RLock()
	fastPeriod := s.emaFastPeriod
	slowPeriod := s.emaSlowPeriod
	trendPeriod := s.trendEMAPeriod
	rsiPeriod := s.rsiPeriod
	rsiOverbought := s.rsiOverbought
	rsiOversold := s.rsiOversold
	cooldownSec := s.cooldownSec
	maxDailyTrades := s.maxDailyTrades
	orderNotional := s.orderNotional
	s.mu.RUnlock()

	if len(history) < slowPeriod {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "insufficient_history"}), nil
	}

	emaFast := calcEMA(history, fastPeriod)
	emaSlow := calcEMA(history, slowPeriod)

	if s.isInCooldown(cooldownSec) {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "cooldown"}), nil
	}
	if s.isDailyLimitReached(maxDailyTrades) {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "daily_limit"}), nil
	}

	action := strategy.SignalHold
	if emaFast > emaSlow {
		action = strategy.SignalBuy
	} else if emaFast < emaSlow {
		action = strategy.SignalSell
	}

	// Trend filter: suppress BUY when price is below the long-term EMA.
	if action == strategy.SignalBuy && trendPeriod > 0 {
		if len(history) < trendPeriod {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "trend_filter_insufficient_history"}), nil
		}
		emaTrend := calcEMA(history, trendPeriod)
		if data.Price < emaTrend {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "trend_filter", "ema_trend": emaTrend}), nil
		}
	}

	// RSI filter.
	if action != strategy.SignalHold && rsiPeriod > 0 {
		rsi := calcRSI(history, rsiPeriod)
		if action == strategy.SignalBuy && rsi > rsiOverbought {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "rsi_overbought", "rsi": rsi}), nil
		}
		if action == strategy.SignalSell && rsi < rsiOversold {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "rsi_oversold", "rsi": rsi}), nil
		}
	}

	qty := 0.0
	if data.Price > 0 {
		qty = orderNotional / data.Price
	}
	return s.CreateSignal(data.Symbol, action, 1.0, data.Price, qty,
		map[string]interface{}{"ema_fast": emaFast, "ema_slow": emaSlow}), nil
}

// Analyze generates a signal from a batch of historical data.
func (s *Strategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}
	latest := data[len(data)-1]
	return s.GenerateSignal(context.Background(), &latest, data)
}

// RecordTrade updates cooldown and daily trade counter.
func (s *Strategy) RecordTrade() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTradeTime = time.Now()
	today := jstToday()
	if s.lastTradeDate != today {
		s.dailyTradeCount = 1
		s.lastTradeDate = today
	} else {
		s.dailyTradeCount++
	}
}

// InitializeDailyTradeCount seeds today's trade counter from persistent storage.
// Does NOT set lastTradeTime so the cooldown timer is not activated on startup.
func (s *Strategy) InitializeDailyTradeCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dailyTradeCount = count
	s.lastTradeDate = jstToday()
}

// Reset clears all runtime state.
func (s *Strategy) Reset() error {
	s.mu.Lock()
	s.lastTradeTime = time.Time{}
	s.dailyTradeCount = 0
	s.lastTradeDate = ""
	s.mu.Unlock()
	return s.BaseStrategy.Reset()
}

// GetTakeProfitPrice returns the take-profit exit price for a long entered at entry.
func (s *Strategy) GetTakeProfitPrice(entry float64) float64 {
	s.mu.RLock()
	pct := s.takeProfitPct
	s.mu.RUnlock()
	return entry * (1.0 + pct/100.0)
}

// GetStopLossPrice returns the stop-loss exit price for a long entered at entry.
func (s *Strategy) GetStopLossPrice(entry float64) float64 {
	s.mu.RLock()
	pct := s.stopLossPct
	s.mu.RUnlock()
	return entry * (1.0 - pct/100.0)
}

// GetBaseNotional returns the configured order notional in JPY.
func (s *Strategy) GetBaseNotional(_ string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orderNotional
}

// GetAutoScaleConfig returns a disabled auto-scale config.
// Extend this method to enable balance-proportional order sizing.
func (s *Strategy) GetAutoScaleConfig() strategy.AutoScaleConfig {
	return strategy.AutoScaleConfig{}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *Strategy) validate() error {
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
	if s.orderNotional <= 0 {
		return fmt.Errorf("order_notional must be positive")
	}
	return nil
}

func (s *Strategy) isInCooldown(cooldownSec int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastTradeTime.IsZero() || cooldownSec == 0 {
		return false
	}
	return time.Since(s.lastTradeTime).Seconds() < float64(cooldownSec)
}

func (s *Strategy) isDailyLimitReached(maxDailyTrades int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastTradeDate != jstToday() {
		return false
	}
	return s.dailyTradeCount >= maxDailyTrades
}

// jstToday returns today's date string in JST (UTC+9).
// Counters reset at midnight JST, not UTC, to align with the Japanese trading day.
func jstToday() string {
	return time.Now().UTC().Add(9 * time.Hour).Format("2006-01-02")
}

// calcEMA computes the Exponential Moving Average for the given period.
// Initializes with the SMA of the first period prices, then applies the EMA formula.
func calcEMA(history []strategy.MarketData, period int) float64 {
	if len(history) < period {
		return 0.0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += history[i].Price
	}
	ema := sum / float64(period)
	k := 2.0 / float64(period+1)
	for i := period; i < len(history); i++ {
		ema = (history[i].Price-ema)*k + ema
	}
	return ema
}

// calcRSI computes the Relative Strength Index for the given period.
func calcRSI(history []strategy.MarketData, period int) float64 {
	if len(history) < period+1 {
		return 50.0
	}
	slice := history[len(history)-(period+1):]
	var gains, losses float64
	for i := 1; i < len(slice); i++ {
		delta := slice[i].Price - slice[i-1].Price
		if delta > 0 {
			gains += delta
		} else {
			losses -= delta
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		return 100.0
	}
	return 100.0 - (100.0 / (1.0 + avgGain/avgLoss))
}
