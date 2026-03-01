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
}
