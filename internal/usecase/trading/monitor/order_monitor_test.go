package monitor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
)

// ── test doubles ──────────────────────────────────────────────────────────────

// mockOrderGetter returns a configurable sequence of responses.
type mockOrderGetter struct {
	calls   atomic.Int32
	results [][]*domain.OrderResult // one entry per call (last entry repeated)
	errs    []error
}

func (m *mockOrderGetter) GetOrdersBySymbol(_ context.Context, _ string) ([]*domain.OrderResult, error) {
	idx := int(m.calls.Add(1)) - 1
	if idx >= len(m.results) {
		idx = len(m.results) - 1
	}
	errIdx := idx
	if errIdx >= len(m.errs) {
		errIdx = len(m.errs) - 1
	}
	return m.results[idx], m.errs[errIdx]
}

// singleResult is a convenience helper that returns the same response every call.
func singleResult(status string) *mockOrderGetter {
	result := []*domain.OrderResult{
		{OrderID: "ORD-1", Status: status, FilledSize: 0.001, AveragePrice: 5_000_000},
	}
	return &mockOrderGetter{
		results: [][]*domain.OrderResult{result},
		errs:    []error{nil},
	}
}

type mockBalanceUpdater struct {
	invalidated atomic.Bool
	updated     atomic.Bool
}

func (m *mockBalanceUpdater) InvalidateBalanceCache()             { m.invalidated.Store(true) }
func (m *mockBalanceUpdater) UpdateBalanceToDB(_ context.Context) { m.updated.Store(true) }

type mockPnLCalculator struct {
	called atomic.Bool
	err    error
}

func (m *mockPnLCalculator) CalculateAndSave(_ *domain.OrderResult) (float64, error) {
	m.called.Store(true)
	return 0, m.err
}

// newFastMonitor creates an OrderMonitor with very short intervals suitable for unit tests.
func newFastMonitor(
	getter OrderGetter,
	balance BalanceUpdater,
	pnl PnLCalculator,
) *OrderMonitor {
	om := NewOrderMonitor(nil, getter, balance, pnl)
	om.monitorInterval = 10 * time.Millisecond
	om.monitorTimeout = 500 * time.Millisecond
	return om
}

// ── tests ──────────────────────────────────────────────────────────────────────

// TestOrderMonitor_CompletedOrder verifies that a COMPLETED order triggers the
// completion callback and updates balances.
func TestOrderMonitor_CompletedOrder(t *testing.T) {
	getter := singleResult("COMPLETED")
	balance := &mockBalanceUpdater{}
	pnl := &mockPnLCalculator{}

	om := newFastMonitor(getter, balance, pnl)

	callbackCalled := false
	var callbackResult *domain.OrderResult
	om.SetOnOrderCompleted(func(r *domain.OrderResult) {
		callbackCalled = true
		callbackResult = r
	})

	result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
	om.MonitorExecution(context.Background(), result)

	if !callbackCalled {
		t.Error("expected completion callback to be called for COMPLETED order")
	}
	if callbackResult == nil || callbackResult.Status != "COMPLETED" {
		t.Errorf("expected result status COMPLETED, got %v", callbackResult)
	}
	if !pnl.called.Load() {
		t.Error("expected PnLCalculator.CalculateAndSave to be called")
	}
	if !balance.invalidated.Load() {
		t.Error("expected balance cache to be invalidated")
	}
	if !balance.updated.Load() {
		t.Error("expected balance to be updated to DB")
	}
}

// TestOrderMonitor_CanceledOrder verifies that a CANCELED order is saved to DB
// but does NOT invoke the completion callback.
func TestOrderMonitor_CanceledOrder(t *testing.T) {
	for _, status := range []string{"CANCELED", "EXPIRED", "REJECTED"} {
		t.Run(status, func(t *testing.T) {
			getter := singleResult(status)
			balance := &mockBalanceUpdater{}
			pnl := &mockPnLCalculator{}

			om := newFastMonitor(getter, balance, pnl)

			callbackCalled := false
			om.SetOnOrderCompleted(func(_ *domain.OrderResult) { callbackCalled = true })

			result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
			om.MonitorExecution(context.Background(), result)

			if callbackCalled {
				t.Errorf("status %s: completion callback must NOT be called", status)
			}
			if !pnl.called.Load() {
				t.Errorf("status %s: PnLCalculator.CalculateAndSave should be called", status)
			}
			// Balance must NOT be updated for terminal non-COMPLETED orders
			if balance.updated.Load() {
				t.Errorf("status %s: balance update must NOT happen", status)
			}
		})
	}
}

// TestOrderMonitor_AuthError verifies that an authentication error causes the
// order to be saved with UNKNOWN_AUTH_ERROR status and stops monitoring without
// invoking the completion callback.
func TestOrderMonitor_AuthError(t *testing.T) {
	authErr := &bitflyer.APIError{StatusCode: 401, Code: "UNAUTHORIZED", Message: "auth failed"}
	getter := &mockOrderGetter{
		results: [][]*domain.OrderResult{nil},
		errs:    []error{authErr},
	}
	balance := &mockBalanceUpdater{}
	pnl := &mockPnLCalculator{}

	om := newFastMonitor(getter, balance, pnl)

	callbackCalled := false
	om.SetOnOrderCompleted(func(_ *domain.OrderResult) { callbackCalled = true })

	result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
	om.MonitorExecution(context.Background(), result)

	if callbackCalled {
		t.Error("completion callback must NOT be called after auth error")
	}
	if result.Status != "UNKNOWN_AUTH_ERROR" {
		t.Errorf("expected status UNKNOWN_AUTH_ERROR, got %s", result.Status)
	}
	if !pnl.called.Load() {
		t.Error("PnLCalculator.CalculateAndSave should be called to persist the unknown-auth record")
	}
}

// TestOrderMonitor_IntermittentErrors verifies that transient GetOrders failures
// are retried and monitoring continues until success.
func TestOrderMonitor_IntermittentErrors(t *testing.T) {
	transientErr := errors.New("temporary network error")

	completedResult := []*domain.OrderResult{
		{OrderID: "ORD-1", Status: "COMPLETED", FilledSize: 0.001, AveragePrice: 5_000_000},
	}

	// First two calls return an error; third call returns COMPLETED.
	getter := &mockOrderGetter{
		results: [][]*domain.OrderResult{
			nil,             // call 1 – error
			nil,             // call 2 – error
			completedResult, // call 3+ – success
		},
		errs: []error{
			transientErr,
			transientErr,
			nil,
		},
	}
	pnl := &mockPnLCalculator{}
	balance := &mockBalanceUpdater{}

	om := newFastMonitor(getter, balance, pnl)

	callbackCalled := false
	om.SetOnOrderCompleted(func(_ *domain.OrderResult) { callbackCalled = true })

	result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
	om.MonitorExecution(context.Background(), result)

	if !callbackCalled {
		t.Error("expected completion callback after transient errors resolved")
	}
	if getter.calls.Load() < 3 {
		t.Errorf("expected at least 3 GetOrders calls, got %d", getter.calls.Load())
	}
}

// TestOrderMonitor_ContextCancellation verifies that the monitor stops promptly
// when the context is canceled.
func TestOrderMonitor_ContextCancellation(t *testing.T) {
	// GetOrders always returns "OPEN" (never terminates naturally)
	getter := &mockOrderGetter{
		results: [][]*domain.OrderResult{
			{{OrderID: "ORD-1", Status: "OPEN"}},
		},
		errs: []error{nil},
	}
	pnl := &mockPnLCalculator{}
	balance := &mockBalanceUpdater{}

	om := newFastMonitor(getter, balance, pnl)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
		om.MonitorExecution(ctx, result)
		close(done)
	}()

	// Cancel after a short delay
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Error("MonitorExecution did not return after context cancellation")
	}
}

// TestOrderMonitor_Timeout verifies that when the monitor times out before the
// order completes, saveFinalOrderState is invoked: if the order is found in the
// final check, PnL is calculated and saved.
func TestOrderMonitor_Timeout(t *testing.T) {
	// GetOrders always reports "OPEN" so the monitor never terminates naturally.
	finalResult := []*domain.OrderResult{
		{OrderID: "ORD-1", Status: "OPEN", FilledSize: 0.001},
	}
	getter := &mockOrderGetter{
		results: [][]*domain.OrderResult{finalResult},
		errs:    []error{nil},
	}
	pnl := &mockPnLCalculator{}
	balance := &mockBalanceUpdater{}

	om := newFastMonitor(getter, balance, pnl)
	om.monitorTimeout = 30 * time.Millisecond // force quick timeout

	callbackCalled := false
	om.SetOnOrderCompleted(func(_ *domain.OrderResult) { callbackCalled = true })

	result := &domain.OrderResult{OrderID: "ORD-1", Status: "OPEN"}
	om.MonitorExecution(context.Background(), result)

	// Completion callback must NOT be called on timeout (order never COMPLETED)
	if callbackCalled {
		t.Error("completion callback must NOT be called on monitoring timeout")
	}
	// PnLCalculator should be called by saveFinalOrderState (order found in final check)
	if !pnl.called.Load() {
		t.Error("PnLCalculator.CalculateAndSave should be called during saveFinalOrderState on timeout")
	}
	// Balance should NOT be updated (order did not complete)
	if balance.updated.Load() {
		t.Error("balance update must NOT happen on monitoring timeout")
	}
}
