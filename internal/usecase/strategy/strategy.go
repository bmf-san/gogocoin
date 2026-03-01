package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// Signal represents a trading signal
type Signal struct {
	Symbol    string                 `json:"symbol"`
	Action    SignalAction           `json:"action"`
	Strength  float64                `json:"strength"` // 0.0-1.0
	Price     float64                `json:"price"`
	Quantity  float64                `json:"quantity"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// SignalAction represents a signal action
type SignalAction string

const (
	SignalBuy  SignalAction = "BUY"
	SignalSell SignalAction = "SELL"
	SignalHold SignalAction = "HOLD"
)

// MarketData is an alias to domain.MarketData for backward compatibility
// Deprecated: Use domain.MarketData directly
type MarketData = domain.MarketData

// Strategy is the complete trading strategy interface.
type Strategy interface {
	// Signal generation
	GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error)
	Analyze(data []MarketData) (*Signal, error)
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	GetStatus() StrategyStatus
	Reset() error
	// Metrics
	GetMetrics() StrategyMetrics
	RecordTrade()
	InitializeDailyTradeCount(count int)
	// Configuration
	Name() string
	Description() string
	Version() string
	Initialize(config map[string]interface{}) error
	UpdateConfig(config map[string]interface{}) error
	GetConfig() map[string]interface{}
}

// StrategyStatus represents the execution state of a strategy
type StrategyStatus struct {
	IsRunning        bool                 `json:"is_running"`
	StartTime        time.Time            `json:"start_time"`
	LastSignalTime   time.Time            `json:"last_signal_time"`
	TotalSignals     int                  `json:"total_signals"`
	SignalsByAction  map[SignalAction]int `json:"signals_by_action"`
	LastError        string               `json:"last_error,omitempty"`
	CurrentPositions map[string]float64   `json:"current_positions"`
}

// StrategyMetrics represents performance metrics of a strategy
type StrategyMetrics struct {
	// Basic statistics
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`

	// PnL
	TotalProfit   float64 `json:"total_profit"`
	AverageProfit float64 `json:"average_profit"`
	MaxProfit     float64 `json:"max_profit"`
	MaxLoss       float64 `json:"max_loss"`

	// Risk metrics
	MaxDrawdown  float64 `json:"max_drawdown"`
	ProfitFactor float64 `json:"profit_factor"`
	SharpeRatio  float64 `json:"sharpe_ratio"`

	// Performance by period
	Daily   []DailyMetrics   `json:"daily"`
	Monthly []MonthlyMetrics `json:"monthly"`
}

// DailyMetrics represents daily performance metrics
type DailyMetrics struct {
	Date        time.Time `json:"date"`
	Trades      int       `json:"trades"`
	Profit      float64   `json:"profit"`
	WinRate     float64   `json:"win_rate"`
	MaxDrawdown float64   `json:"max_drawdown"`
}

// MonthlyMetrics represents monthly performance metrics
type MonthlyMetrics struct {
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	Trades      int     `json:"trades"`
	Profit      float64 `json:"profit"`
	WinRate     float64 `json:"win_rate"`
	MaxDrawdown float64 `json:"max_drawdown"`
}

// BaseStrategy provides the base implementation of a strategy
type BaseStrategy struct {
	name        string
	description string
	version     string
	config      map[string]interface{}
	status      StrategyStatus
	metrics     StrategyMetrics
	isRunning   bool
}

// NewBaseStrategy creates a base strategy
func NewBaseStrategy(name, description, version string) *BaseStrategy {
	return &BaseStrategy{
		name:        name,
		description: description,
		version:     version,
		config:      make(map[string]interface{}),
		status: StrategyStatus{
			SignalsByAction:  make(map[SignalAction]int),
			CurrentPositions: make(map[string]float64),
		},
		metrics: StrategyMetrics{
			Daily:   make([]DailyMetrics, 0),
			Monthly: make([]MonthlyMetrics, 0),
		},
	}
}

// Name returns the strategy name
func (bs *BaseStrategy) Name() string {
	return bs.name
}

// Description returns the strategy description
func (bs *BaseStrategy) Description() string {
	return bs.description
}

// Version returns the strategy version
func (bs *BaseStrategy) Version() string {
	return bs.version
}

// GetConfig returns the configuration
func (bs *BaseStrategy) GetConfig() map[string]interface{} {
	return bs.config
}

// IsRunning returns whether the strategy is running
func (bs *BaseStrategy) IsRunning() bool {
	return bs.isRunning
}

// GetStatus returns the status
func (bs *BaseStrategy) GetStatus() StrategyStatus {
	return bs.status
}

// GetMetrics returns the metrics
func (bs *BaseStrategy) GetMetrics() StrategyMetrics {
	return bs.metrics
}

// Start starts the strategy
func (bs *BaseStrategy) Start(ctx context.Context) error {
	bs.isRunning = true
	bs.status.IsRunning = true
	bs.status.StartTime = time.Now()
	return nil
}

// Stop stops the strategy
func (bs *BaseStrategy) Stop(ctx context.Context) error {
	bs.isRunning = false
	bs.status.IsRunning = false
	return nil
}

// Reset resets the strategy
func (bs *BaseStrategy) Reset() error {
	bs.status = StrategyStatus{
		SignalsByAction:  make(map[SignalAction]int),
		CurrentPositions: make(map[string]float64),
	}
	bs.metrics = StrategyMetrics{
		Daily:   make([]DailyMetrics, 0),
		Monthly: make([]MonthlyMetrics, 0),
	}
	return nil
}

// RecordTrade records a trade (default implementation: no-op)
func (bs *BaseStrategy) RecordTrade() {
	// BaseStrategy does nothing by default
	// Override in concrete strategies as needed
}

// InitializeDailyTradeCount initializes the daily trade count (default implementation: no-op)
func (bs *BaseStrategy) InitializeDailyTradeCount(count int) {
	// BaseStrategy does nothing by default
	// Override in concrete strategies as needed
}

// UpdateSignalCount updates the signal count
func (bs *BaseStrategy) UpdateSignalCount(action SignalAction) {
	bs.status.TotalSignals++
	bs.status.SignalsByAction[action]++
	bs.status.LastSignalTime = time.Now()
}

// UpdateMetrics updates the metrics
func (bs *BaseStrategy) UpdateMetrics(profit float64, isWin bool) {
	bs.metrics.TotalTrades++
	bs.metrics.TotalProfit += profit

	if isWin {
		bs.metrics.WinningTrades++
		if profit > bs.metrics.MaxProfit {
			bs.metrics.MaxProfit = profit
		}
	} else {
		bs.metrics.LosingTrades++
		if profit < bs.metrics.MaxLoss {
			bs.metrics.MaxLoss = profit
		}
	}

	// Calculate win rate (0-100 percentage scale)
	if bs.metrics.TotalTrades > 0 {
		bs.metrics.WinRate = float64(bs.metrics.WinningTrades) / float64(bs.metrics.TotalTrades) * 100
	}

	// Calculate average profit
	if bs.metrics.TotalTrades > 0 {
		bs.metrics.AverageProfit = bs.metrics.TotalProfit / float64(bs.metrics.TotalTrades)
	}

	// Calculate profit factor
	if bs.metrics.MaxLoss != 0 {
		bs.metrics.ProfitFactor = bs.metrics.MaxProfit / (-bs.metrics.MaxLoss)
	}
}

// CalculateQuantity calculates the trading size
func (bs *BaseStrategy) CalculateQuantity(price float64, balance float64, riskPercent float64) float64 {
	if price <= 0 || balance <= 0 || riskPercent <= 0 {
		return 0
	}

	// Risk management: up to the specified percentage of balance
	maxAmount := balance * (riskPercent / 100.0)
	quantity := maxAmount / price

	return quantity
}

// ValidateSignal validates the signal
func (bs *BaseStrategy) ValidateSignal(signal *Signal) error {
	if signal == nil {
		return fmt.Errorf("signal is nil")
	}

	if signal.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if signal.Action != SignalBuy && signal.Action != SignalSell && signal.Action != SignalHold {
		return fmt.Errorf("invalid action: %s", signal.Action)
	}

	if signal.Strength < 0 || signal.Strength > 1 {
		return fmt.Errorf("strength must be between 0 and 1: %f", signal.Strength)
	}

	if signal.Price <= 0 {
		return fmt.Errorf("price must be positive: %f", signal.Price)
	}

	if signal.Quantity < 0 {
		return fmt.Errorf("quantity must be non-negative: %f", signal.Quantity)
	}

	return nil
}

// CreateSignal creates a signal
func (bs *BaseStrategy) CreateSignal(symbol string, action SignalAction, strength, price, quantity float64, metadata map[string]interface{}) *Signal {
	return &Signal{
		Symbol:    symbol,
		Action:    action,
		Strength:  strength,
		Price:     price,
		Quantity:  quantity,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
}

// StrategyFactory creates strategies
type StrategyFactory struct{}

// NewStrategyFactory creates a factory
func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{}
}

// CreateStrategy creates a strategy from name and configuration
func (sf *StrategyFactory) CreateStrategy(name string, config interface{}) (Strategy, error) {
	switch name {
	case "scalping":
		return sf.createScalping(config)
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", name)
	}
}

// createScalping creates a minimal stateless scalping strategy
func (sf *StrategyFactory) createScalping(config interface{}) (Strategy, error) {
	// Default configuration for minimal stateless scalping
	defaultConfig := ScalpingParams{
		EMAFastPeriod:  9,     // Fast EMA period (9 bars)
		EMASlowPeriod:  21,    // Slow EMA period (21 bars)
		TakeProfitPct:  0.8,   // Take profit at +0.8%
		StopLossPct:    0.4,   // Stop loss at -0.4%
		CooldownSec:    90,    // 90 seconds cooldown
		MaxDailyTrades: 3,     // Maximum 3 trades per day
		MinNotional:    200.0, // Minimum 200 JPY order
		FeeRate:        0.001, // 0.1% fee rate
	}

	// Override with provided configuration
	// Only accept ScalpingParams struct for type safety
	if config != nil {
		if params, ok := config.(ScalpingParams); ok {
			if params.EMAFastPeriod > 0 {
				defaultConfig.EMAFastPeriod = params.EMAFastPeriod
			}
			if params.EMASlowPeriod > 0 {
				defaultConfig.EMASlowPeriod = params.EMASlowPeriod
			}
			if params.TakeProfitPct > 0 {
				defaultConfig.TakeProfitPct = params.TakeProfitPct
			}
			if params.StopLossPct > 0 {
				defaultConfig.StopLossPct = params.StopLossPct
			}
			if params.CooldownSec > 0 {
				defaultConfig.CooldownSec = params.CooldownSec
			}
			if params.MaxDailyTrades > 0 {
				defaultConfig.MaxDailyTrades = params.MaxDailyTrades
			}
			if params.MinNotional > 0 {
				defaultConfig.MinNotional = params.MinNotional
			}
			if params.FeeRate > 0 {
				defaultConfig.FeeRate = params.FeeRate
			}
		}
		// If config is not ScalpingParams, ignore it and use defaults
		// This ensures type safety - no runtime type assertions needed
	}

	strategy := NewScalping(defaultConfig)

	// Reset strategy state
	if err := strategy.Reset(); err != nil {
		return nil, fmt.Errorf("failed to reset strategy: %w", err)
	}

	return strategy, nil
}

// GetSupportedStrategies returns a list of supported strategy names
func (sf *StrategyFactory) GetSupportedStrategies() []string {
	return []string{"scalping"}
}
