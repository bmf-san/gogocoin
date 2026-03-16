package strategy

// ScalpingParams holds configuration parameters for the scalping strategy.
// This is a usecase-layer struct, free from any infrastructure/config dependency.
type ScalpingParams struct {
	EMAFastPeriod  int     `yaml:"ema_fast_period"`
	EMASlowPeriod  int     `yaml:"ema_slow_period"`
	TakeProfitPct  float64 `yaml:"take_profit_pct"`
	StopLossPct    float64 `yaml:"stop_loss_pct"`
	CooldownSec    int     `yaml:"cooldown_sec"`
	MaxDailyTrades int     `yaml:"max_daily_trades"`
	MinNotional    float64 `yaml:"min_notional"`
	FeeRate        float64 `yaml:"fee_rate"`
	// RSI filter (0 = disabled)
	RSIPeriod     int     `yaml:"rsi_period"`
	RSIOverbought float64 `yaml:"rsi_overbought"`
	RSIOversold   float64 `yaml:"rsi_oversold"`
	// Per-symbol parameter overrides
	SymbolParams map[string]ScalpingSymbolOverride `yaml:"symbol_params"`
}

// ScalpingSymbolOverride holds per-symbol overrides for the scalping strategy.
// Zero values mean "use the global default".
type ScalpingSymbolOverride struct {
	EMAFastPeriod int     `yaml:"ema_fast_period"`
	EMASlowPeriod int     `yaml:"ema_slow_period"`
	CooldownSec   int     `yaml:"cooldown_sec"`
	MinNotional   float64 `yaml:"min_notional"`
}
