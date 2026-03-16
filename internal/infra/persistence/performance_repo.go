package persistence

import (
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// PerformanceRepository implements domain.PerformanceRepository over *DB.
type PerformanceRepository struct{ db *DB }

// NewPerformanceRepository creates a PerformanceRepository backed by db.
func NewPerformanceRepository(db *DB) *PerformanceRepository {
	return &PerformanceRepository{db: db}
}

// Compile-time check.
var _ domain.PerformanceRepository = (*PerformanceRepository)(nil)

// SavePerformanceMetric inserts a performance metric record.
func (r *PerformanceRepository) SavePerformanceMetric(metric *domain.PerformanceMetric) error {
	if metric.Date.IsZero() {
		metric.Date = time.Now()
	}
	query := `INSERT INTO performance_metrics (date, total_return, daily_return, win_rate,
			  max_drawdown, sharpe_ratio, profit_factor, total_trades, winning_trades,
			  losing_trades, average_win, average_loss, largest_win, largest_loss,
			  consecutive_wins, consecutive_loss, total_pnl)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.db.Exec(query, metric.Date, metric.TotalReturn, metric.DailyReturn,
		metric.WinRate, metric.MaxDrawdown, metric.SharpeRatio, metric.ProfitFactor,
		metric.TotalTrades, metric.WinningTrades, metric.LosingTrades, metric.AverageWin,
		metric.AverageLoss, metric.LargestWin, metric.LargestLoss, metric.ConsecutiveWins,
		metric.ConsecutiveLoss, metric.TotalPnL)
	return err
}

// GetLatestPerformanceMetric returns the most recent performance metric.
func (r *PerformanceRepository) GetLatestPerformanceMetric() (*domain.PerformanceMetric, error) {
	query := `SELECT id, date, total_return, daily_return, win_rate, max_drawdown,
			  sharpe_ratio, profit_factor, total_trades, winning_trades, losing_trades,
			  average_win, average_loss, largest_win, largest_loss, consecutive_wins,
			  consecutive_loss, total_pnl FROM performance_metrics ORDER BY date DESC LIMIT 1`
	row := r.db.db.QueryRow(query)
	var m domain.PerformanceMetric
	var id int
	err := row.Scan(&id, &m.Date, &m.TotalReturn, &m.DailyReturn, &m.WinRate,
		&m.MaxDrawdown, &m.SharpeRatio, &m.ProfitFactor, &m.TotalTrades,
		&m.WinningTrades, &m.LosingTrades, &m.AverageWin, &m.AverageLoss,
		&m.LargestWin, &m.LargestLoss, &m.ConsecutiveWins, &m.ConsecutiveLoss,
		&m.TotalPnL)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetPerformanceMetrics returns metrics for the past days (0 = all).
func (r *PerformanceRepository) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {
	var (
		query string
		args  []interface{}
	)
	if days > 0 {
		query = `SELECT id, date, total_return, daily_return, win_rate, max_drawdown,
				 sharpe_ratio, profit_factor, total_trades, winning_trades, losing_trades,
				 average_win, average_loss, largest_win, largest_loss, consecutive_wins,
				 consecutive_loss, total_pnl FROM performance_metrics
				 WHERE date >= datetime('now', '-' || ? || ' days') ORDER BY date DESC`
		args = append(args, days)
	} else {
		query = `SELECT id, date, total_return, daily_return, win_rate, max_drawdown,
				 sharpe_ratio, profit_factor, total_trades, winning_trades, losing_trades,
				 average_win, average_loss, largest_win, largest_loss, consecutive_wins,
				 consecutive_loss, total_pnl FROM performance_metrics ORDER BY date DESC`
	}
	rows, err := r.db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	metrics := make([]domain.PerformanceMetric, 0, 100)
	for rows.Next() {
		var m domain.PerformanceMetric
		var id int
		if err := rows.Scan(&id, &m.Date, &m.TotalReturn, &m.DailyReturn, &m.WinRate,
			&m.MaxDrawdown, &m.SharpeRatio, &m.ProfitFactor, &m.TotalTrades,
			&m.WinningTrades, &m.LosingTrades, &m.AverageWin, &m.AverageLoss,
			&m.LargestWin, &m.LargestLoss, &m.ConsecutiveWins, &m.ConsecutiveLoss,
			&m.TotalPnL); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}
