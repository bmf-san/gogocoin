package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	"github.com/bmf-san/gogocoin/internal/usecase/trading"
	"github.com/bmf-san/gogocoin/internal/utils"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// RiskManager defines the interface for risk management operations
// Following Dependency Inversion Principle, consumers depend on this interface
type RiskManager interface {
	CheckRiskManagement(ctx context.Context, signal *strategy.Signal) error
}

// Manager provides risk management functionality for trading operations
// Enforces trading limits, intervals, and loss protection rules
type Manager struct {
	cfg           ManagerConfig
	tradingRepo   TradingRepository
	analyticsRepo AnalyticsRepository
	trader        trading.Trader
	logger        logger.LoggerInterface
}

// Verify Manager implements RiskManager interface at compile time
var _ RiskManager = (*Manager)(nil)

// TradingRepository defines database operations needed for risk checks
type TradingRepository interface {
	GetRecentTrades(limit int) ([]domain.Trade, error)
}

// AnalyticsRepository defines analytics operations needed for risk checks
type AnalyticsRepository interface {
	GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error)
}

// NewRiskManager creates a new risk manager
func NewRiskManager(
	cfg ManagerConfig,
	tradingRepo TradingRepository,
	analyticsRepo AnalyticsRepository,
	trader trading.Trader,
	log logger.LoggerInterface,
) *Manager {
	return &Manager{
		cfg:           cfg,
		tradingRepo:   tradingRepo,
		analyticsRepo: analyticsRepo,
		trader:        trader,
		logger:        log,
	}
}

// CheckRiskManagement performs all risk management checks before placing an order
// Returns error if any risk rule is violated
func (rm *Manager) CheckRiskManagement(ctx context.Context, signal *strategy.Signal) error {
	// Get current balance to calculate max trade amount
	balances, err := rm.trader.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to get balance for risk check: %w", err)
	}

	// Calculate total balance in JPY
	totalBalanceJPY := 0.0
	for _, bal := range balances {
		if bal.Currency == "JPY" {
			totalBalanceJPY = bal.Available
			break
		}
	}

	// BUY orders require positive JPY balance and are subject to the trade-amount
	// cap.  SELL orders liquidate crypto holdings back to JPY, so a zero JPY
	// balance is expected and the cap does not apply.
	if signal.Action == strategy.SignalBuy {
		if totalBalanceJPY <= 0 {
			return fmt.Errorf("insufficient JPY balance for BUY order: %f", totalBalanceJPY)
		}
		if err := rm.checkTradeAmount(signal, totalBalanceJPY); err != nil {
			return fmt.Errorf("trade amount validation failed: %w", err)
		}
	}

	// Check daily trade limit
	if err := rm.checkDailyTradeLimit(); err != nil {
		return err
	}

	// Check consecutive trade interval limit
	if err := rm.checkTradeInterval(); err != nil {
		return fmt.Errorf("trade interval too short: %w", err)
	}

	// Check total loss limit
	if err := rm.checkTotalLossLimit(ctx); err != nil {
		return fmt.Errorf("total loss limit exceeded: %w", err)
	}

	return nil
}

// checkTradeAmount validates trade amount against balance and configured limits
func (rm *Manager) checkTradeAmount(signal *strategy.Signal, totalBalanceJPY float64) error {
	tradeAmount := signal.Price * signal.Quantity

	// For BUY orders, include estimated fee in the trade amount
	if signal.Action == strategy.SignalBuy {
		feeRate := rm.cfg.FeeRate
		tradeAmount *= (1 + feeRate)
	}

	maxAmount := totalBalanceJPY * rm.cfg.MaxTradeAmountPercent / 100

	if tradeAmount > maxAmount {
		return fmt.Errorf("trade amount %f (including fees) exceeds maximum %f (%.1f%% of balance %f)",
			tradeAmount, maxAmount, rm.cfg.MaxTradeAmountPercent, totalBalanceJPY)
	}

	return nil
}

// checkDailyTradeLimit checks if daily trade limit has been reached
func (rm *Manager) checkDailyTradeLimit() error {
	// Get today's date in JST (year, month, day only)
	now := utils.NowInJST()
	todayYear, todayMonth, todayDay := now.Date()

	trades, err := rm.tradingRepo.GetRecentTrades(100) // Get recent 100 trades
	if err != nil {
		return fmt.Errorf("failed to get recent trades: %w", err)
	}

	// Count today's trades
	todayTrades := 0
	for i := range trades {
		tradeTime := utils.ToJST(trades[i].CreatedAt)
		tradeYear, tradeMonth, tradeDay := tradeTime.Date()
		// Compare date components instead of truncating
		if tradeYear == todayYear && tradeMonth == todayMonth && tradeDay == todayDay {
			todayTrades++
		}
	}

	maxDailyTrades := rm.cfg.MaxDailyTrades
	if todayTrades >= maxDailyTrades {
		return fmt.Errorf("daily trade limit reached: %d/%d", todayTrades, maxDailyTrades)
	}

	return nil
}

// checkTradeInterval validates minimum time between consecutive trades
func (rm *Manager) checkTradeInterval() error {
	// Get last trade time
	trades, err := rm.tradingRepo.GetRecentTrades(1)
	if err != nil {
		return fmt.Errorf("failed to get recent trades: %w", err)
	}

	if len(trades) == 0 {
		return nil // OK for first trade
	}

	// Use ExecutedAt (actual execution time) instead of CreatedAt (order placement time)
	lastTradeTime := trades[0].ExecutedAt
	minInterval := rm.cfg.MinTradeInterval

	// Use duration directly (already parsed in ManagerConfig)
	duration := minInterval
	if duration == 0 {
		duration = 5 * time.Minute
	}

	if time.Since(lastTradeTime) < duration {
		return fmt.Errorf("trade interval too short: %v < %v", time.Since(lastTradeTime), duration)
	}

	return nil
}

// checkTotalLossLimit validates total loss against configured maximum loss percentage
func (rm *Manager) checkTotalLossLimit(ctx context.Context) error {
	// Get recent performance metrics
	metrics, err := rm.analyticsRepo.GetPerformanceMetrics(30) // Past 30 days
	if err != nil {
		return fmt.Errorf("failed to get performance metrics: %w", err)
	}

	if len(metrics) == 0 {
		return nil // OK if no data
	}

	// GetPerformanceMetrics returns rows ORDER BY date DESC, so metrics[0] is the
	// most recent entry.  Using metrics[len-1] was a bug that read the oldest record.
	latestMetric := metrics[0]
	totalLoss := -latestMetric.TotalPnL // Negative value indicates loss

	if totalLoss <= 0 {
		return nil // OK if no loss
	}

	// Get current total balance
	balances, err := rm.trader.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to get balance for loss limit check: %w", err)
	}

	// Calculate total assets (current balance + unrealized PnL)
	// For simplicity, use current JPY balance as baseline
	totalAssets := rm.cfg.InitialBalance
	for _, bal := range balances {
		if bal.Currency == "JPY" {
			totalAssets = bal.Available
			break
		}
	}

	// If JPY balance is zero (all capital is deployed in crypto),
	// fall back to InitialBalance to avoid a division-by-zero that would
	// produce +Inf and incorrectly block SELL orders.
	if totalAssets <= 0 {
		totalAssets = rm.cfg.InitialBalance
	}
	if totalAssets <= 0 {
		// InitialBalance also zero — cannot evaluate loss percentage, skip check.
		return nil
	}

	// Calculate loss percentage against current assets.
	// totalLoss is the absolute loss value; totalAssets is the current balance.
	lossPercent := (totalLoss / totalAssets) * 100
	maxLossPercent := rm.cfg.MaxTotalLossPercent

	if lossPercent > maxLossPercent {
		return fmt.Errorf("total loss limit exceeded: %.2f%% > %.2f%% (total loss: %.2f, current assets: %.2f)",
			lossPercent, maxLossPercent, totalLoss, totalAssets)
	}

	return nil
}
