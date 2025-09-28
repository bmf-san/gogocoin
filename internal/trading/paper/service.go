package paper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
)

// Trader is the paper trading implementation
// It simulates trading in memory
type Trader struct {
	client       *bitflyer.Client
	logger       *logger.Logger
	orders       map[string]*domain.OrderResult
	balance      *BalanceManager
	pnl          *PnLCalculator
	db           trading.DatabaseSaver
	strategyName string
	mu           sync.RWMutex
}

// NewTrader creates a new paper trader
func NewTrader(client *bitflyer.Client, log *logger.Logger, initialBalance, feeRate float64) *Trader {
	s := &Trader{
		client:  client,
		logger:  log,
		orders:  make(map[string]*domain.OrderResult),
		balance: NewBalanceManager(initialBalance),
		mu:      sync.RWMutex{},
	}

	s.pnl = NewPnLCalculator(feeRate, s.logger)

	return s
}

// PlaceOrder executes an order (simulation)
func (t *Trader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate input
	if err := t.validateOrder(order); err != nil {
		return nil, fmt.Errorf("invalid order: %w", err)
	}

	// Check balance
	if err := t.balance.CheckBalance(order); err != nil {
		return nil, fmt.Errorf("insufficient balance: %w", err)
	}

	// Generate order ID
	orderID := fmt.Sprintf("paper_%d", time.Now().UnixNano())

	// Branch processing based on order type
	var status string
	var filledSize, remainingSize float64

	if order.Type == "MARKET" {
		// Market orders are filled immediately
		status = "COMPLETED"
		filledSize = order.Size
		remainingSize = 0
	} else {
		// Limit orders remain active for a certain period
		status = "ACTIVE"
		filledSize = 0
		remainingSize = order.Size
	}

	// Calculate fee (only when filled)
	var fee float64
	if status == "COMPLETED" {
		fee = t.pnl.CalculateFee(order.Size, order.Price)
	}

	// Create order result
	result := &domain.OrderResult{
		OrderID:         orderID,
		Symbol:          order.Symbol,
		Side:            order.Side,
		Type:            order.Type,
		Size:            order.Size,
		Price:           order.Price,
		Status:          status,
		FilledSize:      filledSize,
		RemainingSize:   remainingSize,
		AveragePrice:    order.Price,
		TotalCommission: fee,
		Fee:             fee,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Record order
	t.orders[orderID] = result

	// For market orders, update balance and record trade immediately
	if status == "COMPLETED" {
		// Update balance
		if err := t.balance.UpdateBalance(order, fee); err != nil {
			return nil, fmt.Errorf("failed to update balance: %w", err)
		}

		// Save balance to database
		t.saveBalanceToDB()

		// Save trade to database
		t.saveTradeToDB(result)
	} else {
		// For limit orders, process execution asynchronously
		go t.monitorOrderExecution(ctx, result)
	}

	t.logger.LogTrade("PAPER_ORDER", order.Symbol, order.Price, order.Size, map[string]interface{}{
		"order_id": orderID,
		"side":     order.Side,
		"type":     order.Type,
		"status":   status,
		"fee":      fee,
	})

	return result, nil
}

// CancelOrder cancels an order
func (t *Trader) CancelOrder(ctx context.Context, orderID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if order, exists := t.orders[orderID]; exists {
		if order.Status == "ACTIVE" {
			order.Status = "CANCELED"
			order.UpdatedAt = time.Now()
			t.logger.Trading().WithField("order_id", orderID).Info("Paper order canceled")
		}
		return nil
	}

	return fmt.Errorf("order not found: %s", orderID)
}

// GetBalance retrieves balance information
func (t *Trader) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.balance.GetAll(), nil
}

// GetOrders retrieves the list of orders
func (t *Trader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var orders []*domain.OrderResult
	for _, order := range t.orders {
		orders = append(orders, order)
	}
	return orders, nil
}

// SetDatabase sets the database saver interface
func (t *Trader) SetDatabase(db trading.DatabaseSaver) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.db = db

	// Save initial balance to database
	if db != nil {
		t.saveBalanceToDB()
	}
}

// SetStrategyName sets the strategy name
func (t *Trader) SetStrategyName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.strategyName = name
}

// validateOrder validates the order
func (t *Trader) validateOrder(order *domain.OrderRequest) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if order.Side != "BUY" && order.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}
	if order.Size <= 0 {
		return fmt.Errorf("size must be positive: %f", order.Size)
	}
	if order.Type == "LIMIT" && order.Price <= 0 {
		return fmt.Errorf("price must be positive for LIMIT orders")
	}
	return nil
}

// monitorOrderExecution monitors paper order execution
func (t *Trader) monitorOrderExecution(ctx context.Context, result *domain.OrderResult) {
	// Execute randomly after 5-30 seconds (mimicking real exchanges)
	executionDelay := time.Duration(5+time.Now().UnixNano()%25) * time.Second

	select {
	case <-ctx.Done():
		t.logger.Trading().WithField("order_id", result.OrderID).Info("Paper order monitoring canceled")
		return
	case <-time.After(executionDelay):
		t.executeOrder(result)
	}
}

// executeOrder executes a paper order
func (t *Trader) executeOrder(result *domain.OrderResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Update order status
	result.Status = "COMPLETED"
	result.FilledSize = result.Size
	result.RemainingSize = 0
	result.UpdatedAt = time.Now()

	// Calculate fee
	fee := t.pnl.CalculateFee(result.FilledSize, result.AveragePrice)
	result.Fee = fee
	result.TotalCommission = fee

	// Reconstruct order request (for balance update)
	order := &domain.OrderRequest{
		Symbol: result.Symbol,
		Side:   result.Side,
		Type:   result.Type,
		Size:   result.Size,
		Price:  result.Price,
	}

	// Update balance
	if err := t.balance.UpdateBalance(order, fee); err != nil {
		t.logger.Trading().WithError(err).Error("Failed to update balance")
		return
	}

	// Save balance to database
	t.saveBalanceToDB()

	// Save trade to database
	t.saveTradeToDB(result)

	t.logger.Trading().
		WithField("order_id", result.OrderID).
		WithField("execution_delay", time.Since(result.CreatedAt)).
		Info("Paper order executed")
}

// saveTradeToDB saves trade to database
func (t *Trader) saveTradeToDB(result *domain.OrderResult) {
	if t.db == nil {
		return
	}

	strategyName := t.strategyName
	if strategyName == "" {
		strategyName = "unknown"
	}

	// Calculate PnL (SELL orders only)
	var pnl float64
	if result.Side == "SELL" {
		pnl = t.pnl.CalculatePnL(result, t.db)
	}

	trade := &domain.Trade{
		Symbol:       result.Symbol,
		Side:         result.Side,
		Type:         result.Type,
		Size:         result.FilledSize,
		Price:        result.AveragePrice,
		Status:       result.Status,
		Fee:          result.Fee,
		ExecutedAt:   result.UpdatedAt,
		CreatedAt:    result.CreatedAt,
		OrderID:      result.OrderID,
		StrategyName: strategyName,
		PnL:          pnl,
	}

	if err := t.db.SaveTrade(trade); err != nil {
		t.logger.Trading().WithError(err).Error("Failed to save trade to database")
	}
}

// saveBalanceToDB saves balance to database
func (t *Trader) saveBalanceToDB() {
	if t.db == nil {
		return
	}

	balances := t.balance.GetAll()
	for _, bal := range balances {
		if err := t.db.SaveBalance(bal); err != nil {
			t.logger.Trading().WithError(err).
				WithField("currency", bal.Currency).
				Error("Failed to save balance to database")
		}
	}
}
