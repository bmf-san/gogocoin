package scalping

// Params holds configuration for the simple EMA crossover scalping strategy.
type Params struct {
	EMAFastPeriod int     `yaml:"ema_fast_period"`
	EMASlowPeriod int     `yaml:"ema_slow_period"`
	TakeProfitPct float64 `yaml:"take_profit_pct"`
	StopLossPct   float64 `yaml:"stop_loss_pct"`
	OrderNotional float64 `yaml:"order_notional"`
}
