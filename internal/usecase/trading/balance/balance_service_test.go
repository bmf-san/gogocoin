package balance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func TestInvalidateCache(t *testing.T) {
	log := logger.NewNopLogger()

	service := NewBalanceService(nil, nil, log)

	// Set cache data manually
	service.cache.mu.Lock()
	service.cache.data = []domain.Balance{
		{Currency: "BTC", Amount: 1.0, Available: 1.0},
	}
	service.cache.timestamp = time.Now()
	service.cache.mu.Unlock()

	// Verify cache is set
	service.cache.mu.RLock()
	if service.cache.timestamp.IsZero() {
		t.Fatal("Cache timestamp should not be zero before invalidation")
	}
	service.cache.mu.RUnlock()

	// Invalidate cache
	service.InvalidateCache()

	// Verify cache is invalidated
	service.cache.mu.RLock()
	defer service.cache.mu.RUnlock()
	if !service.cache.timestamp.IsZero() {
		t.Error("Cache timestamp should be zero after invalidation")
	}
}

func TestCacheConcurrency(t *testing.T) {
	log := logger.NewNopLogger()

	service := NewBalanceService(nil, nil, log)

	// Set initial cache
	service.cache.mu.Lock()
	service.cache.data = []domain.Balance{
		{Currency: "BTC", Amount: 1.0, Available: 1.0},
	}
	service.cache.timestamp = time.Now()
	service.cache.mu.Unlock()

	// Test concurrent reads and invalidations
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				service.cache.mu.RLock()
				_ = service.cache.data
				_ = service.cache.timestamp
				service.cache.mu.RUnlock()
			}
		}()
	}

	// Concurrent invalidators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				service.InvalidateCache()
			}
		}()
	}

	// Concurrent writers (simulating cache update)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				service.cache.mu.Lock()
				service.cache.data = []domain.Balance{
					{Currency: "BTC", Amount: float64(j), Available: float64(j)},
				}
				service.cache.timestamp = time.Now()
				service.cache.mu.Unlock()
			}
		}()
	}

	wg.Wait()
	// If we reach here without data race, the test passes
}

func TestCacheExpiration(t *testing.T) {
	log := logger.NewNopLogger()

	service := NewBalanceService(nil, nil, log)

	// Set cache with expired timestamp
	service.cache.mu.Lock()
	service.cache.data = []domain.Balance{
		{Currency: "BTC", Amount: 1.0, Available: 1.0},
	}
	service.cache.timestamp = time.Now().Add(-CacheDuration - time.Second)
	service.cache.mu.Unlock()

	// Read cache values
	service.cache.mu.RLock()
	cacheTimestamp := service.cache.timestamp
	cacheData := service.cache.data
	service.cache.mu.RUnlock()

	// Check if cache is considered expired
	if time.Since(cacheTimestamp) < CacheDuration {
		t.Error("Cache should be expired")
	}

	if len(cacheData) == 0 {
		t.Error("Cache data should exist even if expired")
	}
}

// Mock balance repository for testing
type mockBalanceRepository struct {
	balances map[string]domain.Balance
	mu       sync.Mutex
}

func (m *mockBalanceRepository) SaveBalance(balance domain.Balance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.balances == nil {
		m.balances = make(map[string]domain.Balance)
	}
	m.balances[balance.Currency] = balance
	return nil
}

func (m *mockBalanceRepository) GetBalance(currency string) (*domain.Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if bal, ok := m.balances[currency]; ok {
		return &bal, nil
	}
	return nil, nil
}

func TestUpdateBalanceToDB(t *testing.T) {
	log := logger.NewNopLogger()

	// Test with nil DB
	service := NewBalanceService(nil, nil, log)
	ctx := context.Background()

	// Set cache to avoid API call
	service.cache.mu.Lock()
	service.cache.data = []domain.Balance{
		{Currency: "BTC", Amount: 1.0, Available: 1.0},
		{Currency: "JPY", Amount: 100000, Available: 100000},
	}
	service.cache.timestamp = time.Now()
	service.cache.mu.Unlock()

	// Should not panic with nil DB
	service.UpdateBalanceToDB(ctx)

	// Test with mock DB
	mockDB := &mockBalanceRepository{}
	service2 := NewBalanceService(nil, mockDB, log)

	// Set cache to avoid API call
	service2.cache.mu.Lock()
	service2.cache.data = []domain.Balance{
		{Currency: "BTC", Amount: 2.0, Available: 2.0},
		{Currency: "ETH", Amount: 10.0, Available: 10.0},
	}
	service2.cache.timestamp = time.Now()
	service2.cache.mu.Unlock()

	// Should save to mock DB
	service2.UpdateBalanceToDB(ctx)

	// Verify data was saved
	mockDB.mu.Lock()
	defer mockDB.mu.Unlock()
	if len(mockDB.balances) != 2 {
		t.Errorf("Expected 2 balances saved, got %d", len(mockDB.balances))
	}
	if mockDB.balances["BTC"].Amount != 2.0 {
		t.Errorf("Expected BTC amount 2.0, got %f", mockDB.balances["BTC"].Amount)
	}
}
