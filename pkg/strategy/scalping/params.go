package scalping

// Params holds all configuration for the scalping strategy.
type Params struct {
	EMAFastPeriod        int                       `yaml:"ema_fast_period"`
	EMASlowPeriod        int                       `yaml:"ema_slow_period"`
	// TrendEMAPeriod is the period of the long-term trend EMA used as a trend
	// filter. BUY signals are only emitted when the current price is above this
	// EMA, indicating an overall uptrend. Set to 0 to disable (default).
	TrendEMAPeriod       int                       `yaml:"trend_ema_period"`
	TakeProfitPct        float64                   `yaml:"take_profit_pct"`
	StopLossPct          float64                   `yaml:"stop_loss_pct"`
	CooldownSec          int                       `yaml:"cooldown_sec"`
	MaxDailyTrades       int                       `yaml:"max_daily_trades"`
	OrderNotional        float64                   `yaml:"order_notional"`
	AutoScaleEnabled     bool                      `yaml:"auto_scale_enabled"`
	AutoScaleBalancePct  float64                   `yaml:"auto_scale_balance_pct"`
	AutoScaleMaxNotional float64                   `yaml:"auto_scale_max_notional"`
	FeeRate              float64                   `yaml:"fee_rate"`
	RSIPeriod            int                       `yaml:"rsi_period"`
	RSIOverbought        float64                   `yaml:"rsi_overbought"`
	RSIOversold          float64                   `yaml:"rsi_oversold"`
	SymbolParams         map[string]SymbolOverride `yaml:"symbol_params"`
}

// SymbolOverride holds per-symbol overrides; zero values fall back to global defaults.
type SymbolOverride struct {
	EMAFastPeriod int     `yaml:"ema_fast_period"`
	EMASlowPeriod int     `yaml:"ema_slow_period"`
	CooldownSec   int     `yaml:"cooldown_sec"`
	OrderNotional float64 `yaml:"order_notional"`
}
