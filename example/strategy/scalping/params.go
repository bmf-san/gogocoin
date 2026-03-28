package scalping

// Params holds configuration for the example EMA + RSI scalping strategy.
type Params struct {
    EMAFastPeriod  int     `yaml:"ema_fast_period"`
    EMASlowPeriod  int     `yaml:"ema_slow_period"`
    TrendEMAPeriod int     `yaml:"trend_ema_period"`
    RSIPeriod      int     `yaml:"rsi_period"`
    RSIOverbought  float64 `yaml:"rsi_overbought"`
    RSIOversold    float64 `yaml:"rsi_oversold"`
    TakeProfitPct  float64 `yaml:"take_profit_pct"`
    StopLossPct    float64 `yaml:"stop_loss_pct"`
    CooldownSec    int     `yaml:"cooldown_sec"`
    MaxDailyTrades int     `yaml:"max_daily_trades"`
    OrderNotional  float64 `yaml:"order_notional"`
}
