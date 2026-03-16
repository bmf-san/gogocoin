package worker

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// mockStrategyGetter implements StrategyGetter for testing
type mockStrategyGetter struct {
	strategy strategy.Strategy
}

func (m *mockStrategyGetter) GetCurrentStrategy() strategy.Strategy {
	return m.strategy
}

// mockStrategy implements strategy.Strategy for testing
type mockStrategy struct {
	name       string
	isRunning  bool
	resetCalls int
}

func (m *mockStrategy) Name() string {
	return m.name
}

func (m *mockStrategy) Description() string {
	return "Mock strategy for testing"
}

func (m *mockStrategy) Version() string {
	return "1.0.0"
}

func (m *mockStrategy) Initialize(config map[string]any) error {
	return nil
}

func (m *mockStrategy) UpdateConfig(config map[string]any) error {
	return nil
}

func (m *mockStrategy) GetConfig() map[string]any {
	return map[string]any{}
}

func (m *mockStrategy) GenerateSignal(ctx context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	return nil, nil
}

func (m *mockStrategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	return nil, nil
}

func (m *mockStrategy) Start(ctx context.Context) error {
	m.isRunning = true
	return nil
}

func (m *mockStrategy) Stop(ctx context.Context) error {
	m.isRunning = false
	return nil
}

func (m *mockStrategy) IsRunning() bool {
	return m.isRunning
}

func (m *mockStrategy) GetStatus() strategy.StrategyStatus {
	return strategy.StrategyStatus{
		IsRunning: m.isRunning,
	}
}

func (m *mockStrategy) GetMetrics() strategy.StrategyMetrics {
	return strategy.StrategyMetrics{
		TotalTrades: 0,
	}
}

func (m *mockStrategy) RecordTrade() {
	// no-op for testing
}

func (m *mockStrategy) InitializeDailyTradeCount(count int) {
	// no-op for testing
}

func (m *mockStrategy) Reset() error {
	m.resetCalls++
	return nil
}

func TestStrategyMonitorWorker_Creation(t *testing.T) {
	cfg := &logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	mockStrat := &mockStrategy{
		name:      "test-strategy",
		isRunning: true,
	}
	mockGetter := &mockStrategyGetter{strategy: mockStrat}

	worker := NewStrategyMonitorWorker(log, mockGetter)

	if worker == nil {
		t.Fatal("Expected worker to be created")
	}
}

func TestStrategyMonitorWorker_RunAndCancel(t *testing.T) {
	cfg := &logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	mockStrat := &mockStrategy{
		name:      "test-strategy",
		isRunning: true,
	}
	mockGetter := &mockStrategyGetter{strategy: mockStrat}

	worker := NewStrategyMonitorWorker(log, mockGetter)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	// Wait for worker to stop
	select {
	case <-done:
		// Worker stopped successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Worker did not stop within timeout")
	}
}

func TestStrategyMonitorWorker_DailyReset(t *testing.T) {
	cfg := &logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	mockStrat := &mockStrategy{
		name:      "test-strategy",
		isRunning: true,
	}
	mockGetter := &mockStrategyGetter{strategy: mockStrat}

	worker := NewStrategyMonitorWorker(log, mockGetter)

	// Run worker briefly (daily reset only happens at midnight)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	<-done

	// In this short test, reset won't be called (only at midnight)
	// This test verifies the worker starts and stops cleanly
}
