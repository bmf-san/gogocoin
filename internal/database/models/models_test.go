package models

import (
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

func TestTradeModel_ToDomain(t *testing.T) {
	now := time.Now()
	model := &TradeModel{
		ID:         1,
		OrderID:    "ORDER123",
		Symbol:     "BTC_JPY",
		Side:       "BUY",
		Amount:     10000.0,
		Size:       0.01,
		Price:      1000000.0,
		Fee:        10.0,
		PnL:        100.0,
		Status:     "COMPLETED",
		Strategy:   "scalping",
		ExecutedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	trade := model.ToDomain()

	if trade.ID != 1 {
		t.Errorf("Expected ID 1, got %d", trade.ID)
	}
	if trade.OrderID != "ORDER123" {
		t.Errorf("Expected OrderID ORDER123, got %s", trade.OrderID)
	}
	if trade.Symbol != "BTC_JPY" {
		t.Errorf("Expected Symbol BTC_JPY, got %s", trade.Symbol)
	}
}

func TestFromDomainTrade(t *testing.T) {
	now := time.Now()
	trade := domain.Trade{
		ID:         2,
		OrderID:    "ORDER456",
		Symbol:     "ETH_JPY",
		Side:       "SELL",
		Amount:     5000.0,
		Size:       0.1,
		Price:      50000.0,
		Fee:        5.0,
		PnL:        50.0,
		Status:     "COMPLETED",
		Strategy:   "scalping",
		ExecutedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	model := FromDomainTrade(trade)

	if model.ID != 2 {
		t.Errorf("Expected ID 2, got %d", model.ID)
	}
	if model.OrderID != "ORDER456" {
		t.Errorf("Expected OrderID ORDER456, got %s", model.OrderID)
	}
	if model.Symbol != "ETH_JPY" {
		t.Errorf("Expected Symbol ETH_JPY, got %s", model.Symbol)
	}
}

func TestPositionModel_ToDomain(t *testing.T) {
	now := time.Now()
	model := &PositionModel{
		ID:           1,
		Symbol:       "BTC_JPY",
		Side:         "LONG",
		EntryPrice:   1000000.0,
		CurrentPrice: 1010000.0,
		Size:         0.01,
		UnrealizedPL: 100.0,
		RealizedPL:   0.0,
		Status:       "OPEN",
		Strategy:     "scalping",
		OpenedAt:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	position := model.ToDomain()

	if position.ID != 1 {
		t.Errorf("Expected ID 1, got %d", position.ID)
	}
	if position.Symbol != "BTC_JPY" {
		t.Errorf("Expected Symbol BTC_JPY, got %s", position.Symbol)
	}
	if position.EntryPrice != 1000000.0 {
		t.Errorf("Expected EntryPrice 1000000.0, got %f", position.EntryPrice)
	}
}

func TestFromDomainPosition(t *testing.T) {
	now := time.Now()
	position := domain.Position{
		ID:           2,
		Symbol:       "ETH_JPY",
		Side:         "SHORT",
		EntryPrice:   50000.0,
		CurrentPrice: 49000.0,
		Size:         0.1,
		UnrealizedPL: 100.0,
		PnL:          0.0,
		Status:       "OPEN",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	model := FromDomainPosition(position)

	if model.ID != 2 {
		t.Errorf("Expected ID 2, got %d", model.ID)
	}
	if model.Symbol != "ETH_JPY" {
		t.Errorf("Expected Symbol ETH_JPY, got %s", model.Symbol)
	}
	if model.Size != 0.1 {
		t.Errorf("Expected Size 0.1, got %f", model.Size)
	}
}

func TestBalanceModel_ToDomain(t *testing.T) {
	now := time.Now()
	model := &BalanceModel{
		ID:        1,
		Currency:  "BTC",
		Amount:    1.0,
		Available: 0.9,
		Reserved:  0.1,
		Timestamp: now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	balance := model.ToDomain()

	if balance.Currency != "BTC" {
		t.Errorf("Expected Currency BTC, got %s", balance.Currency)
	}
	if balance.Amount != 1.0 {
		t.Errorf("Expected Amount 1.0, got %f", balance.Amount)
	}
	if balance.Available != 0.9 {
		t.Errorf("Expected Available 0.9, got %f", balance.Available)
	}
}

func TestFromDomainBalance(t *testing.T) {
	now := time.Now()
	balance := domain.Balance{
		Currency:  "JPY",
		Amount:    100000.0,
		Available: 95000.0,
		Timestamp: now,
	}

	model := FromDomainBalance(balance)

	if model.Currency != "JPY" {
		t.Errorf("Expected Currency JPY, got %s", model.Currency)
	}
	if model.Amount != 100000.0 {
		t.Errorf("Expected Amount 100000.0, got %f", model.Amount)
	}
	if model.Available != 95000.0 {
		t.Errorf("Expected Available 95000.0, got %f", model.Available)
	}
}
