package strategy

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSignalAction_String(t *testing.T) {
	tests := []struct {
		action   SignalAction
		expected string
	}{
		{SignalBuy, "BUY"},
		{SignalSell, "SELL"},
		{SignalHold, "HOLD"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if string(tt.action) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.action))
			}
		})
	}
}

func TestNewBaseStrategy(t *testing.T) {
	name := "test_strategy"
	description := "Test strategy description"
	version := "1.0.0"

	strategy := NewBaseStrategy(name, description, version)

	if strategy.Name() != name {
		t.Errorf("Expected name %s, got %s", name, strategy.Name())
	}
	if strategy.Description() != description {
		t.Errorf("Expected description %s, got %s", description, strategy.Description())
	}
	if strategy.Version() != version {
		t.Errorf("Expected version %s, got %s", version, strategy.Version())
	}
	if strategy.IsRunning() {
		t.Error("New strategy should not be running")
	}

	// configurationが初期化されているか確認
	config := strategy.GetConfig()
	if config == nil {
		t.Error("Config should be initialized")
	}

	// ステータスが初期化されているか確認
	status := strategy.GetStatus()
	if status.SignalsByAction == nil {
		t.Error("SignalsByAction should be initialized")
	}
	if status.CurrentPositions == nil {
		t.Error("CurrentPositions should be initialized")
	}

	// メトリクスが初期化されているか確認
	metrics := strategy.GetMetrics()
	if metrics.Daily == nil {
		t.Error("Daily metrics should be initialized")
	}
	if metrics.Monthly == nil {
		t.Error("Monthly metrics should be initialized")
	}
}

func TestBaseStrategy_StartStop(t *testing.T) {
	strategy := NewBaseStrategy("test", "test", "1.0.0")
	ctx := context.Background()

	// 初期状態is停止中
	if strategy.IsRunning() {
		t.Error("Strategy should not be running initially")
	}

	// 開始
	err := strategy.Start(ctx)
	if err != nil {
		t.Errorf("Start should not return error: %v", err)
	}
	if !strategy.IsRunning() {
		t.Error("Strategy should be running after Start")
	}

	status := strategy.GetStatus()
	if !status.IsRunning {
		t.Error("Status should show running")
	}
	if status.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}

	// 停止
	err = strategy.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should not return error: %v", err)
	}
	if strategy.IsRunning() {
		t.Error("Strategy should not be running after Stop")
	}

	status = strategy.GetStatus()
	if status.IsRunning {
		t.Error("Status should show not running")
	}
}

func TestBaseStrategy_Reset(t *testing.T) {
	strategy := NewBaseStrategy("test", "test", "1.0.0")

	// 初期状態" "変更
	strategy.status.TotalSignals = 10
	strategy.status.SignalsByAction[SignalBuy] = 5
	strategy.metrics.TotalTrades = 20

	// リセット
	err := strategy.Reset()
	if err != nil {
		t.Errorf("Reset should not return error: %v", err)
	}

	// リセット後of確認
	status := strategy.GetStatus()
	if status.TotalSignals != 0 {
		t.Error("TotalSignals should be reset to 0")
	}
	if len(status.SignalsByAction) != 0 {
		t.Error("SignalsByAction should be reset")
	}

	metrics := strategy.GetMetrics()
	if metrics.TotalTrades != 0 {
		t.Error("TotalTrades should be reset to 0")
	}
}

func TestSignal_Creation(t *testing.T) {
	signal := &Signal{
		Symbol:    "BTC_JPY",
		Action:    SignalBuy,
		Strength:  0.8,
		Price:     4000000,
		Quantity:  0.01,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"reason": "test",
		},
	}

	if signal.Symbol != "BTC_JPY" {
		t.Errorf("Expected symbol BTC_JPY, got %s", signal.Symbol)
	}
	if signal.Action != SignalBuy {
		t.Errorf("Expected action BUY, got %s", signal.Action)
	}
	if signal.Strength != 0.8 {
		t.Errorf("Expected strength 0.8, got %f", signal.Strength)
	}
	if signal.Price != 4000000 {
		t.Errorf("Expected price 4000000, got %f", signal.Price)
	}
	if signal.Quantity != 0.01 {
		t.Errorf("Expected quantity 0.01, got %f", signal.Quantity)
	}
	if signal.Metadata["reason"] != "test" {
		t.Errorf("Expected metadata reason 'test', got %v", signal.Metadata["reason"])
	}
}

func TestMarketData_Creation(t *testing.T) {
	timestamp := time.Now()
	data := MarketData{
		Symbol:      "BTC_JPY",
		ProductCode: "BTC_JPY",
		Price:       4000000,
		Volume:      100,
		BestBid:     3999000,
		BestAsk:     4001000,
		Spread:      2000,
		Timestamp:   timestamp,
		Open:        3995000,
		High:        4005000,
		Low:         3990000,
		Close:       4000000,
	}

	if data.Symbol != "BTC_JPY" {
		t.Errorf("Expected symbol BTC_JPY, got %s", data.Symbol)
	}
	if data.Price != 4000000 {
		t.Errorf("Expected price 4000000, got %f", data.Price)
	}
	if data.Volume != 100 {
		t.Errorf("Expected volume 100, got %f", data.Volume)
	}
	if data.Spread != 2000 {
		t.Errorf("Expected spread 2000, got %f", data.Spread)
	}
	if !data.Timestamp.Equal(timestamp) {
		t.Errorf("Expected timestamp %v, got %v", timestamp, data.Timestamp)
	}
	if data.High != 4005000 {
		t.Errorf("Expected high 4005000, got %f", data.High)
	}
	if data.Low != 3990000 {
		t.Errorf("Expected low 3990000, got %f", data.Low)
	}
}

func TestStrategyStatus_Initialization(t *testing.T) {
	status := StrategyStatus{
		IsRunning:        true,
		StartTime:        time.Now(),
		LastSignalTime:   time.Now(),
		TotalSignals:     10,
		SignalsByAction:  make(map[SignalAction]int),
		CurrentPositions: make(map[string]float64),
	}

	status.SignalsByAction[SignalBuy] = 5
	status.SignalsByAction[SignalSell] = 3
	status.SignalsByAction[SignalHold] = 2
	status.CurrentPositions["BTC_JPY"] = 0.01

	if !status.IsRunning {
		t.Error("Status should be running")
	}
	if status.TotalSignals != 10 {
		t.Errorf("Expected total signals 10, got %d", status.TotalSignals)
	}
	if status.SignalsByAction[SignalBuy] != 5 {
		t.Errorf("Expected 5 buy signals, got %d", status.SignalsByAction[SignalBuy])
	}
	if status.CurrentPositions["BTC_JPY"] != 0.01 {
		t.Errorf("Expected position 0.01, got %f", status.CurrentPositions["BTC_JPY"])
	}
}

func TestStrategyMetrics_Initialization(t *testing.T) {
	metrics := StrategyMetrics{
		TotalTrades:   20,
		WinningTrades: 12,
		LosingTrades:  8,
		WinRate:       60.0,
		TotalProfit:   1000.0,
		AverageProfit: 50.0,
		MaxProfit:     200.0,
		MaxLoss:       -150.0,
		SharpeRatio:   1.5,
		MaxDrawdown:   -300.0,
		ProfitFactor:  1.8,
		Daily:         make([]DailyMetrics, 0),
		Monthly:       make([]MonthlyMetrics, 0),
	}

	if metrics.TotalTrades != 20 {
		t.Errorf("Expected total trades 20, got %d", metrics.TotalTrades)
	}
	if metrics.WinRate != 60.0 {
		t.Errorf("Expected win rate 60.0, got %f", metrics.WinRate)
	}
	if metrics.SharpeRatio != 1.5 {
		t.Errorf("Expected Sharpe ratio 1.5, got %f", metrics.SharpeRatio)
	}
	if metrics.Daily == nil {
		t.Error("Daily metrics should be initialized")
	}
	if metrics.Monthly == nil {
		t.Error("Monthly metrics should be initialized")
	}
}

func TestDailyMetrics_Creation(t *testing.T) {
	date := time.Now()
	daily := DailyMetrics{
		Date:        date,
		Trades:      5,
		Profit:      100.0,
		WinRate:     80.0,
		MaxDrawdown: -50.0,
	}

	if !daily.Date.Equal(date) {
		t.Errorf("Expected date %v, got %v", date, daily.Date)
	}
	if daily.Trades != 5 {
		t.Errorf("Expected trades 5, got %d", daily.Trades)
	}
	if daily.Profit != 100.0 {
		t.Errorf("Expected profit 100.0, got %f", daily.Profit)
	}
	if daily.WinRate != 80.0 {
		t.Errorf("Expected win rate 80.0, got %f", daily.WinRate)
	}
}

func TestMonthlyMetrics_Creation(t *testing.T) {
	monthly := MonthlyMetrics{
		Year:        2024,
		Month:       9,
		Trades:      150,
		Profit:      3000.0,
		WinRate:     65.0,
		MaxDrawdown: -500.0,
	}

	if monthly.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", monthly.Year)
	}
	if monthly.Month != 9 {
		t.Errorf("Expected month 9, got %d", monthly.Month)
	}
	if monthly.Trades != 150 {
		t.Errorf("Expected trades 150, got %d", monthly.Trades)
	}
	if monthly.Profit != 3000.0 {
		t.Errorf("Expected profit 3000.0, got %f", monthly.Profit)
	}
}

// モックstrategy（for testing）
type MockTestStrategy struct {
	*BaseStrategy
	analyzeFunc        func([]MarketData) (*Signal, error)
	generateSignalFunc func(context.Context, *MarketData, []MarketData) (*Signal, error)
	initializeFunc     func(map[string]interface{}) error
	updateConfigFunc   func(map[string]interface{}) error
}

func NewMockTestStrategy() *MockTestStrategy {
	return &MockTestStrategy{
		BaseStrategy: NewBaseStrategy("mock", "Mock strategy for testing", "1.0.0"),
	}
}

func (m *MockTestStrategy) Initialize(config map[string]interface{}) error {
	if m.initializeFunc != nil {
		return m.initializeFunc(config)
	}
	return nil
}

func (m *MockTestStrategy) UpdateConfig(config map[string]interface{}) error {
	if m.updateConfigFunc != nil {
		return m.updateConfigFunc(config)
	}
	return nil
}

func (m *MockTestStrategy) GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error) {
	if m.generateSignalFunc != nil {
		return m.generateSignalFunc(ctx, data, history)
	}
	return &Signal{Action: SignalHold}, nil
}

func (m *MockTestStrategy) Analyze(data []MarketData) (*Signal, error) {
	if m.analyzeFunc != nil {
		return m.analyzeFunc(data)
	}
	return &Signal{Action: SignalHold}, nil
}

func TestMockStrategy_Interface(t *testing.T) {
	strategy := NewMockTestStrategy()

	// Strategy インターフェース" "実装しているか確認
	var _ Strategy = strategy

	// 基本メソッドoftest
	if strategy.Name() != "mock" {
		t.Errorf("Expected name 'mock', got %s", strategy.Name())
	}
	if strategy.Description() != "Mock strategy for testing" {
		t.Errorf("Expected description 'Mock strategy for testing', got %s", strategy.Description())
	}
	if strategy.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", strategy.Version())
	}

	// 開始・停止oftest
	ctx := context.Background()
	err := strategy.Start(ctx)
	if err != nil {
		t.Errorf("Start should not return error: %v", err)
	}
	if !strategy.IsRunning() {
		t.Error("Strategy should be running after Start")
	}

	err = strategy.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should not return error: %v", err)
	}
	if strategy.IsRunning() {
		t.Error("Strategy should not be running after Stop")
	}
}

func TestMockStrategy_CustomBehavior(t *testing.T) {
	strategy := NewMockTestStrategy()

	// カスタム分析関数" "configuration
	strategy.analyzeFunc = func(data []MarketData) (*Signal, error) {
		if len(data) == 0 {
			return &Signal{Action: SignalHold}, nil
		}
		return &Signal{
			Action:   SignalBuy,
			Symbol:   "BTC_JPY",
			Price:    data[len(data)-1].Price,
			Quantity: 0.01,
		}, nil
	}

	// testdata
	testData := []MarketData{
		{Symbol: "BTC_JPY", Price: 4000000, Timestamp: time.Now()},
	}

	signal, err := strategy.Analyze(testData)
	if err != nil {
		t.Errorf("Analyze should not return error: %v", err)
	}
	if signal.Action != SignalBuy {
		t.Errorf("Expected BUY signal, got %s", signal.Action)
	}
	if signal.Symbol != "BTC_JPY" {
		t.Errorf("Expected symbol BTC_JPY, got %s", signal.Symbol)
	}
	if signal.Price != 4000000 {
		t.Errorf("Expected price 4000000, got %f", signal.Price)
	}
}

func TestMockStrategy_GenerateSignal(t *testing.T) {
	strategy := NewMockTestStrategy()

	// カスタムsignal生成関数" "configuration
	strategy.generateSignalFunc = func(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error) {
		return &Signal{
			Action:    SignalSell,
			Symbol:    data.Symbol,
			Price:     data.Price,
			Quantity:  0.005,
			Timestamp: data.Timestamp,
			Metadata: map[string]interface{}{
				"reason": "custom_signal",
			},
		}, nil
	}

	ctx := context.Background()
	data := MarketData{
		Symbol:    "ETH_JPY",
		Price:     300000,
		Timestamp: time.Now(),
	}
	history := []MarketData{data}

	signal, err := strategy.GenerateSignal(ctx, &data, history)
	if err != nil {
		t.Errorf("GenerateSignal should not return error: %v", err)
	}
	if signal.Action != SignalSell {
		t.Errorf("Expected SELL signal, got %s", signal.Action)
	}
	if signal.Symbol != "ETH_JPY" {
		t.Errorf("Expected symbol ETH_JPY, got %s", signal.Symbol)
	}
	if signal.Quantity != 0.005 {
		t.Errorf("Expected quantity 0.005, got %f", signal.Quantity)
	}
	if signal.Metadata["reason"] != "custom_signal" {
		t.Errorf("Expected reason 'custom_signal', got %v", signal.Metadata["reason"])
	}
}

func TestMockStrategy_ConfigMethods(t *testing.T) {
	strategy := NewMockTestStrategy()

	// Initialize oftest
	initializeCalled := false
	strategy.initializeFunc = func(config map[string]interface{}) error {
		initializeCalled = true
		if config["test_param"] != "test_value" {
			t.Errorf("Expected test_param 'test_value', got %v", config["test_param"])
		}
		return nil
	}

	config := map[string]interface{}{
		"test_param": "test_value",
	}
	err := strategy.Initialize(config)
	if err != nil {
		t.Errorf("Initialize should not return error: %v", err)
	}
	if !initializeCalled {
		t.Error("Initialize function should have been called")
	}

	// UpdateConfig oftest
	updateCalled := false
	strategy.updateConfigFunc = func(config map[string]interface{}) error {
		updateCalled = true
		if config["update_param"] != "update_value" {
			t.Errorf("Expected update_param 'update_value', got %v", config["update_param"])
		}
		return nil
	}

	updateConfig := map[string]interface{}{
		"update_param": "update_value",
	}
	err = strategy.UpdateConfig(updateConfig)
	if err != nil {
		t.Errorf("UpdateConfig should not return error: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateConfig function should have been called")
	}
}

// ベンチマークtest
func BenchmarkBaseStrategy_StartStop(b *testing.B) {
	strategy := NewBaseStrategy("bench", "benchmark", "1.0.0")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := strategy.Start(ctx); err != nil {
			b.Fatalf("Failed to start strategy: %v", err)
		}
		if err := strategy.Stop(ctx); err != nil {
			b.Fatalf("Failed to stop strategy: %v", err)
		}
	}
}

func BenchmarkSignal_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Signal{
			Symbol:    "BTC_JPY",
			Action:    SignalBuy,
			Strength:  0.8,
			Price:     4000000,
			Quantity:  0.01,
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
	}
}

func BenchmarkMarketData_Creation(b *testing.B) {
	timestamp := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MarketData{
			Symbol:      "BTC_JPY",
			ProductCode: "BTC_JPY",
			Price:       4000000,
			Volume:      100,
			BestBid:     3999000,
			BestAsk:     4001000,
			Spread:      2000,
			Timestamp:   timestamp,
		}
	}
}

// StrategyFactory Tests

func TestNewStrategyFactory(t *testing.T) {
	factory := NewStrategyFactory()
	if factory == nil {
		t.Error("NewStrategyFactory should return non-nil factory")
	}
}

func TestStrategyFactory_GetSupportedStrategies(t *testing.T) {
	factory := NewStrategyFactory()
	strategies := factory.GetSupportedStrategies()

	expectedStrategies := []string{"scalping"}
	if len(strategies) != len(expectedStrategies) {
		t.Errorf("Expected %d strategies, got %d", len(expectedStrategies), len(strategies))
	}

	for i, expected := range expectedStrategies {
		if strategies[i] != expected {
			t.Errorf("Expected strategy %s at index %d, got %s", expected, i, strategies[i])
		}
	}
}

func TestStrategyFactory_CreateStrategy_Scalping(t *testing.T) {
	factory := NewStrategyFactory()
	riskConfig := RiskConfig{
		MaxTradeAmountPercent: 10.0,
		InitialBalance:        100000.0,
		StopLossPercent:       2.0,
		TakeProfitPercent:     5.0,
	}

	// Test with default config (nil)
	strategy, err := factory.CreateStrategy("scalping", nil, riskConfig)
	if err != nil {
		t.Errorf("CreateStrategy failed: %v", err)
	}
	if strategy == nil {
		t.Error("Strategy should not be nil")
	}
	if strategy.Name() != "scalping" {
		t.Errorf("Expected strategy name 'scalping', got '%s'", strategy.Name())
	}

	// Test with custom config
	customConfig := map[string]interface{}{
		"ema_fast_period":  5,
		"ema_slow_period":  15,
		"take_profit_pct":  1.0,
		"stop_loss_pct":    0.5,
		"cooldown_sec":     60,
		"max_daily_trades": 5,
		"min_notional":     300.0,
		"fee_rate":         0.002,
	}
	strategy2, err := factory.CreateStrategy("scalping", customConfig, riskConfig)
	if err != nil {
		t.Errorf("CreateStrategy with custom config failed: %v", err)
	}
	if strategy2 == nil {
		t.Error("Strategy should not be nil")
	}
}

func TestStrategyFactory_CreateStrategy_UnsupportedStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	riskConfig := RiskConfig{
		MaxTradeAmountPercent: 10.0,
		InitialBalance:        100000.0,
		StopLossPercent:       2.0,
		TakeProfitPercent:     5.0,
	}

	strategy, err := factory.CreateStrategy("unsupported_strategy", nil, riskConfig)
	if err == nil {
		t.Error("Expected error for unsupported strategy")
	}
	if strategy != nil {
		t.Error("Strategy should be nil for unsupported strategy")
	}
	if !strings.Contains(err.Error(), "unsupported strategy") {
		t.Errorf("Expected error message to contain 'unsupported strategy', got: %s", err.Error())
	}
}

func TestStrategyFactory_CreateStrategy_InvalidConfig(t *testing.T) {
	factory := NewStrategyFactory()
	riskConfig := RiskConfig{
		MaxTradeAmountPercent: 10.0,
		InitialBalance:        100000.0,
		StopLossPercent:       2.0,
		TakeProfitPercent:     5.0,
	}

	// Test with invalid config type (should still work with defaults)
	invalidConfig := "invalid_config_type"
	strategy, err := factory.CreateStrategy("scalping", invalidConfig, riskConfig)
	if err != nil {
		t.Errorf("CreateStrategy should handle invalid config gracefully: %v", err)
	}
	if strategy == nil {
		t.Error("Strategy should not be nil even with invalid config")
	}
}

func TestRiskConfig_Creation(t *testing.T) {
	riskConfig := RiskConfig{
		MaxTradeAmountPercent: 15.0,
		InitialBalance:        50000.0,
		StopLossPercent:       3.0,
		TakeProfitPercent:     6.0,
	}

	if riskConfig.MaxTradeAmountPercent != 15.0 {
		t.Errorf("Expected MaxTradeAmountPercent 15.0, got %f", riskConfig.MaxTradeAmountPercent)
	}
	if riskConfig.InitialBalance != 50000.0 {
		t.Errorf("Expected InitialBalance 50000.0, got %f", riskConfig.InitialBalance)
	}
	if riskConfig.StopLossPercent != 3.0 {
		t.Errorf("Expected StopLossPercent 3.0, got %f", riskConfig.StopLossPercent)
	}
	if riskConfig.TakeProfitPercent != 6.0 {
		t.Errorf("Expected TakeProfitPercent 6.0, got %f", riskConfig.TakeProfitPercent)
	}
}

// Benchmark tests for Strategy factory
func BenchmarkStrategyFactory_CreateScalping(b *testing.B) {
	factory := NewStrategyFactory()
	riskConfig := RiskConfig{
		MaxTradeAmountPercent: 10.0,
		InitialBalance:        100000.0,
		StopLossPercent:       2.0,
		TakeProfitPercent:     5.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy, err := factory.CreateStrategy("scalping", nil, riskConfig)
		if err != nil {
			b.Fatalf("CreateStrategy failed: %v", err)
		}
		if strategy == nil {
			b.Fatal("Strategy should not be nil")
		}
	}
}
