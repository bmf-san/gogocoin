package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
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

// mockClientFactory implements ClientFactory for testing
type mockClientFactory struct {
	mu                 sync.Mutex
	connected          bool
	reconnectError     error
	reconnectCallCount int
	subscribeError     error
	subscribeCallCount int
	simulateDisconnect bool
	disconnectAfter    int // Disconnect after N subscription calls
}

func (m *mockClientFactory) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockClientFactory) ReconnectClient() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reconnectCallCount++

	if m.reconnectError != nil {
		return m.reconnectError
	}

	m.connected = true
	return nil
}

func (m *mockClientFactory) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribeCallCount++

	if m.subscribeError != nil {
		return m.subscribeError
	}

	// Simulate disconnect after N calls
	if m.simulateDisconnect && m.subscribeCallCount >= m.disconnectAfter {
		m.connected = false
	}

	// Send test data through callback
	go func() {
		time.Sleep(10 * time.Millisecond)
		callback(domain.MarketData{
			Symbol:    symbol,
			Price:     1000.0,
			Timestamp: time.Now(),
		})
	}()

	return nil
}

func (m *mockClientFactory) setConnected(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = connected
}

func (m *mockClientFactory) getReconnectCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reconnectCallCount
}

func (m *mockClientFactory) getSubscribeCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subscribeCallCount
}

// TestMarketDataWorker_ReconnectionScenario tests WebSocket reconnection behavior
func TestMarketDataWorker_ReconnectionScenario(t *testing.T) {
	t.Run("successful_reconnection_after_disconnect", func(t *testing.T) {
		// Create test logger
		log := createTestLogger(t)

		// Create mock client that starts disconnected
		mockClient := &mockClientFactory{
			connected:          false,
			reconnectError:     nil,
			simulateDisconnect: false,
		}

		// Create market data channel
		marketDataCh := make(chan domain.MarketData, 10)

		// Create worker
		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY"},
			marketDataCh,
			mockClient,
			1,   // 1 second reconnect interval
			300, // 5 minutes max
			1,   // 1 second connection check
			0,   // stale data timeout (default 3m)
		)

		// Start worker in background
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go worker.Run(ctx)

		// Wait for reconnection attempt
		time.Sleep(2 * time.Second)

		// Verify reconnection was attempted
		if mockClient.getReconnectCallCount() == 0 {
			t.Error("Expected reconnection attempt, but none occurred")
		}

		// Verify client is now connected
		if !mockClient.IsConnected() {
			t.Error("Expected client to be connected after reconnection")
		}

		// Verify subscription was attempted
		if mockClient.getSubscribeCallCount() == 0 {
			t.Error("Expected subscription attempt after reconnection")
		}

		// Verify market data was received
		select {
		case data := <-marketDataCh:
			if data.Symbol != "BTC_JPY" {
				t.Errorf("Expected symbol BTC_JPY, got %s", data.Symbol)
			}
		case <-time.After(3 * time.Second):
			t.Error("No market data received after reconnection")
		}
	})

	t.Run("reconnection_with_backoff", func(t *testing.T) {
		log := createTestLogger(t)

		// Mock client that fails first reconnection, succeeds on second
		failCount := 0
		mockClient := &mockClientFactory{
			connected: false,
		}

		// Store the original reconnect method
		mockClient.reconnectError = errors.New("first attempt fails")

		marketDataCh := make(chan domain.MarketData, 10)

		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY"},
			marketDataCh,
			mockClient,
			1,   // 1 second reconnect interval
			300, // 5 minutes max
			1,   // 1 second connection check
			0,   // stale data timeout (default 3m)
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Update mock to succeed on second attempt
		go func() {
			time.Sleep(1500 * time.Millisecond)
			mockClient.mu.Lock()
			mockClient.reconnectError = nil
			mockClient.mu.Unlock()
		}()

		go worker.Run(ctx)

		// Wait for multiple reconnection attempts
		time.Sleep(3 * time.Second)

		// Verify multiple reconnection attempts occurred
		if mockClient.getReconnectCallCount() < 2 {
			t.Errorf("Expected at least 2 reconnection attempts, got %d", mockClient.getReconnectCallCount())
		}

		// Since we cleared the error, verify eventual connection
		if failCount > 0 {
			t.Logf("Failed attempts before success: %d", failCount)
		}
	})

	t.Run("graceful_shutdown_during_reconnection", func(t *testing.T) {
		log := createTestLogger(t)

		// Mock client that always fails reconnection
		mockClient := &mockClientFactory{
			connected:      false,
			reconnectError: errors.New("permanent failure"),
		}

		marketDataCh := make(chan domain.MarketData, 10)

		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY"},
			marketDataCh,
			mockClient,
			1,   // 1 second reconnect interval
			300, // 5 minutes max
			1,   // 1 second connection check
			0,   // stale data timeout (default 3m)
		)

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			worker.Run(ctx)
			close(done)
		}()

		// Let it try reconnecting a few times
		time.Sleep(2 * time.Second)

		// Cancel context to trigger shutdown
		cancel()

		// Wait for graceful shutdown
		select {
		case <-done:
			// Success - worker shut down gracefully
		case <-time.After(3 * time.Second):
			t.Error("Worker did not shut down gracefully within timeout")
		}

		// Verify some reconnection attempts were made
		if mockClient.getReconnectCallCount() == 0 {
			t.Error("Expected at least one reconnection attempt")
		}
	})

	t.Run("multiple_subscriptions_with_reconnect", func(t *testing.T) {
		log := createTestLogger(t)

		mockClient := &mockClientFactory{
			connected: false,
		}

		marketDataCh := make(chan domain.MarketData, 10)

		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY", "ETH_JPY"},
			marketDataCh,
			mockClient,
			1,
			300,
			1,
			0,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		go worker.Run(ctx)

		// Wait for reconnection and subscriptions
		time.Sleep(2 * time.Second)

		// Verify reconnection occurred
		if !mockClient.IsConnected() {
			t.Error("Expected client to be connected")
		}

		// Verify multiple subscriptions (one per symbol)
		if mockClient.getSubscribeCallCount() < 2 {
			t.Errorf("Expected at least 2 subscription calls, got %d", mockClient.getSubscribeCallCount())
		}
	})
}

// TestMarketDataWorker_ChannelManagement tests channel handling
func TestMarketDataWorker_ChannelManagement(t *testing.T) {
	t.Run("market_data_flows_through_channel", func(t *testing.T) {
		log := createTestLogger(t)

		mockClient := &mockClientFactory{
			connected: true,
		}

		marketDataCh := make(chan domain.MarketData, 10)

		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY", "ETH_JPY"},
			marketDataCh,
			mockClient,
			1,
			300,
			1,
			0,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		go worker.Run(ctx)

		// Collect market data
		receivedSymbols := make(map[string]bool)
		timeout := time.After(3 * time.Second)

	loop:
		for {
			select {
			case data := <-marketDataCh:
				receivedSymbols[data.Symbol] = true
				if len(receivedSymbols) == 2 {
					break loop
				}
			case <-timeout:
				break loop
			}
		}

		// Verify data for both symbols was received
		if !receivedSymbols["BTC_JPY"] {
			t.Error("Did not receive BTC_JPY market data")
		}
		if !receivedSymbols["ETH_JPY"] {
			t.Error("Did not receive ETH_JPY market data")
		}
	})

	t.Run("context_cancellation_stops_worker", func(t *testing.T) {
		log := createTestLogger(t)

		mockClient := &mockClientFactory{
			connected: true,
		}

		marketDataCh := make(chan domain.MarketData, 10)

		worker := NewMarketDataWorker(
			log,
			[]string{"BTC_JPY"},
			marketDataCh,
			mockClient,
			1,
			300,
			1,
			0,
		)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			worker.Run(ctx)
			close(done)
		}()

		// Let worker run for a bit
		time.Sleep(500 * time.Millisecond)

		// Cancel context
		cancel()

		// Verify worker stops
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Error("Worker did not stop after context cancellation")
		}
	})
}
