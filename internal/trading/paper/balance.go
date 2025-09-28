package paper

import (
	"fmt"
	"strings"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// BalanceManager manages balances for paper trading
type BalanceManager struct {
	balances map[string]*domain.Balance
}

// NewBalanceManager creates a new balance manager
func NewBalanceManager(initialBalance float64) *BalanceManager {
	bm := &BalanceManager{
		balances: make(map[string]*domain.Balance),
	}

	// Set initial balance
	if initialBalance <= 0 {
		initialBalance = 1000000.0 // Default: 1 million JPY
	}

	now := time.Now()

	// JPY balance
	bm.balances["JPY"] = &domain.Balance{
		Currency:  "JPY",
		Amount:    initialBalance,
		Available: initialBalance,
		Timestamp: now,
	}

	// Initial balance for cryptocurrencies is 0
	cryptoCurrencies := []string{"BTC", "ETH", "XRP", "XLM", "MONA"}
	for _, currency := range cryptoCurrencies {
		bm.balances[currency] = &domain.Balance{
			Currency:  currency,
			Amount:    0.0,
			Available: 0.0,
			Timestamp: now,
		}
	}

	return bm
}

// CheckBalance checks if balance is sufficient
func (bm *BalanceManager) CheckBalance(order *domain.OrderRequest) error {
	if order.Side == "BUY" {
		// Buy order: check JPY balance
		jpyBalance := bm.balances["JPY"]
		requiredAmount := order.Size * order.Price
		if jpyBalance.Available < requiredAmount {
			return fmt.Errorf("insufficient JPY balance: required %f, available %f",
				requiredAmount, jpyBalance.Available)
		}
	} else {
		// Sell order: check currency balance
		currency := getCurrencyFromSymbol(order.Symbol)
		balance, exists := bm.balances[currency]
		if !exists {
			return fmt.Errorf("no %s balance found", currency)
		}
		if balance.Available < order.Size {
			return fmt.Errorf("insufficient %s balance: required %f, available %f",
				currency, order.Size, balance.Available)
		}
	}

	return nil
}

// UpdateBalance updates balance
func (bm *BalanceManager) UpdateBalance(order *domain.OrderRequest, fee float64) error {
	now := time.Now()

	if order.Side == "BUY" {
		// Buy order: decrease JPY, increase currency
		jpyBalance := bm.balances["JPY"]
		currency := getCurrencyFromSymbol(order.Symbol)
		cryptoBalance := bm.balances[currency]

		totalCost := order.Size*order.Price + fee
		jpyBalance.Amount -= totalCost
		jpyBalance.Available -= totalCost
		jpyBalance.Timestamp = now

		cryptoBalance.Amount += order.Size
		cryptoBalance.Available += order.Size
		cryptoBalance.Timestamp = now
	} else {
		// Sell order: decrease currency, increase JPY
		currency := getCurrencyFromSymbol(order.Symbol)
		cryptoBalance := bm.balances[currency]
		jpyBalance := bm.balances["JPY"]

		cryptoBalance.Amount -= order.Size
		cryptoBalance.Available -= order.Size
		cryptoBalance.Timestamp = now

		totalRevenue := order.Size*order.Price - fee
		jpyBalance.Amount += totalRevenue
		jpyBalance.Available += totalRevenue
		jpyBalance.Timestamp = now
	}

	return nil
}

// GetAll retrieves all balances
func (bm *BalanceManager) GetAll() []domain.Balance {
	var balances []domain.Balance
	for _, balance := range bm.balances {
		balances = append(balances, *balance)
	}
	return balances
}

// Get retrieves balance for specified currency
func (bm *BalanceManager) Get(currency string) (*domain.Balance, bool) {
	balance, exists := bm.balances[currency]
	return balance, exists
}

// getCurrencyFromSymbol extracts currency code from symbol
// Example: BTC_JPY -> BTC, ETH_JPY -> ETH
func getCurrencyFromSymbol(symbol string) string {
	parts := strings.Split(symbol, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return symbol
}
