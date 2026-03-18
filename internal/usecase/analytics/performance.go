package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
)

// PerformanceAnalyticsService defines the interface for performance analytics operations
// Following Dependency Inversion Principle, consumers depend on this interface
type PerformanceAnalyticsService interface {
	UpdateMetrics(ctx context.Context) error
}

// PerformanceAnalytics provides performance metrics calculation and tracking
type PerformanceAnalytics struct {
	tradingRepo    TradingRepository
	analyticsRepo  AnalyticsRepository
	logger         logger.LoggerInterface
	initialBalance float64
}

// Verify PerformanceAnalytics implements PerformanceAnalyticsService interface at compile time
var _ PerformanceAnalyticsService = (*PerformanceAnalytics)(nil)

// TradingRepository defines database operations for trades
type TradingRepository interface {
	GetRecentTrades(limit int) ([]domain.Trade, error)
	GetAllTrades() ([]domain.Trade, error)
}

// AnalyticsRepository defines database operations for metrics
type AnalyticsRepository interface {
	SavePerformanceMetric(metric *domain.PerformanceMetric) error
	GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error)
}

// NewPerformanceAnalytics creates a new performance analytics service
func NewPerformanceAnalytics(
	tradingRepo TradingRepository,
	analyticsRepo AnalyticsRepository,
	logger logger.LoggerInterface,
	initialBalance float64,
) *PerformanceAnalytics {
	return &PerformanceAnalytics{
		tradingRepo:    tradingRepo,
		analyticsRepo:  analyticsRepo,
		logger:         logger,
		initialBalance: initialBalance,
	}
}

// UpdateMetrics calculates and updates performance metrics based on recent trades
func (pa *PerformanceAnalytics) UpdateMetrics(ctx context.Context) error {
	if pa.logger != nil {
		pa.logger.System().Debug("Calculating performance metrics")
	}

	// Get all trades for accurate cumulative P&L
	trades, err := pa.tradingRepo.GetAllTrades()
	if err != nil {
		if pa.logger != nil {
			pa.logger.System().WithError(err).Error("Failed to get all trades for performance calculation")
		}
		return fmt.Errorf("failed to get all trades for performance calculation: %w", err)
	}

	if len(trades) == 0 {
		if pa.logger != nil {
			pa.logger.System().Debug("No trades found for performance calculation")
		}
		return nil
	}

	// Calculate performance metrics
	metrics := pa.CalculateFromTrades(trades)

	// Save to database
	if err := pa.analyticsRepo.SavePerformanceMetric(&metrics); err != nil {
		if pa.logger != nil {
			pa.logger.System().WithError(err).Error("Failed to save performance metrics")
		}
		return fmt.Errorf("failed to save performance metrics: %w", err)
	}

	if pa.logger != nil {
		pa.logger.System().Info("Performance metrics updated",
			"total_return", metrics.TotalReturn,
			"win_rate", metrics.WinRate,
			"total_trades", metrics.TotalTrades)
	}

	return nil
}

// CalculateFromTrades calculates comprehensive performance metrics from trade history
func (pa *PerformanceAnalytics) CalculateFromTrades(trades []domain.Trade) domain.PerformanceMetric {
	if len(trades) == 0 {
		return domain.PerformanceMetric{Date: time.Now()}
	}

	var totalPnL float64
	var totalReturn float64
	var winningTrades, losingTrades int
	var totalWin, totalLoss float64
	var maxWin, maxLoss float64
	returns := make([]float64, 0, len(trades)) // Pre-allocate for all trades

	// Calculate PnL for each trade
	for i := range trades {
		trade := &trades[i]

		// Use PnL saved to database
		pnl := trade.PnL

		// Improved handling for PnL = 0 case
		if pnl == 0 {
			switch trade.Side {
			case "BUY":
				// For BUY trades, record only fee as loss
				pnl = -trade.Fee
			case "SELL":
				// For SELL trades with PnL=0, record only fee as loss
				pnl = -trade.Fee
			}
		}

		totalPnL += pnl

		// Improved win/loss determination (based on actual PnL considering fees)
		if pnl > 0.01 { // Profit of 1 yen or more
			winningTrades++
			totalWin += pnl
			if pnl > maxWin {
				maxWin = pnl
			}
		} else if pnl < -0.01 { // Loss of 1 yen or more
			losingTrades++
			totalLoss += -pnl
			if -pnl > maxLoss {
				maxLoss = -pnl
			}
		}
		// Between -0.01 and 0.01 yen is treated as draw (not counted in win/loss)

		// Calculate return rate
		returnRate := pnl / pa.initialBalance
		returns = append(returns, returnRate)
	}

	totalReturn = totalPnL / pa.initialBalance * 100

	// Calculate win rate (only settled trades: wins + losses)
	var winRate float64
	decidedTrades := winningTrades + losingTrades
	totalTrades := len(trades) // Total number of all trades (BUY + SELL)
	if decidedTrades > 0 {
		winRate = float64(winningTrades) / float64(decidedTrades) * 100
	}

	// Calculate average win/loss
	var avgWin, avgLoss float64
	if winningTrades > 0 {
		avgWin = totalWin / float64(winningTrades)
	}
	if losingTrades > 0 {
		avgLoss = totalLoss / float64(losingTrades)
	}

	// Calculate profit factor
	var profitFactor float64
	if totalLoss > 0 {
		profitFactor = totalWin / totalLoss
	}

	// Calculate Sharpe ratio
	sharpeRatio := pa.calculateSharpeRatio(returns, totalReturn)

	// Calculate maximum drawdown
	maxDrawdown := pa.calculateMaxDrawdown(trades)

	return domain.PerformanceMetric{
		Date:          time.Now(),
		TotalReturn:   totalReturn,
		TotalPnL:      totalPnL,
		WinRate:       winRate,
		MaxDrawdown:   maxDrawdown,
		SharpeRatio:   sharpeRatio,
		ProfitFactor:  profitFactor,
		TotalTrades:   totalTrades,
		WinningTrades: winningTrades,
		LosingTrades:  losingTrades,
		AverageWin:    avgWin,
		AverageLoss:   avgLoss,
		LargestWin:    maxWin,
		LargestLoss:   maxLoss,
	}
}

// calculateSharpeRatio calculates Sharpe ratio from returns
func (pa *PerformanceAnalytics) calculateSharpeRatio(returns []float64, totalReturn float64) float64 {
	if len(returns) <= 1 {
		return 0
	}

	mean := totalReturn / float64(len(returns))
	var variance float64
	for _, r := range returns {
		variance += (r*100 - mean) * (r*100 - mean)
	}
	variance /= float64(len(returns) - 1)
	stdDev := math.Sqrt(variance)

	if stdDev > 0 {
		return mean / stdDev
	}
	return 0
}

// calculateMaxDrawdown calculates maximum drawdown from trade history
func (pa *PerformanceAnalytics) calculateMaxDrawdown(trades []domain.Trade) float64 {
	var maxDrawdown float64
	var peak float64
	runningPnL := 0.0

	for i := range trades {
		pnl := trades[i].PnL
		if pnl == 0 && trades[i].Fee > 0 {
			pnl = -trades[i].Fee
		}
		runningPnL += pnl
		if runningPnL > peak {
			peak = runningPnL
		}
		drawdown := (peak - runningPnL) / pa.initialBalance * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}
