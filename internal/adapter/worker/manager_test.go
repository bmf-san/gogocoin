package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/logger"
)

// mockWorker is a mock implementation of Worker for testing
type mockWorker struct {
	name      string
	runFunc   func(ctx context.Context) error
	runCalled bool
}

func (m *mockWorker) Run(ctx context.Context) error {
	m.runCalled = true
	if m.runFunc != nil {
		return m.runFunc(ctx)
	}
	<-ctx.Done()
	return nil
}

func (m *mockWorker) Name() string {
	return m.name
}

func TestWorkerManager_Register(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	wm := NewWorkerManager(log)

	worker1 := &mockWorker{name: "worker1"}
	worker2 := &mockWorker{name: "worker2"}

	// Test successful registration
	if err := wm.Register("worker1", worker1); err != nil {
		t.Errorf("Failed to register worker1: %v", err)
	}

	// Test duplicate registration
	if err := wm.Register("worker1", worker2); err == nil {
		t.Error("Expected error when registering duplicate worker")
	}

	// Test registering another worker
	if err := wm.Register("worker2", worker2); err != nil {
		t.Errorf("Failed to register worker2: %v", err)
	}
}

func TestWorkerManager_StartAll(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	wm := NewWorkerManager(log)

	worker1 := &mockWorker{name: "worker1"}
	worker2 := &mockWorker{name: "worker2"}

	wm.Register("worker1", worker1)
	wm.Register("worker2", worker2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := wm.StartAll(ctx); err != nil {
		t.Errorf("Failed to start workers: %v", err)
	}

	// Give workers time to start
	time.Sleep(100 * time.Millisecond)

	// Check that workers are running
	status := wm.GetStatus()
	if !status["worker1"] {
		t.Error("worker1 should be running")
	}
	if !status["worker2"] {
		t.Error("worker2 should be running")
	}

	// Cancel context and wait for workers to stop
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestWorkerManager_StopAll(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	wm := NewWorkerManager(log)

	worker1 := &mockWorker{name: "worker1"}

	wm.Register("worker1", worker1)

	ctx, cancel := context.WithCancel(context.Background())

	wm.StartAll(ctx)
	time.Sleep(100 * time.Millisecond)

	// Cancel context to signal workers to stop
	cancel()

	// Stop all workers
	if err := wm.StopAll(); err != nil {
		t.Errorf("Failed to stop workers: %v", err)
	}

	// Check that workers are stopped
	status := wm.GetStatus()
	if status["worker1"] {
		t.Error("worker1 should be stopped")
	}
}

func TestWorkerManager_ErrorHandling(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	wm := NewWorkerManager(log)

	expectedErr := errors.New("worker error")
	errorWorker := &mockWorker{
		name: "error_worker",
		runFunc: func(ctx context.Context) error {
			return expectedErr
		},
	}

	wm.Register("error_worker", errorWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm.StartAll(ctx)
	time.Sleep(100 * time.Millisecond)

	// Check errors
	errors := wm.GetErrors()
	if len(errors) == 0 {
		t.Error("Expected error from worker")
	}
}

func TestWorkerManager_HealthCheck(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	wm := NewWorkerManager(log)

	worker1 := &mockWorker{name: "worker1"}
	wm.Register("worker1", worker1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm.StartAll(ctx)
	time.Sleep(100 * time.Millisecond)

	health := wm.HealthCheck()
	if len(health) == 0 {
		t.Error("Expected health status for workers")
	}

	if health["worker1"].Message != "running" {
		t.Errorf("Expected worker1 to be healthy, got: %s", health["worker1"].Message)
	}
}
