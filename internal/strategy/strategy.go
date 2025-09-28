package strategy

import (
	"context"
	"fmt"
	"time"
)

// Signal is a trading signalrepresents
type Signal struct {
	Symbol    string                 `json:"symbol"`
	Action    SignalAction           `json:"action"`
	Strength  float64                `json:"strength"` // 0.0-1.0
	Price     float64                `json:"price"`
	Quantity  float64                `json:"quantity"`
	Amount    float64                `json:"amount"` // testcompatibilityofため
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// SignalAction issignalofアクション
type SignalAction string

const (
	SignalBuy  SignalAction = "BUY"
	SignalSell SignalAction = "SELL"
	SignalHold SignalAction = "HOLD"
)

// MarketData ismarket datarepresents
type MarketData struct {
	Symbol      string    `json:"symbol"`
	ProductCode string    `json:"product_code"`
	Price       float64   `json:"price"`
	Volume      float64   `json:"volume"`
	BestBid     float64   `json:"best_bid"`
	BestAsk     float64   `json:"best_ask"`
	Spread      float64   `json:"spread"`
	Timestamp   time.Time `json:"timestamp"`

	// OHLCV data
	Open  float64 `json:"open,omitempty"`
	High  float64 `json:"high,omitempty"`
	Low   float64 `json:"low,omitempty"`
	Close float64 `json:"close,omitempty"`
}

// Strategy is the trading strategy interface
type Strategy interface {
	// Strategyof基本情報
	Name() string
	Description() string
	Version() string

	// 初期化・configuration
	Initialize(config map[string]interface{}) error
	UpdateConfig(config map[string]interface{}) error
	GetConfig() map[string]interface{}

	// signal生成
	GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error)
	Analyze(data []MarketData) (*Signal, error)

	// Strategyof状態管理
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	GetStatus() StrategyStatus

	// performance分析
	GetMetrics() StrategyMetrics
	Reset() error
}

// StrategyStatus isstrategyof実行状態
type StrategyStatus struct {
	IsRunning        bool                 `json:"is_running"`
	StartTime        time.Time            `json:"start_time"`
	LastSignalTime   time.Time            `json:"last_signal_time"`
	TotalSignals     int                  `json:"total_signals"`
	SignalsByAction  map[SignalAction]int `json:"signals_by_action"`
	LastError        string               `json:"last_error,omitempty"`
	CurrentPositions map[string]float64   `json:"current_positions"`
}

// StrategyMetrics isstrategyofperformancemetrics
type StrategyMetrics struct {
	// 基本統計
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`

	// PnL
	TotalProfit   float64 `json:"total_profit"`
	AverageProfit float64 `json:"average_profit"`
	MaxProfit     float64 `json:"max_profit"`
	MaxLoss       float64 `json:"max_loss"`

	// リスクmetrics
	MaxDrawdown  float64 `json:"max_drawdown"`
	ProfitFactor float64 `json:"profit_factor"`
	SharpeRatio  float64 `json:"sharpe_ratio"`

	// 期間別performance
	Daily   []DailyMetrics   `json:"daily"`
	Monthly []MonthlyMetrics `json:"monthly"`
}

// DailyMetrics is日次performancemetrics
type DailyMetrics struct {
	Date        time.Time `json:"date"`
	Trades      int       `json:"trades"`
	Profit      float64   `json:"profit"`
	WinRate     float64   `json:"win_rate"`
	MaxDrawdown float64   `json:"max_drawdown"`
}

// MonthlyMetrics is月次performancemetrics
type MonthlyMetrics struct {
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	Trades      int     `json:"trades"`
	Profit      float64 `json:"profit"`
	WinRate     float64 `json:"win_rate"`
	MaxDrawdown float64 `json:"max_drawdown"`
}

// BaseStrategy isstrategyof基本実装
type BaseStrategy struct {
	name        string
	description string
	version     string
	config      map[string]interface{}
	status      StrategyStatus
	metrics     StrategyMetrics
	isRunning   bool
}

// NewBaseStrategy is基本strategycreates
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

// Name isstrategy名returns
func (bs *BaseStrategy) Name() string {
	return bs.name
}

// Description isstrategyof説明returns
func (bs *BaseStrategy) Description() string {
	return bs.description
}

// Version isstrategyofバージョンreturns
func (bs *BaseStrategy) Version() string {
	return bs.version
}

// GetConfig isconfigurationreturns
func (bs *BaseStrategy) GetConfig() map[string]interface{} {
	return bs.config
}

// IsRunning is実行中かどうかreturns
func (bs *BaseStrategy) IsRunning() bool {
	return bs.isRunning
}

// GetStatus is状態returns
func (bs *BaseStrategy) GetStatus() StrategyStatus {
	return bs.status
}

// GetMetrics isメトリクスreturns
func (bs *BaseStrategy) GetMetrics() StrategyMetrics {
	return bs.metrics
}

// Start isstrategystarts
func (bs *BaseStrategy) Start(ctx context.Context) error {
	bs.isRunning = true
	bs.status.IsRunning = true
	bs.status.StartTime = time.Now()
	return nil
}

// Stop isstrategystops
func (bs *BaseStrategy) Stop(ctx context.Context) error {
	bs.isRunning = false
	bs.status.IsRunning = false
	return nil
}

// Reset isstrategy" "リセットする
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

// UpdateSignalCount issignal数updates
func (bs *BaseStrategy) UpdateSignalCount(action SignalAction) {
	bs.status.TotalSignals++
	bs.status.SignalsByAction[action]++
	bs.status.LastSignalTime = time.Now()
}

// UpdateMetrics isメトリクスupdates
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

	// 勝率" "計算
	if bs.metrics.TotalTrades > 0 {
		bs.metrics.WinRate = float64(bs.metrics.WinningTrades) / float64(bs.metrics.TotalTrades)
	}

	// 平均利益" "計算
	if bs.metrics.TotalTrades > 0 {
		bs.metrics.AverageProfit = bs.metrics.TotalProfit / float64(bs.metrics.TotalTrades)
	}

	// プロフィットファクター" "計算
	if bs.metrics.MaxLoss != 0 {
		bs.metrics.ProfitFactor = bs.metrics.MaxProfit / (-bs.metrics.MaxLoss)
	}
}

// CalculateQuantity istradingsizecalculates
func (bs *BaseStrategy) CalculateQuantity(price float64, balance float64, riskPercent float64) float64 {
	if price <= 0 || balance <= 0 || riskPercent <= 0 {
		return 0
	}

	// risk management：balanceof指定パーセンテージまwith
	maxAmount := balance * (riskPercent / 100.0)
	quantity := maxAmount / price

	return quantity
}

// ValidateSignal issignalof妥当性validates
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

// CreateSignal issignalcreates
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
func (sf *StrategyFactory) CreateStrategy(name string, config interface{}, riskConfig RiskConfig) (Strategy, error) {
	switch name {
	case "scalping":
		return sf.createScalping(config, riskConfig)
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", name)
	}
}

// RiskConfig isrisk management configuration
type RiskConfig struct {
	MaxTradeAmountPercent float64
	InitialBalance        float64
	StopLossPercent       float64
	TakeProfitPercent     float64
}

// createScalping creates a minimal stateless scalping strategy
func (sf *StrategyFactory) createScalping(config interface{}, riskConfig RiskConfig) (Strategy, error) {
	// Default configuration for minimal stateless scalping
	defaultConfig := ScalpingConfig{
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
	if config != nil {
		if params, ok := config.(map[string]interface{}); ok {
			if emaFast, ok := params["ema_fast_period"].(int); ok {
				defaultConfig.EMAFastPeriod = emaFast
			}
			if emaSlow, ok := params["ema_slow_period"].(int); ok {
				defaultConfig.EMASlowPeriod = emaSlow
			}
			if tp, ok := params["take_profit_pct"].(float64); ok {
				defaultConfig.TakeProfitPct = tp
			}
			if sl, ok := params["stop_loss_pct"].(float64); ok {
				defaultConfig.StopLossPct = sl
			}
			// Handle cooldown_sec as either int or float64
			if cooldown, ok := params["cooldown_sec"].(int); ok {
				defaultConfig.CooldownSec = cooldown
			} else if cooldownFloat, ok := params["cooldown_sec"].(float64); ok {
				defaultConfig.CooldownSec = int(cooldownFloat)
			}
			// Handle max_daily_trades as either int or float64
			if maxTrades, ok := params["max_daily_trades"].(int); ok {
				defaultConfig.MaxDailyTrades = maxTrades
			} else if maxTradesFloat, ok := params["max_daily_trades"].(float64); ok {
				defaultConfig.MaxDailyTrades = int(maxTradesFloat)
			}
			if minNotional, ok := params["min_notional"].(float64); ok {
				defaultConfig.MinNotional = minNotional
			}
			if feeRate, ok := params["fee_rate"].(float64); ok {
				defaultConfig.FeeRate = feeRate
			}
		}
	}

	strategy := NewScalping(defaultConfig)

	// Reset strategy state
	if err := strategy.Reset(); err != nil {
		return nil, fmt.Errorf("failed to reset strategy: %w", err)
	}

	return strategy, nil
}

// GetSupportedStrategies issupportedstrategy名ofリストreturns
func (sf *StrategyFactory) GetSupportedStrategies() []string {
	return []string{"scalping"}
}
