package analytics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// Mock implementations
type mockTradingRepo struct {
	trades []domain.Trade
}

func (m *mockTradingRepo) GetRecentTrades(limit int) ([]domain.Trade, error) {
	if limit > len(m.trades) {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

type mockAnalyticsRepo struct {
	savedMetrics []*domain.PerformanceMetric
}

func (m *mockAnalyticsRepo) SavePerformanceMetric(metric *domain.PerformanceMetric) error {
	m.savedMetrics = append(m.savedMetrics, metric)
	return nil
}

func (m *mockAnalyticsRepo) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {
	var result []domain.PerformanceMetric
	for _, m := range m.savedMetrics {
		result = append(result, *m)
	}
	return result, nil
}

func TestCalculateFromTrades_NoTrades(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	metrics := pa.CalculateFromTrades([]domain.Trade{})

	if metrics.TotalTrades != 0 {
		t.Errorf("Expected 0 trades, got %d", metrics.TotalTrades)
	}
	if metrics.TotalPnL != 0 {
		t.Errorf("Expected 0 PnL, got %f", metrics.TotalPnL)
	}
}

func TestCalculateFromTrades_SingleWinningTrade(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	trades := []domain.Trade{
		{
			Symbol:     "BTC_JPY",
			Side:       "SELL",
			Price:      1000000,
			Size:       0.01,
			Fee:        15,
			PnL:        100, // Profit
			CreatedAt:  time.Now(),
			ExecutedAt: time.Now(),
		},
	}

	metrics := pa.CalculateFromTrades(trades)

	if metrics.TotalTrades != 1 {
		t.Errorf("Expected 1 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 1 {
		t.Errorf("Expected 1 winning trades, got %d", metrics.WinningTrades)
	}
	if metrics.LosingTrades != 0 {
		t.Errorf("Expected 0 losing trades, got %d", metrics.LosingTrades)
	}
	if metrics.WinRate != 100.0 {
		t.Errorf("Expected 100%% win rate, got %.2f%%", metrics.WinRate)
	}
	if metrics.TotalPnL != 100 {
		t.Errorf("Expected 100 total PnL, got %.2f", metrics.TotalPnL)
	}
}

func TestCalculateFromTrades_SingleLosingTrade(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	trades := []domain.Trade{
		{
			Symbol:     "BTC_JPY",
			Side:       "SELL",
			Price:      1000000,
			Size:       0.01,
			Fee:        15,
			PnL:        -100, // Loss
			CreatedAt:  time.Now(),
			ExecutedAt: time.Now(),
		},
	}

	metrics := pa.CalculateFromTrades(trades)

	if metrics.TotalTrades != 1 {
		t.Errorf("Expected 1 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 0 {
		t.Errorf("Expected 0 winning trades, got %d", metrics.WinningTrades)
	}
	if metrics.LosingTrades != 1 {
		t.Errorf("Expected 1 losing trades, got %d", metrics.LosingTrades)
	}
	if metrics.WinRate != 0.0 {
		t.Errorf("Expected 0%% win rate, got %.2f%%", metrics.WinRate)
	}
	if metrics.TotalPnL != -100 {
		t.Errorf("Expected -100 total PnL, got %.2f", metrics.TotalPnL)
	}
}

func TestCalculateFromTrades_MixedTrades(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	trades := []domain.Trade{
		{PnL: 100, Fee: 15},  // Win
		{PnL: -50, Fee: 15},  // Loss
		{PnL: 200, Fee: 15},  // Win
		{PnL: -100, Fee: 15}, // Loss
		{PnL: 150, Fee: 15},  // Win
	}

	metrics := pa.CalculateFromTrades(trades)

	expectedTotalPnL := 100.0 - 50.0 + 200.0 - 100.0 + 150.0
	if math.Abs(metrics.TotalPnL-expectedTotalPnL) > 0.01 {
		t.Errorf("Expected total PnL %.2f, got %.2f", expectedTotalPnL, metrics.TotalPnL)
	}

	if metrics.TotalTrades != 5 {
		t.Errorf("Expected 5 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 3 {
		t.Errorf("Expected 3 winning trades, got %d", metrics.WinningTrades)
	}
	if metrics.LosingTrades != 2 {
		t.Errorf("Expected 2 losing trades, got %d", metrics.LosingTrades)
	}

	expectedWinRate := 3.0 / 5.0 * 100.0
	if math.Abs(metrics.WinRate-expectedWinRate) > 0.01 {
		t.Errorf("Expected win rate %.2f%%, got %.2f%%", expectedWinRate, metrics.WinRate)
	}
}

func TestCalculateFromTrades_ZeroPnLHandling(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	trades := []domain.Trade{
		{
			Side: "BUY",
			PnL:  0,
			Fee:  15,
		},
	}

	metrics := pa.CalculateFromTrades(trades)

	// For BUY with PnL=0, should count fee as loss
	expectedPnL := -15.0
	if math.Abs(metrics.TotalPnL-expectedPnL) > 0.01 {
		t.Errorf("Expected total PnL %.2f, got %.2f", expectedPnL, metrics.TotalPnL)
	}
}

func TestCalculateSharpeRatio(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	tests := []struct {
		name          string
		returns       []float64
		totalReturn   float64
		expectNonZero bool
	}{
		{
			name:          "No returns",
			returns:       []float64{},
			totalReturn:   0,
			expectNonZero: false,
		},
		{
			name:          "Single return",
			returns:       []float64{0.01},
			totalReturn:   1.0,
			expectNonZero: false,
		},
		{
			name:          "Multiple returns with variance",
			returns:       []float64{0.01, 0.02, -0.01, 0.03},
			totalReturn:   5.0,
			expectNonZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharpe := pa.calculateSharpeRatio(tt.returns, tt.totalReturn)

			if tt.expectNonZero && sharpe == 0 {
				t.Error("Expected non-zero Sharpe ratio")
			}
			if !tt.expectNonZero && sharpe != 0 {
				t.Errorf("Expected zero Sharpe ratio, got %.4f", sharpe)
			}
		})
	}
}

func TestCalculateMaxDrawdown(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	tests := []struct {
		name     string
		trades   []domain.Trade
		expected float64
	}{
		{
			name:     "No trades",
			trades:   []domain.Trade{},
			expected: 0,
		},
		{
			name: "Only winning trades - no drawdown",
			trades: []domain.Trade{
				{PnL: 100},
				{PnL: 200},
				{PnL: 150},
			},
			expected: 0,
		},
		{
			name: "With drawdown",
			trades: []domain.Trade{
				{PnL: 1000}, // Peak at 1000
				{PnL: -500}, // Drawdown to 500
				{PnL: 200},  // Recovery to 700
				{PnL: -300}, // Drawdown to 400
			},
			expected: 0.6, // (1000-400)/100000*100 = 0.6%
		},
		{
			name: "Zero PnL with fee",
			trades: []domain.Trade{
				{PnL: 0, Fee: 15}, // Should count as -15
			},
			expected: 0.015, // 15/100000*100 = 0.015%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxDD := pa.calculateMaxDrawdown(tt.trades)

			if math.Abs(maxDD-tt.expected) > 0.01 {
				t.Errorf("Expected max drawdown %.4f%%, got %.4f%%", tt.expected, maxDD)
			}
		})
	}
}

func TestUpdateMetrics(t *testing.T) {
	trades := []domain.Trade{
		{PnL: 100, Fee: 15},
		{PnL: -50, Fee: 15},
		{PnL: 200, Fee: 15},
	}

	tradingRepo := &mockTradingRepo{trades: trades}
	analyticsRepo := &mockAnalyticsRepo{}

	pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000)

	err := pa.UpdateMetrics(context.Background())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	if len(analyticsRepo.savedMetrics) != 1 {
		t.Errorf("Expected 1 saved metric, got %d", len(analyticsRepo.savedMetrics))
	}

	metric := analyticsRepo.savedMetrics[0]
	expectedPnL := 250.0
	if math.Abs(metric.TotalPnL-expectedPnL) > 0.01 {
		t.Errorf("Expected total PnL %.2f, got %.2f", expectedPnL, metric.TotalPnL)
	}
}

func TestUpdateMetrics_NoTrades(t *testing.T) {
	tradingRepo := &mockTradingRepo{trades: []domain.Trade{}}
	analyticsRepo := &mockAnalyticsRepo{}

	pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000)

	err := pa.UpdateMetrics(context.Background())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Should not save any metrics when there are no trades
	if len(analyticsRepo.savedMetrics) != 0 {
		t.Errorf("Expected 0 saved metrics, got %d", len(analyticsRepo.savedMetrics))
	}
}

func TestProfitFactor(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000)

	tests := []struct {
		name           string
		trades         []domain.Trade
		expectedFactor float64
	}{
		{
			name: "Profit factor 2.0",
			trades: []domain.Trade{
				{PnL: 200},  // Win
				{PnL: -100}, // Loss
			},
			expectedFactor: 2.0,
		},
		{
			name: "Only wins - no profit factor",
			trades: []domain.Trade{
				{PnL: 100},
				{PnL: 200},
			},
			expectedFactor: 0, // No losses, so no profit factor
		},
		{
			name: "Only losses",
			trades: []domain.Trade{
				{PnL: -100},
				{PnL: -200},
			},
			expectedFactor: 0, // No wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := pa.CalculateFromTrades(tt.trades)

			if math.Abs(metrics.ProfitFactor-tt.expectedFactor) > 0.01 {
				t.Errorf("Expected profit factor %.2f, got %.2f", tt.expectedFactor, metrics.ProfitFactor)
			}
		})
	}
}
