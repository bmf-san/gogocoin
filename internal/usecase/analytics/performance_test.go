package analytics

import (
	"context"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// Mock implementations
type mockTradingRepo struct {
	trades []domain.Trade
	// sinceArg records the cut-off GetTradesSince was called with.
	sinceArg time.Time
}

func (m *mockTradingRepo) GetRecentTrades(limit int) ([]domain.Trade, error) {
	if limit > len(m.trades) {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

func (m *mockTradingRepo) GetAllTrades() ([]domain.Trade, error) {
	return m.trades, nil
}

// GetTradesSince mirrors the real repository: filtered by executed_at and
// returned newest first.
func (m *mockTradingRepo) GetTradesSince(since time.Time, limit int) ([]domain.Trade, error) {
	m.sinceArg = since
	var out []domain.Trade
	for i := range m.trades {
		if !m.trades[i].ExecutedAt.Before(since) {
			out = append(out, m.trades[i])
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExecutedAt.After(out[j].ExecutedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
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
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

	metrics := pa.CalculateFromTrades([]domain.Trade{})

	if metrics.TotalTrades != 0 {
		t.Errorf("Expected 0 trades, got %d", metrics.TotalTrades)
	}
	if metrics.TotalPnL != 0 {
		t.Errorf("Expected 0 PnL, got %f", metrics.TotalPnL)
	}
}

func TestCalculateFromTrades_SingleWinningTrade(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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

// TestCalculateFromTrades_SellZeroPnLNoDoubleFee asserts that a SELL row whose
// stored PnL is exactly 0 (a rare break-even sale where sellRevenue-totalCost
// equals totalFees) is NOT re-subtracted for fees by CalculateFromTrades.
// calculator.go already subtracts fees when computing PnL, so substituting
// pnl = -fee here would double-count the fee.
func TestCalculateFromTrades_SellZeroPnLNoDoubleFee(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

	trades := []domain.Trade{
		{Side: "SELL", PnL: 0, Fee: 15},
	}

	metrics := pa.CalculateFromTrades(trades)

	if math.Abs(metrics.TotalPnL) > 0.01 {
		t.Errorf("Expected SELL PnL=0 to stay 0 (no double fee), got %.2f", metrics.TotalPnL)
	}
}

func TestCalculateSharpeRatio(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
			name: "Zero PnL with fee (legacy BUY row)",
			trades: []domain.Trade{
				// Legacy rows where PnL was never stored have Side="BUY" & PnL=0.
				// SELL rows with PnL=0 must NOT be substituted to avoid
				// double-counting fees (calculator.go already subtracts them).
				{Side: "BUY", PnL: 0, Fee: 15}, // Should count as -15
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

	pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000, time.Time{})

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

	pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000, time.Time{})

	err := pa.UpdateMetrics(context.Background())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Should not save any metrics when there are no trades
	if len(analyticsRepo.savedMetrics) != 0 {
		t.Errorf("Expected 0 saved metrics, got %d", len(analyticsRepo.savedMetrics))
	}
}

// TestUpdateMetrics_PnLEpoch verifies that a configured epoch excludes earlier
// trades entirely. The pre-epoch rows here carry the kind of inflated PnL the
// epoch exists to ignore, so leaking even one of them into the total is the
// exact failure this guards against.
func TestUpdateMetrics_PnLEpoch(t *testing.T) {
	epoch := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	trades := []domain.Trade{
		{PnL: 900, ExecutedAt: epoch.Add(-48 * time.Hour)},
		{PnL: 17, ExecutedAt: epoch.Add(-1 * time.Second)},
		{PnL: -30, ExecutedAt: epoch},
		{PnL: -70, ExecutedAt: epoch.Add(72 * time.Hour)},
	}

	t.Run("epoch set excludes earlier trades", func(t *testing.T) {
		tradingRepo := &mockTradingRepo{trades: trades}
		analyticsRepo := &mockAnalyticsRepo{}
		pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000, epoch)

		if err := pa.UpdateMetrics(context.Background()); err != nil {
			t.Fatalf("UpdateMetrics failed: %v", err)
		}
		if !tradingRepo.sinceArg.Equal(epoch) {
			t.Errorf("expected GetTradesSince(%v), got %v", epoch, tradingRepo.sinceArg)
		}
		if len(analyticsRepo.savedMetrics) != 1 {
			t.Fatalf("expected 1 saved metric, got %d", len(analyticsRepo.savedMetrics))
		}
		metric := analyticsRepo.savedMetrics[0]
		if math.Abs(metric.TotalPnL-(-100)) > 0.01 {
			t.Errorf("expected total PnL -100, got %.2f", metric.TotalPnL)
		}
		if metric.TotalTrades != 2 {
			t.Errorf("expected 2 trades in scope, got %d", metric.TotalTrades)
		}
	})

	t.Run("zero epoch keeps the full history", func(t *testing.T) {
		tradingRepo := &mockTradingRepo{trades: trades}
		analyticsRepo := &mockAnalyticsRepo{}
		pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000, time.Time{})

		if err := pa.UpdateMetrics(context.Background()); err != nil {
			t.Fatalf("UpdateMetrics failed: %v", err)
		}
		metric := analyticsRepo.savedMetrics[0]
		if math.Abs(metric.TotalPnL-817) > 0.01 {
			t.Errorf("expected total PnL 817, got %.2f", metric.TotalPnL)
		}
	})

	t.Run("no trades after epoch saves nothing", func(t *testing.T) {
		tradingRepo := &mockTradingRepo{trades: trades[:2]}
		analyticsRepo := &mockAnalyticsRepo{}
		pa := NewPerformanceAnalytics(tradingRepo, analyticsRepo, nil, 100000, epoch)

		if err := pa.UpdateMetrics(context.Background()); err != nil {
			t.Fatalf("UpdateMetrics failed: %v", err)
		}
		if len(analyticsRepo.savedMetrics) != 0 {
			t.Errorf("expected no saved metrics, got %d", len(analyticsRepo.savedMetrics))
		}
	})
}

// TestTradesInScope_OrdersAscending guards the drawdown calculation, which
// walks the slice as a running equity curve and silently produces nonsense if
// the newest-first order of GetTradesSince is passed through.
func TestTradesInScope_OrdersAscending(t *testing.T) {
	epoch := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	tradingRepo := &mockTradingRepo{trades: []domain.Trade{
		{PnL: 10, ExecutedAt: epoch.Add(3 * time.Hour)},
		{PnL: 20, ExecutedAt: epoch.Add(1 * time.Hour)},
		{PnL: 30, ExecutedAt: epoch.Add(2 * time.Hour)},
	}}
	pa := NewPerformanceAnalytics(tradingRepo, &mockAnalyticsRepo{}, nil, 100000, epoch)

	got, err := pa.tradesInScope()
	if err != nil {
		t.Fatalf("tradesInScope failed: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i].ExecutedAt.Before(got[i-1].ExecutedAt) {
			t.Fatalf("trades are not in ascending order: %v", got)
		}
	}
}

func TestProfitFactor(t *testing.T) {
	pa := NewPerformanceAnalytics(nil, nil, nil, 100000, time.Time{})

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
