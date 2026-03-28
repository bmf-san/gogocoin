package scalping

import (
	"context"
	"fmt"

	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// Strategy is a minimal EMA-crossover scalping strategy included in gogocoin
// as a reference implementation. For production use, implement your own
// strategy in a separate module and register it via an init() blank import.
//
// It embeds *strategy.BaseStrategy and satisfies the strategy.Strategy interface.
type Strategy struct {
	*strategy.BaseStrategy

	emaFastPeriod int
	emaSlowPeriod int
	takeProfitPct float64
	stopLossPct   float64
	orderNotional float64
}

// New creates a new Strategy with the given Params.
func New(cfg Params) *Strategy {
	return &Strategy{
		BaseStrategy:  strategy.NewBaseStrategy("scalping", "Minimal EMA crossover scalping strategy", "1.0.0"),
		emaFastPeriod: cfg.EMAFastPeriod,
		emaSlowPeriod: cfg.EMASlowPeriod,
		takeProfitPct: cfg.TakeProfitPct,
		stopLossPct:   cfg.StopLossPct,
		orderNotional: cfg.OrderNotional,
	}
}

// NewDefault creates a Strategy with sensible default parameters.
func NewDefault() *Strategy {
	return New(Params{
		EMAFastPeriod: 9,
		EMASlowPeriod: 21,
		TakeProfitPct: 1.5,
		StopLossPct:   1.0,
		OrderNotional: 1000.0,
	})
}

// ── strategy.Strategy interface ──────────────────────────────────────────────

// Initialize applies config from a map[string]interface{} (YAML-decoded params).
func (s *Strategy) Initialize(config map[string]interface{}) error {
	if config == nil {
		return nil
	}
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
	if v, ok := config["order_notional"].(float64); ok {
		s.orderNotional = v
	}
	return s.validate()
}

// UpdateConfig is an alias for Initialize.
func (s *Strategy) UpdateConfig(config map[string]interface{}) error {
	return s.Initialize(config)
}

// GetConfig returns the current effective configuration.
func (s *Strategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"ema_fast_period": s.emaFastPeriod,
		"ema_slow_period": s.emaSlowPeriod,
		"take_profit_pct": s.takeProfitPct,
		"stop_loss_pct":   s.stopLossPct,
		"order_notional":  s.orderNotional,
	}
}

// GenerateSignal emits BUY when fast EMA > slow EMA, SELL when fast < slow, else HOLD.
func (s *Strategy) GenerateSignal(_ context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}
	if len(history) < s.emaSlowPeriod {
		return s.CreateSignal(data.Symbol, strategy.SignalHold, 0, data.Price, 0,
			map[string]interface{}{"reason": "insufficient_history"}), nil
	}

	emaFast := calcEMA(history, s.emaFastPeriod)
	emaSlow := calcEMA(history, s.emaSlowPeriod)

	action := strategy.SignalHold
	if emaFast > emaSlow {
		action = strategy.SignalBuy
	} else if emaFast < emaSlow {
		action = strategy.SignalSell
	}

	qty := 0.0
	if data.Price > 0 {
		qty = s.orderNotional / data.Price
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

// GetTakeProfitPrice returns the take-profit price from an entry price.
func (s *Strategy) GetTakeProfitPrice(entry float64) float64 {
	return entry * (1.0 + s.takeProfitPct/100.0)
}

// GetStopLossPrice returns the stop-loss price from an entry price.
func (s *Strategy) GetStopLossPrice(entry float64) float64 {
	return entry * (1.0 - s.stopLossPct/100.0)
}

// GetBaseNotional returns the configured order notional.
func (s *Strategy) GetBaseNotional(_ string) float64 {
	return s.orderNotional
}

// GetAutoScaleConfig returns a disabled auto-scale config.
// Auto-scaling is not supported by this simple reference strategy.
func (s *Strategy) GetAutoScaleConfig() strategy.AutoScaleConfig {
	return strategy.AutoScaleConfig{}
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
	if s.orderNotional <= 0 {
		return fmt.Errorf("order_notional must be positive")
	}
	return nil
}

// calcEMA calculates the Exponential Moving Average over history for the given period.
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
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(history); i++ {
		ema = (history[i].Price-ema)*multiplier + ema
	}
	return ema
}
