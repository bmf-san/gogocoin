package scalping

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// MarketSpecService is an optional hook for exchange-provided minimum order sizes.
type MarketSpecService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}

// symbolEMAState tracks per-symbol EMA crossover direction.
type symbolEMAState struct {
	mu                sync.Mutex
	prevFastAboveSlow bool
	initialized       bool
}

// Strategy is a stateless EMA-crossover scalping strategy with optional RSI
// filter and per-symbol parameter overrides.
//
// It embeds *strategy.BaseStrategy and satisfies the strategy.Strategy interface.
type Strategy struct {
	*strategy.BaseStrategy

	emaFastPeriod        int
	emaSlowPeriod        int
	takeProfitPct        float64
	stopLossPct          float64
	cooldownSec          int
	maxDailyTrades       int
	orderNotional        float64
	autoScaleEnabled     bool
	autoScaleBalancePct  float64
	autoScaleMaxNotional float64
	feeRate              float64

	rsiPeriod     int
	rsiOverbought float64
	rsiOversold   float64

	symbolParams map[string]SymbolOverride

	marketSpecSvc MarketSpecService

	lastTradeTime   time.Time
	dailyTradeCount int
	lastTradeDate   string
	mu              sync.RWMutex

	symbolEMAStates sync.Map // map[string]*symbolEMAState
}

// New creates a new scalping Strategy with the given Params.
func New(cfg Params) *Strategy { //nolint:gocritic // hugeParam: Params is passed by value intentionally (immutable config)
	base := strategy.NewBaseStrategy(
		"scalping",
		"Stateless EMA-crossover scalping strategy with RSI filter",
		"2.0.0",
	)

	rsiOverbought := cfg.RSIOverbought
	if rsiOverbought == 0 {
		rsiOverbought = 70.0
	}
	rsiOversold := cfg.RSIOversold
	if rsiOversold == 0 {
		rsiOversold = 30.0
	}
	autoScaleBalancePct := cfg.AutoScaleBalancePct
	if autoScaleBalancePct == 0 {
		autoScaleBalancePct = 80.0
	}

	return &Strategy{
		BaseStrategy:         base,
		emaFastPeriod:        cfg.EMAFastPeriod,
		emaSlowPeriod:        cfg.EMASlowPeriod,
		takeProfitPct:        cfg.TakeProfitPct,
		stopLossPct:          cfg.StopLossPct,
		cooldownSec:          cfg.CooldownSec,
		maxDailyTrades:       cfg.MaxDailyTrades,
		orderNotional:        cfg.OrderNotional,
		autoScaleEnabled:     cfg.AutoScaleEnabled,
		autoScaleBalancePct:  autoScaleBalancePct,
		autoScaleMaxNotional: cfg.AutoScaleMaxNotional,
		feeRate:              cfg.FeeRate,
		rsiPeriod:            cfg.RSIPeriod,
		rsiOverbought:        rsiOverbought,
		rsiOversold:          rsiOversold,
		symbolParams:         cfg.SymbolParams,
	}
}

// NewDefault creates a Strategy with conservative default parameters.
func NewDefault() *Strategy {
	return New(Params{
		EMAFastPeriod:        9,
		EMASlowPeriod:        21,
		TakeProfitPct:        0.8,
		StopLossPct:          0.4,
		CooldownSec:          90,
		MaxDailyTrades:       3,
		OrderNotional:        200.0,
		AutoScaleEnabled:     false,
		AutoScaleBalancePct:  80.0,
		AutoScaleMaxNotional: 0,
		FeeRate:              0.001,
	})
}

// SetMarketSpecService injects an optional exchange-specific order-size service.
func (s *Strategy) SetMarketSpecService(svc MarketSpecService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marketSpecSvc = svc
}

// ── strategy.Strategy interface ──────────────────────────────────────────────

// Initialize applies config from a map[string]interface{} (YAML-decoded params).
func (s *Strategy) Initialize(config map[string]interface{}) error {
	if config == nil {
		return nil
	}
	// Hold the write lock for the duration so that concurrent readers
	// (GetConfig, GenerateSignal, GetTakeProfitPrice, …) see a consistent
	// view. validate() is called while the lock is held; it is safe because
	// it only reads struct fields and does not re-acquire the lock.
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := config["ema_fast_period"].(int); ok {
		s.emaFastPeriod = v
	}
	if v, ok := config["ema_slow_period"].(int); ok {
		s.emaSlowPeriod = v
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
	if v, ok := config["auto_scale_enabled"].(bool); ok {
		s.autoScaleEnabled = v
	}
	if v, ok := config["auto_scale_balance_pct"].(float64); ok {
		s.autoScaleBalancePct = v
	}
	if v, ok := config["auto_scale_max_notional"].(float64); ok {
		s.autoScaleMaxNotional = v
	}
	if v, ok := config["fee_rate"].(float64); ok {
		s.feeRate = v
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
	// Parse per-symbol overrides.  strategyParamsToMap provides
	// map[string]map[string]interface{}; GetConfig/UpdateConfig roundtrip
	// provides map[string]SymbolOverride.  Handle both.
	if raw, ok := config["symbol_params"]; ok {
		switch v := raw.(type) {
		case map[string]map[string]interface{}:
			if len(v) > 0 {
				overrides := make(map[string]SymbolOverride, len(v))
				for sym, m := range v {
					var ov SymbolOverride
					if f, ok2 := m["ema_fast_period"].(int); ok2 {
						ov.EMAFastPeriod = f
					}
					if sl, ok2 := m["ema_slow_period"].(int); ok2 {
						ov.EMASlowPeriod = sl
					}
					if c, ok2 := m["cooldown_sec"].(int); ok2 {
						ov.CooldownSec = c
					}
					if n, ok2 := m["order_notional"].(float64); ok2 {
						ov.OrderNotional = n
					}
					overrides[sym] = ov
				}
				s.symbolParams = overrides
			}
		case map[string]SymbolOverride:
			if len(v) > 0 {
				s.symbolParams = v
			}
		}
	}
	return s.validate()
}

// UpdateConfig is an alias for Initialize.
func (s *Strategy) UpdateConfig(config map[string]interface{}) error {
	return s.Initialize(config)
}

// GetConfig returns the current effective configuration.
func (s *Strategy) GetConfig() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"ema_fast_period":         s.emaFastPeriod,
		"ema_slow_period":         s.emaSlowPeriod,
		"take_profit_pct":         s.takeProfitPct,
		"stop_loss_pct":           s.stopLossPct,
		"cooldown_sec":            s.cooldownSec,
		"max_daily_trades":        s.maxDailyTrades,
		"order_notional":          s.orderNotional,
		"auto_scale_enabled":      s.autoScaleEnabled,
		"auto_scale_balance_pct":  s.autoScaleBalancePct,
		"auto_scale_max_notional": s.autoScaleMaxNotional,
		"fee_rate":                s.feeRate,
		"rsi_period":              s.rsiPeriod,
		"rsi_overbought":          s.rsiOverbought,
		"rsi_oversold":            s.rsiOversold,
		"symbol_params":           s.symbolParams,
	}
}

// GenerateSignal generates a BUY/SELL/HOLD signal with EMA crossover
// confirmation and an optional RSI filter.
func (s *Strategy) GenerateSignal(ctx context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	fastPeriod, slowPeriod := s.symbolEMAPeriods(data.Symbol)

	if len(history) < slowPeriod {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "insufficient_history"}), nil
	}

	emaFast := s.calculateEMA(history, fastPeriod)
	emaSlow := s.calculateEMA(history, slowPeriod)

	if s.isInCooldown() {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "cooldown", "ema_fast": emaFast, "ema_slow": emaSlow}), nil
	}

	if s.isDailyLimitReached() {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "daily_limit", "ema_fast": emaFast, "ema_slow": emaSlow}), nil
	}

	// EMA crossover confirmation – only signal on direction change tick.
	currentFastAbove := emaFast > emaSlow
	stateVal, _ := s.symbolEMAStates.LoadOrStore(data.Symbol, &symbolEMAState{})
	st := stateVal.(*symbolEMAState)
	st.mu.Lock()
	crossoverConfirmed := !st.initialized || (currentFastAbove != st.prevFastAboveSlow)
	st.prevFastAboveSlow = currentFastAbove
	st.initialized = true
	st.mu.Unlock()

	action := s.baseSignal(data.Price, emaFast, emaSlow)

	if action != strategy.SignalHold && !crossoverConfirmed {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "no_new_crossover", "ema_fast": emaFast, "ema_slow": emaSlow}), nil
	}

	// Snapshot RSI config under read lock (these fields may be updated concurrently
	// by UpdateConfig called from the HTTP API handler).
	s.mu.RLock()
	rsiPeriod := s.rsiPeriod
	rsiOverbought := s.rsiOverbought
	rsiOversold := s.rsiOversold
	s.mu.RUnlock()

	// RSI filter
	if action != strategy.SignalHold && rsiPeriod > 0 {
		rsi := s.calculateRSI(history, rsiPeriod)
		if action == strategy.SignalBuy && rsi > rsiOverbought {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "rsi_overbought", "rsi": rsi, "ema_fast": emaFast, "ema_slow": emaSlow}), nil
		}
		if action == strategy.SignalSell && rsi < rsiOversold {
			return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
				map[string]interface{}{"reason": "rsi_oversold", "rsi": rsi, "ema_fast": emaFast, "ema_slow": emaSlow}), nil
		}
	}

	qty := s.quantityForSymbol(data.Symbol, data.Price)
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

// Reset clears all internal state.
func (s *Strategy) Reset() error {
	s.mu.Lock()
	s.lastTradeTime = time.Time{}
	s.dailyTradeCount = 0
	s.lastTradeDate = ""
	s.mu.Unlock()
	s.symbolEMAStates.Range(func(k, _ interface{}) bool {
		s.symbolEMAStates.Delete(k)
		return true
	})
	return s.BaseStrategy.Reset()
}

// RecordTrade records a completed trade (updates cooldown and daily counter).
func (s *Strategy) RecordTrade() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTradeTime = time.Now()
	// Use JST date so the daily counter resets at midnight JST, not UTC midnight.
	// On a UTC VPS, UTC midnight = 09:00 JST which would reset the counter 9 hours
	// early and allow double the intended trades in a single JST day.
	today := time.Now().UTC().Add(9 * time.Hour).Format("2006-01-02")
	if s.lastTradeDate != today {
		s.dailyTradeCount = 1
		s.lastTradeDate = today
	} else {
		s.dailyTradeCount++
	}
}

// InitializeDailyTradeCount seeds today's trade counter from persistent storage.
func (s *Strategy) InitializeDailyTradeCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dailyTradeCount = count
	// Use JST date to be consistent with RecordTrade.
	s.lastTradeDate = time.Now().UTC().Add(9 * time.Hour).Format("2006-01-02")
}

// GetTakeProfitPrice returns the take-profit price from an entry price.
func (s *Strategy) GetTakeProfitPrice(entry float64) float64 {
	s.mu.RLock()
	pct := s.takeProfitPct
	s.mu.RUnlock()
	return entry * (1.0 + pct/100.0)
}

// GetStopLossPrice returns the stop-loss price from an entry price.
func (s *Strategy) GetStopLossPrice(entry float64) float64 {
	s.mu.RLock()
	pct := s.stopLossPct
	s.mu.RUnlock()
	return entry * (1.0 - pct/100.0)
}

// ── internal helpers ─────────────────────────────────────────────────────────

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
	if s.autoScaleEnabled {
		if s.autoScaleBalancePct <= 0 || s.autoScaleBalancePct > 100 {
			return fmt.Errorf("auto_scale_balance_pct must be between 0 and 100 when auto_scale_enabled is true")
		}
		if s.autoScaleMaxNotional > 0 && s.autoScaleMaxNotional < s.orderNotional {
			return fmt.Errorf("auto_scale_max_notional must be >= order_notional when set")
		}
	}
	if s.feeRate < 0 || s.feeRate > 0.1 {
		return fmt.Errorf("fee_rate must be between 0 and 0.1")
	}
	return nil
}

func (s *Strategy) baseSignal(price, emaFast, emaSlow float64) strategy.SignalAction {
	if emaFast > emaSlow && price > emaFast {
		return strategy.SignalBuy
	}
	if emaFast < emaSlow && price < emaFast {
		return strategy.SignalSell
	}
	return strategy.SignalHold
}

func (s *Strategy) calculateEMA(history []strategy.MarketData, period int) float64 {
	if len(history) < period {
		return 0.0
	}
		// Initialize with SMA of the first period points, then apply the EMA
	// formula to every subsequent point.  Using the full history avoids the
	// previous bug where SMA and EMA loop operated on the same slice,
	// effectively double-counting the most-recent prices.
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += history[i].Price
	}
	ema := sum / float64(period)
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(history); i++ {
		ema = (history[i].Price-ema)*multiplier + ema
	}
	return ema
}

func (s *Strategy) calculateRSI(history []strategy.MarketData, period int) float64 {
	if len(history) < period+1 {
		return 50.0
	}
	data := history[len(history)-(period+1):]
	var gains, losses float64
	for i := 1; i < len(data); i++ {
		change := data[i].Price - data[i-1].Price
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		return 100.0
	}
	return 100.0 - (100.0 / (1.0 + avgGain/avgLoss))
}

func (s *Strategy) symbolEMAPeriods(symbol string) (fast, slow int) {
	s.mu.RLock()
	symbolParams := s.symbolParams
	defaultFast := s.emaFastPeriod
	defaultSlow := s.emaSlowPeriod
	s.mu.RUnlock()
	if symbolParams != nil {
		if ov, ok := symbolParams[symbol]; ok {
			if ov.EMAFastPeriod > 0 {
				fast = ov.EMAFastPeriod
			}
			if ov.EMASlowPeriod > 0 {
				slow = ov.EMASlowPeriod
			}
		}
	}
	if fast == 0 {
		fast = defaultFast
	}
	if slow == 0 {
		slow = defaultSlow
	}
	return
}

func (s *Strategy) symbolOrderNotional(symbol string) float64 {
	s.mu.RLock()
	symbolParams := s.symbolParams
	orderNotional := s.orderNotional
	s.mu.RUnlock()
	if symbolParams != nil {
		if ov, ok := symbolParams[symbol]; ok && ov.OrderNotional > 0 {
			return ov.OrderNotional
		}
	}
	return orderNotional
}

func (s *Strategy) quantityForSymbol(symbol string, price float64) float64 {
	if price <= 0 {
		return 0
	}
	notional := s.symbolOrderNotional(symbol)
	lotSize := s.minimumOrderSize(symbol)
	if lotSize <= 0 {
		lotSize = fallbackMinOrderSize(symbol)
	}
	// Round UP to nearest lot to cover at least the required notional
	qty := math.Ceil(notional/price/lotSize) * lotSize
	if qty < lotSize {
		qty = lotSize
	}
	return qty
}

func (s *Strategy) minimumOrderSize(symbol string) float64 {
	s.mu.RLock()
	svc := s.marketSpecSvc
	s.mu.RUnlock()
	if svc != nil {
		if min, err := svc.GetMinimumOrderSize(symbol); err == nil && min > 0 {
			return min
		}
	}
	return fallbackMinOrderSize(symbol)
}

func fallbackMinOrderSize(symbol string) float64 {
	switch symbol {
	case "BTC_JPY":
		return 0.001
	case "ETH_JPY":
		return 0.01
	case "XRP_JPY":
		return 1.0
	case "XLM_JPY":
		return 10.0
	case "MONA_JPY", "ELF_JPY":
		return 1.0
	case "BCH_JPY":
		return 0.01
	default:
		return 0.001
	}
}

func (s *Strategy) isInCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastTradeTime.IsZero() {
		return false
	}
	return time.Since(s.lastTradeTime).Seconds() < float64(s.cooldownSec)
}

func (s *Strategy) isDailyLimitReached() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Use JST date to be consistent with RecordTrade.
	today := time.Now().UTC().Add(9 * time.Hour).Format("2006-01-02")
	if s.lastTradeDate != today {
		return false
	}
	return s.dailyTradeCount >= s.maxDailyTrades
}
