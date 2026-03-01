package domain

import "time"

// PerformanceMetric represents performance metrics
type PerformanceMetric struct {
	Date            time.Time `json:"date"`
	TotalReturn     float64   `json:"total_return"`
	DailyReturn     float64   `json:"daily_return"`
	WinRate         float64   `json:"win_rate"`
	MaxDrawdown     float64   `json:"max_drawdown"`
	SharpeRatio     float64   `json:"sharpe_ratio"`
	ProfitFactor    float64   `json:"profit_factor"`
	TotalTrades     int       `json:"total_trades"`
	WinningTrades   int       `json:"winning_trades"`
	LosingTrades    int       `json:"losing_trades"`
	AverageWin      float64   `json:"average_win"`
	AverageLoss     float64   `json:"average_loss"`
	LargestWin      float64   `json:"largest_win"`
	LargestLoss     float64   `json:"largest_loss"`
	ConsecutiveWins int       `json:"consecutive_wins"`
	ConsecutiveLoss int       `json:"consecutive_loss"`
	TotalPnL        float64   `json:"total_pnl"`
}
