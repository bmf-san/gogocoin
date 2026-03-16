package strategy

import "time"

// StrategyStatus represents the runtime state of a Strategy.
type StrategyStatus struct {
	IsRunning        bool                 `json:"is_running"`
	StartTime        time.Time            `json:"start_time"`
	LastSignalTime   time.Time            `json:"last_signal_time"`
	TotalSignals     int                  `json:"total_signals"`
	SignalsByAction  map[SignalAction]int `json:"signals_by_action"`
	LastError        string               `json:"last_error,omitempty"`
	CurrentPositions map[string]float64   `json:"current_positions"`
}

// StrategyMetrics represents cumulative performance metrics of a Strategy.
type StrategyMetrics struct {
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`

	TotalProfit   float64 `json:"total_profit"`
	AverageProfit float64 `json:"average_profit"`
	MaxProfit     float64 `json:"max_profit"`
	MaxLoss       float64 `json:"max_loss"`

	MaxDrawdown  float64 `json:"max_drawdown"`
	ProfitFactor float64 `json:"profit_factor"`
	SharpeRatio  float64 `json:"sharpe_ratio"`

	Daily   []DailyMetrics   `json:"daily"`
	Monthly []MonthlyMetrics `json:"monthly"`
}

// DailyMetrics represents daily performance metrics.
type DailyMetrics struct {
	Date        time.Time `json:"date"`
	Trades      int       `json:"trades"`
	Profit      float64   `json:"profit"`
	WinRate     float64   `json:"win_rate"`
	MaxDrawdown float64   `json:"max_drawdown"`
}

// MonthlyMetrics represents monthly performance metrics.
type MonthlyMetrics struct {
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	Trades      int     `json:"trades"`
	Profit      float64 `json:"profit"`
	WinRate     float64 `json:"win_rate"`
	MaxDrawdown float64 `json:"max_drawdown"`
}
