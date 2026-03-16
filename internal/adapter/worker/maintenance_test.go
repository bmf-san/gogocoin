package worker

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/logger"
)

// mockMaintenanceRepository implements domain.MaintenanceRepository for testing
type mockMaintenanceRepository struct {
	getDatabaseSizeCalled bool
	cleanupCalled         bool
	getTableStatsCalled   bool
	cleanupError          error
}

func (m *mockMaintenanceRepository) GetDatabaseSize() (int64, error) {
	m.getDatabaseSizeCalled = true
	return 1024 * 1024 * 10, nil // 10MB
}

func (m *mockMaintenanceRepository) CleanupOldData(retentionDays int) error {
	m.cleanupCalled = true
	return m.cleanupError
}

func (m *mockMaintenanceRepository) GetTableStats() (map[string]int, error) {
	m.getTableStatsCalled = true
	return map[string]int{
		"trades":     100,
		"positions":  50,
		"market_data": 1000,
	}, nil
}

func TestMaintenanceWorker_Creation(t *testing.T) {
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

	mockDB := &mockMaintenanceRepository{}
	worker := NewMaintenanceWorker(log, mockDB, 30, nil)

	if worker == nil {
		t.Fatal("Expected worker to be created")
	}
	if worker.retentionDays != 30 {
		t.Errorf("Expected retentionDays=30, got %d", worker.retentionDays)
	}
	if worker.checkInterval != 10*time.Minute {
		t.Errorf("Expected checkInterval=10m, got %v", worker.checkInterval)
	}
}

func TestMaintenanceWorker_RunAndCancel(t *testing.T) {
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

	mockDB := &mockMaintenanceRepository{}
	worker := NewMaintenanceWorker(log, mockDB, 30, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	// Wait for context to cancel or worker to finish
	select {
	case <-done:
		// Worker stopped successfully
	case <-time.After(3 * time.Second):
		t.Fatal("Worker did not stop within timeout")
	}
}

func TestMaintenanceWorker_CleanupExecution(t *testing.T) {
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

	mockDB := &mockMaintenanceRepository{}
	worker := NewMaintenanceWorker(log, mockDB, 7, nil)

	// Manually trigger cleanup (testing the internal method indirectly)
	// Since runDailyCleanup is private, we verify through database calls
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	<-done

	// Note: Cleanup only runs at midnight, so in this short test it won't be called
	// This test verifies the worker starts and stops cleanly
}
