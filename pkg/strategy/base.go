package strategy

import (
	"context"
	"fmt"
	"time"
)

// BaseStrategy provides default no-op implementations of the Strategy interface.
// Embed *BaseStrategy in concrete strategies to avoid boilerplate.
type BaseStrategy struct {
	name        string
	description string
	version     string
	config      map[string]interface{}
	status      StrategyStatus
	metrics     StrategyMetrics
	isRunning   bool
}

// NewBaseStrategy creates a BaseStrategy with the given metadata.
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

func (bs *BaseStrategy) Name() string        { return bs.name }
func (bs *BaseStrategy) Description() string { return bs.description }
func (bs *BaseStrategy) Version() string     { return bs.version }
func (bs *BaseStrategy) GetConfig() map[string]interface{} {
	return bs.config
}
func (bs *BaseStrategy) IsRunning() bool             { return bs.isRunning }
func (bs *BaseStrategy) GetStatus() StrategyStatus   { return bs.status }
func (bs *BaseStrategy) GetMetrics() StrategyMetrics { return bs.metrics }

func (bs *BaseStrategy) Start(_ context.Context) error {
	bs.isRunning = true
	bs.status.IsRunning = true
	bs.status.StartTime = time.Now()
	return nil
}

func (bs *BaseStrategy) Stop(_ context.Context) error {
	bs.isRunning = false
	bs.status.IsRunning = false
	return nil
}

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

// RecordTrade is a no-op default; override in concrete strategies.
func (bs *BaseStrategy) RecordTrade() {}

// InitializeDailyTradeCount is a no-op default; override in concrete strategies.
func (bs *BaseStrategy) InitializeDailyTradeCount(_ int) {}

// GetStopLossPrice returns 0 by default (stop-loss disabled).
// Override in concrete strategies that support stop-loss.
func (bs *BaseStrategy) GetStopLossPrice(_ float64) float64 { return 0 }

// GetTakeProfitPrice returns 0 by default (take-profit disabled).
// Override in concrete strategies that support take-profit.
func (bs *BaseStrategy) GetTakeProfitPrice(_ float64) float64 { return 0 }

// GetBaseNotional returns 0 by default.
// Override in concrete strategies to provide the configured order size.
func (bs *BaseStrategy) GetBaseNotional(_ string) float64 { return 0 }

// GetAutoScaleConfig returns a disabled auto-scale config by default.
// Override in concrete strategies that support auto-scaling.
func (bs *BaseStrategy) GetAutoScaleConfig() AutoScaleConfig {
	return AutoScaleConfig{Enabled: false, BalancePct: 80.0}
}

// UpdateSignalCount updates the signal accounting in StrategyStatus.
func (bs *BaseStrategy) UpdateSignalCount(action SignalAction) {
	bs.status.TotalSignals++
	bs.status.SignalsByAction[action]++
	bs.status.LastSignalTime = time.Now()
}

// UpdateMetrics updates StrategyMetrics after a trade completes.
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
	if bs.metrics.TotalTrades > 0 {
		bs.metrics.WinRate = float64(bs.metrics.WinningTrades) / float64(bs.metrics.TotalTrades) * 100
		bs.metrics.AverageProfit = bs.metrics.TotalProfit / float64(bs.metrics.TotalTrades)
	}
	if bs.metrics.MaxLoss != 0 {
		bs.metrics.ProfitFactor = bs.metrics.MaxProfit / (-bs.metrics.MaxLoss)
	}
}

// ValidateSignal validates the common fields of a Signal.
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

// CreateSignal is a convenience constructor for Signal.
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
