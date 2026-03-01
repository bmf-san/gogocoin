package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// WorkerManager manages the lifecycle of multiple workers
// Handles registration, starting, stopping, and health monitoring
type WorkerManager struct {
	workers map[string]Worker
	logger  logger.LoggerInterface
	wg      sync.WaitGroup
	mu      sync.RWMutex

	// Track worker states
	running map[string]bool
	errors  map[string]error
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(logger logger.LoggerInterface) *WorkerManager {
	return &WorkerManager{
		workers: make(map[string]Worker),
		logger:  logger,
		running: make(map[string]bool),
		errors:  make(map[string]error),
	}
}

// Register adds a worker to the manager
func (m *WorkerManager) Register(name string, worker Worker) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workers[name]; exists {
		return fmt.Errorf("worker %s already registered", name)
	}

	m.workers[name] = worker
	m.running[name] = false
	return nil
}

// StartAll starts all registered workers
func (m *WorkerManager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, worker := range m.workers {
		if m.running[name] {
			continue
		}

		m.running[name] = true
		m.wg.Add(1)

		go func(n string, w Worker) {
			defer m.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					m.logger.System().Error(fmt.Sprintf("%s panicked", n), "panic", r)
					m.mu.Lock()
					m.errors[n] = fmt.Errorf("panic: %v", r)
					m.running[n] = false
					m.mu.Unlock()
				}
			}()

			m.logger.System().Info(fmt.Sprintf("Starting worker: %s", n))
			if err := w.Run(ctx); err != nil {
				m.mu.Lock()
				m.errors[n] = err
				m.running[n] = false
				m.mu.Unlock()
				m.logger.System().WithError(err).Error(fmt.Sprintf("Worker %s stopped with error", n))
			} else {
				m.logger.System().Info(fmt.Sprintf("Worker %s stopped gracefully", n))
				m.mu.Lock()
				m.running[n] = false
				m.mu.Unlock()
			}
		}(name, worker)
	}

	return nil
}

// StopAll stops all running workers
func (m *WorkerManager) StopAll() error {
	m.mu.Lock()

	// Stop all stoppable workers
	for name, worker := range m.workers {
		if !m.running[name] {
			continue
		}

		if stoppable, ok := worker.(Stoppable); ok {
			if err := stoppable.Stop(); err != nil {
				m.logger.System().WithError(err).Error(fmt.Sprintf("Failed to stop worker %s", name))
			}
		}
	}
	m.mu.Unlock()

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.System().Info("All workers stopped successfully")
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for workers to stop")
	}
}

// HealthCheck returns health status of all workers
func (m *WorkerManager) HealthCheck() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]HealthStatus)

	for name, worker := range m.workers {
		if checker, ok := worker.(HealthChecker); ok {
			health[name] = checker.Health()
		} else {
			// Default health based on running state
			health[name] = HealthStatus{
				Healthy:   m.running[name],
				Message:   getDefaultHealthMessage(m.running[name], m.errors[name]),
				LastCheck: time.Now(),
			}
		}
	}

	return health
}

// GetStatus returns the running status of all workers
func (m *WorkerManager) GetStatus() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]bool)
	for name := range m.workers {
		status[name] = m.running[name]
	}
	return status
}

// GetErrors returns errors from all workers
func (m *WorkerManager) GetErrors() map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make(map[string]error)
	for name, err := range m.errors {
		if err != nil {
			errors[name] = err
		}
	}
	return errors
}

func getDefaultHealthMessage(running bool, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if running {
		return "running"
	}
	return "stopped"
}
