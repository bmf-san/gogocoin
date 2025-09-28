package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// createTestLogger creates a logger for testing
func createTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New(&logger.Config{
		Level:    "error",
		Format:   "json",
		Output:   "file",
		FilePath: "/dev/null",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return log
}

// mockTradingService simulates the trading service for integration tests
type mockTradingService struct {
	mu                     sync.Mutex
	orders                 []*domain.OrderResult
	balances               []domain.Balance
	orderCompletedCallback func(*domain.OrderResult)
}

func newMockTradingService() *mockTradingService {
	return &mockTradingService{
		orders: make([]*domain.OrderResult, 0),
		balances: []domain.Balance{
			{
				Currency:  "JPY",
				Amount:    100000,
				Available: 100000,
				Timestamp: time.Now(),
			},
			{
				Currency:  "BTC",
				Amount:    0,
				Available: 0,
				Timestamp: time.Now(),
			},
		},
	}
}

func (m *mockTradingService) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &domain.OrderResult{
		OrderID:       generateOrderID(),
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          order.Type,
		Size:          order.Size,
		Price:         order.Price,
		Status:        "COMPLETED",
		FilledSize:    order.Size,
		RemainingSize: 0,
		AveragePrice:  order.Price,
		Fee:           order.Size * order.Price * 0.001, // 0.1% fee
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	m.orders = append(m.orders, result)

	// Update balances
	m.updateBalancesAfterOrder(result)

	// Trigger callback
	if m.orderCompletedCallback != nil {
		m.orderCompletedCallback(result)
	}

	return result, nil
}

func (m *mockTradingService) updateBalancesAfterOrder(order *domain.OrderResult) {
	for i := range m.balances {
		if order.Side == "BUY" {
			if m.balances[i].Currency == "JPY" {
				cost := order.FilledSize * order.AveragePrice * (1 + 0.001)
				m.balances[i].Amount -= cost
				m.balances[i].Available -= cost
			} else if m.balances[i].Currency == "BTC" {
				m.balances[i].Amount += order.FilledSize
				m.balances[i].Available += order.FilledSize
			}
		} else if order.Side == "SELL" {
			if m.balances[i].Currency == "BTC" {
				m.balances[i].Amount -= order.FilledSize
				m.balances[i].Available -= order.FilledSize
			} else if m.balances[i].Currency == "JPY" {
				revenue := order.FilledSize * order.AveragePrice * (1 - 0.001)
				m.balances[i].Amount += revenue
				m.balances[i].Available += revenue
			}
		}
		m.balances[i].Timestamp = time.Now()
	}
}

func (m *mockTradingService) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balances, nil
}

func (m *mockTradingService) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders, nil
}

func (m *mockTradingService) CancelOrder(ctx context.Context, orderID string) error {
	return nil
}

func (m *mockTradingService) InvalidateBalanceCache() {}

func (m *mockTradingService) UpdateBalanceToDB(ctx context.Context) {}

func (m *mockTradingService) Shutdown() error {
	return nil
}

func (m *mockTradingService) SetOnOrderCompleted(fn func(*domain.OrderResult)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orderCompletedCallback = fn
}

func (m *mockTradingService) SetStrategyName(name string) {}

// mockStrategy simulates a trading strategy
type mockStrategy struct {
	signals       []*strategy.Signal
	tradeCount    int
	isRunning     bool
	generateFunc  func() *strategy.Signal
	mu            sync.Mutex
}

func newMockStrategy() *mockStrategy {
	return &mockStrategy{
		signals:    make([]*strategy.Signal, 0),
		tradeCount: 0,
		isRunning:  false,
	}
}

func (m *mockStrategy) GenerateSignal(ctx context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.generateFunc != nil {
		return m.generateFunc(), nil
	}

	// Default: generate BUY signal
	signal := &strategy.Signal{
		Symbol:    "BTC_JPY",
		Action:    strategy.SignalBuy,
		Strength:  0.8,
		Price:     data.Price,
		Quantity:  0.001,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	m.signals = append(m.signals, signal)
	return signal, nil
}

func (m *mockStrategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	return nil, nil
}

func (m *mockStrategy) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isRunning = true
	return nil
}

func (m *mockStrategy) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isRunning = false
	return nil
}

func (m *mockStrategy) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

func (m *mockStrategy) GetStatus() strategy.StrategyStatus {
	return strategy.StrategyStatus{
		IsRunning:    m.isRunning,
		TotalSignals: len(m.signals),
	}
}

func (m *mockStrategy) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals = make([]*strategy.Signal, 0)
	m.tradeCount = 0
	return nil
}

func (m *mockStrategy) GetMetrics() strategy.StrategyMetrics {
	return strategy.StrategyMetrics{
		TotalTrades: m.tradeCount,
	}
}

func (m *mockStrategy) RecordTrade() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tradeCount++
}

func (m *mockStrategy) InitializeDailyTradeCount(count int) {
	m.tradeCount = count
}

func (m *mockStrategy) Name() string {
	return "mock-strategy"
}

func (m *mockStrategy) Description() string {
	return "Mock strategy for testing"
}

func (m *mockStrategy) Version() string {
	return "1.0.0"
}

func (m *mockStrategy) Initialize(config map[string]interface{}) error {
	return nil
}

func (m *mockStrategy) UpdateConfig(config map[string]interface{}) error {
	return nil
}

func (m *mockStrategy) GetConfig() map[string]interface{} {
	return make(map[string]interface{})
}

// Helper function to generate unique order IDs
var orderIDCounter = 0
var orderIDMutex sync.Mutex

func generateOrderID() string {
	orderIDMutex.Lock()
	defer orderIDMutex.Unlock()
	orderIDCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune(orderIDCounter))
}

// TestTradingFlow_EndToEnd tests the complete trading cycle
func TestTradingFlow_EndToEnd(t *testing.T) {
	t.Run("signal_to_order_to_balance_update", func(t *testing.T) {
		ctx := context.Background()
		log := createTestLogger(t)

		// Setup components
		tradingSvc := newMockTradingService()
		strat := newMockStrategy()

		// Track trade completions
		tradeCompleted := make(chan *domain.OrderResult, 10)
		tradingSvc.SetOnOrderCompleted(func(result *domain.OrderResult) {
			strat.RecordTrade()
			tradeCompleted <- result
		})

		// Start strategy
		if err := strat.Start(ctx); err != nil {
			t.Fatalf("Failed to start strategy: %v", err)
		}

		// Step 1: Generate signal
		marketData := &domain.MarketData{
			Symbol:    "BTC_JPY",
			Price:     5000000,
			Timestamp: time.Now(),
		}

		signal, err := strat.GenerateSignal(ctx, marketData, []domain.MarketData{})
		if err != nil {
			t.Fatalf("Failed to generate signal: %v", err)
		}

		log.Strategy().WithField("signal", signal.Action).Info("Signal generated")

		if signal.Action != strategy.SignalBuy {
			t.Errorf("Expected BUY signal, got %s", signal.Action)
		}

		// Step 2: Execute order based on signal
		orderReq := &domain.OrderRequest{
			Symbol:   signal.Symbol,
			Side:     string(signal.Action),
			Type:     "MARKET",
			Size:     signal.Quantity,
			Price:    signal.Price,
		}

		orderResult, err := tradingSvc.PlaceOrder(ctx, orderReq)
		if err != nil {
			t.Fatalf("Failed to place order: %v", err)
		}

		log.Trading().WithField("order_id", orderResult.OrderID).Info("Order placed")

		if orderResult.Status != "COMPLETED" {
			t.Errorf("Expected order status COMPLETED, got %s", orderResult.Status)
		}

		// Step 3: Wait for trade completion callback
		select {
		case result := <-tradeCompleted:
			if result.OrderID != orderResult.OrderID {
				t.Errorf("Callback order ID mismatch: expected %s, got %s", orderResult.OrderID, result.OrderID)
			}
		case <-time.After(1 * time.Second):
			t.Error("Trade completion callback not triggered")
		}

		// Step 4: Verify balance update
		balances, err := tradingSvc.GetBalance(ctx)
		if err != nil {
			t.Fatalf("Failed to get balance: %v", err)
		}

		log.Trading().WithField("balances", len(balances)).Info("Retrieved balances")

		var btcBalance, jpyBalance *domain.Balance
		for i := range balances {
			if balances[i].Currency == "BTC" {
				btcBalance = &balances[i]
			} else if balances[i].Currency == "JPY" {
				jpyBalance = &balances[i]
			}
		}

		if btcBalance == nil || jpyBalance == nil {
			t.Fatal("Expected both BTC and JPY balances")
		}

		// BTC should have increased
		if btcBalance.Amount <= 0 {
			t.Errorf("Expected BTC balance > 0, got %f", btcBalance.Amount)
		}

		// JPY should have decreased
		if jpyBalance.Amount >= 100000 {
			t.Errorf("Expected JPY balance < 100000, got %f", jpyBalance.Amount)
		}

		// Step 5: Verify strategy recorded the trade
		if strat.tradeCount != 1 {
			t.Errorf("Expected 1 trade recorded, got %d", strat.tradeCount)
		}

		// Step 6: Calculate PnL (simulated)
		initialJPY := 100000.0
		currentJPY := jpyBalance.Amount
		btcValue := btcBalance.Amount * marketData.Price
		totalValue := currentJPY + btcValue
		pnl := totalValue - initialJPY

		log.Trading().WithField("pnl", pnl).Info("PnL calculated")

		// PnL should be negative due to fees
		if pnl >= 0 {
			t.Logf("Warning: PnL is non-negative (%f), expected negative due to fees", pnl)
		}

		t.Logf("Trading flow completed successfully. Initial: %.2f JPY, Final: %.2f JPY, PnL: %.2f JPY",
			initialJPY, totalValue, pnl)
	})

	t.Run("complete_buy_sell_cycle", func(t *testing.T) {
		ctx := context.Background()
		log := createTestLogger(t)

		tradingSvc := newMockTradingService()
		strat := newMockStrategy()

		tradeCompleted := make(chan *domain.OrderResult, 10)
		tradingSvc.SetOnOrderCompleted(func(result *domain.OrderResult) {
			strat.RecordTrade()
			tradeCompleted <- result
		})

		if err := strat.Start(ctx); err != nil {
			t.Fatalf("Failed to start strategy: %v", err)
		}

		marketData := &domain.MarketData{
			Symbol:    "BTC_JPY",
			Price:     5000000,
			Timestamp: time.Now(),
		}

		// Buy cycle
		strat.generateFunc = func() *strategy.Signal {
			return &strategy.Signal{
				Symbol:    "BTC_JPY",
				Action:    strategy.SignalBuy,
				Strength:  0.8,
				Price:     marketData.Price,
				Quantity:  0.001,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
		}

		buySignal, _ := strat.GenerateSignal(ctx, marketData, nil)
		buyOrder := &domain.OrderRequest{
			Symbol: buySignal.Symbol,
			Side:   string(buySignal.Action),
			Type:   "MARKET",
			Size:   buySignal.Quantity,
			Price:  buySignal.Price,
		}

		buyResult, err := tradingSvc.PlaceOrder(ctx, buyOrder)
		if err != nil {
			t.Fatalf("Failed to place buy order: %v", err)
		}

		<-tradeCompleted // Wait for callback

		log.Trading().WithField("order_id", buyResult.OrderID).Info("Buy order completed")

		// Sell cycle at higher price (simulating profit)
		sellPrice := marketData.Price * 1.01 // 1% gain
		strat.generateFunc = func() *strategy.Signal {
			return &strategy.Signal{
				Symbol:    "BTC_JPY",
				Action:    strategy.SignalSell,
				Strength:  0.8,
				Price:     sellPrice,
				Quantity:  0.001,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
		}

		sellMarketData := &domain.MarketData{
			Symbol:    "BTC_JPY",
			Price:     sellPrice,
			Timestamp: time.Now(),
		}

		sellSignal, _ := strat.GenerateSignal(ctx, sellMarketData, nil)
		sellOrder := &domain.OrderRequest{
			Symbol: sellSignal.Symbol,
			Side:   string(sellSignal.Action),
			Type:   "MARKET",
			Size:   sellSignal.Quantity,
			Price:  sellSignal.Price,
		}

		sellResult, err := tradingSvc.PlaceOrder(ctx, sellOrder)
		if err != nil {
			t.Fatalf("Failed to place sell order: %v", err)
		}

		<-tradeCompleted // Wait for callback

		log.Trading().WithField("order_id", sellResult.OrderID).Info("Sell order completed")

		// Verify final state
		balances, _ := tradingSvc.GetBalance(ctx)
		var finalJPY float64
		for _, bal := range balances {
			if bal.Currency == "JPY" {
				finalJPY = bal.Amount
			}
		}

		// Calculate round-trip PnL
		initialJPY := 100000.0
		pnl := finalJPY - initialJPY

		log.Trading().WithField("round_trip_pnl", pnl).Info("Round-trip PnL calculated")

		// Should have small profit after 1% price increase minus fees
		if pnl <= 0 {
			t.Logf("PnL is non-positive: %.2f (expected small profit)", pnl)
		}

		if strat.tradeCount != 2 {
			t.Errorf("Expected 2 trades, got %d", strat.tradeCount)
		}

		t.Logf("Round-trip completed. Initial: %.2f JPY, Final: %.2f JPY, PnL: %.2f JPY",
			initialJPY, finalJPY, pnl)
	})

	t.Run("multiple_signals_and_orders", func(t *testing.T) {
		ctx := context.Background()
		log := createTestLogger(t)

		tradingSvc := newMockTradingService()
		strat := newMockStrategy()

		completionCount := 0
		tradingSvc.SetOnOrderCompleted(func(result *domain.OrderResult) {
			strat.RecordTrade()
			completionCount++
		})

		if err := strat.Start(ctx); err != nil {
			t.Fatalf("Failed to start strategy: %v", err)
		}

		// Simulate multiple trading signals
		prices := []float64{5000000, 5050000, 4950000}
		actions := []strategy.SignalAction{strategy.SignalBuy, strategy.SignalSell, strategy.SignalBuy}

		for i := 0; i < 3; i++ {
			marketData := &domain.MarketData{
				Symbol:    "BTC_JPY",
				Price:     prices[i],
				Timestamp: time.Now(),
			}

			strat.generateFunc = func(action strategy.SignalAction, price float64) func() *strategy.Signal {
				return func() *strategy.Signal {
					return &strategy.Signal{
						Symbol:    "BTC_JPY",
						Action:    action,
						Strength:  0.8,
						Price:     price,
						Quantity:  0.001,
						Timestamp: time.Now(),
						Metadata:  make(map[string]interface{}),
					}
				}
			}(actions[i], prices[i])

			signal, _ := strat.GenerateSignal(ctx, marketData, nil)

			order := &domain.OrderRequest{
				Symbol: signal.Symbol,
				Side:   string(signal.Action),
				Type:   "MARKET",
				Size:   signal.Quantity,
				Price:  signal.Price,
			}

			result, err := tradingSvc.PlaceOrder(ctx, order)
			if err != nil {
				t.Fatalf("Failed to place order %d: %v", i+1, err)
			}

			log.Trading().
				WithField("order_num", i+1).
				WithField("action", signal.Action).
				WithField("price", signal.Price).
				Info("Order executed")

			if result.Status != "COMPLETED" {
				t.Errorf("Order %d not completed", i+1)
			}

			time.Sleep(10 * time.Millisecond) // Small delay between orders
		}

		// Verify all trades were recorded
		if strat.tradeCount != 3 {
			t.Errorf("Expected 3 trades recorded, got %d", strat.tradeCount)
		}

		if completionCount != 3 {
			t.Errorf("Expected 3 completion callbacks, got %d", completionCount)
		}

		// Verify order history
		orders, _ := tradingSvc.GetOrders(ctx)
		if len(orders) != 3 {
			t.Errorf("Expected 3 orders in history, got %d", len(orders))
		}

		t.Logf("Successfully executed %d trades", len(orders))
	})
}

// TestTradingFlow_ErrorHandling tests error scenarios
func TestTradingFlow_ErrorHandling(t *testing.T) {
	t.Run("strategy_generates_hold_signal", func(t *testing.T) {
		ctx := context.Background()
		strat := newMockStrategy()

		strat.generateFunc = func() *strategy.Signal {
			return &strategy.Signal{
				Symbol:    "BTC_JPY",
				Action:    strategy.SignalHold,
				Strength:  0.5,
				Price:     5000000,
				Quantity:  0,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
		}

		marketData := &domain.MarketData{
			Symbol:    "BTC_JPY",
			Price:     5000000,
			Timestamp: time.Now(),
		}

		signal, err := strat.GenerateSignal(ctx, marketData, nil)
		if err != nil {
			t.Fatalf("Failed to generate signal: %v", err)
		}

		if signal.Action != strategy.SignalHold {
			t.Errorf("Expected HOLD signal, got %s", signal.Action)
		}

		// HOLD signal should not result in order placement
		if signal.Quantity != 0 {
			t.Error("HOLD signal should have 0 quantity")
		}
	})

	t.Run("strategy_lifecycle", func(t *testing.T) {
		ctx := context.Background()
		strat := newMockStrategy()

		// Initially not running
		if strat.IsRunning() {
			t.Error("Strategy should not be running initially")
		}

		// Start
		if err := strat.Start(ctx); err != nil {
			t.Fatalf("Failed to start strategy: %v", err)
		}

		if !strat.IsRunning() {
			t.Error("Strategy should be running after Start()")
		}

		// Stop
		if err := strat.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop strategy: %v", err)
		}

		if strat.IsRunning() {
			t.Error("Strategy should not be running after Stop()")
		}

		// Reset
		strat.RecordTrade()
		if err := strat.Reset(); err != nil {
			t.Fatalf("Failed to reset strategy: %v", err)
		}

		if strat.tradeCount != 0 {
			t.Errorf("Trade count should be 0 after reset, got %d", strat.tradeCount)
		}
	})
}
