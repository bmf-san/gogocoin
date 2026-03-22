package worker

import (
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// mockPositionReader implements PositionReader for testing.
type mockPositionReader struct {
	positions []domain.Position
	err       error
}

func (m *mockPositionReader) GetOpenPositions(symbol, side string) ([]domain.Position, error) {
	return m.positions, m.err
}

// mockStrategyWithConfig is a mockStrategy that returns a configurable GetConfig map.
type mockStrategyWithConfig struct {
	mockStrategy
	cfg map[string]any
}

func (m *mockStrategyWithConfig) GetConfig() map[string]any {
	return m.cfg
}

func (m *mockStrategyWithConfig) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	return &strategy.Signal{
		Symbol: "XRP_JPY",
		Action: strategy.SignalHold,
	}, nil
}

func newTestStrategyWorker(t *testing.T, cfg map[string]any) *StrategyWorker {
	t.Helper()
	log, err := logger.New(&logger.Config{
		Level:    "error",
		Format:   "json",
		Output:   "file",
		FilePath: "/dev/null",
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	strat := &mockStrategyWithConfig{cfg: cfg}
	marketDataCh := make(chan domain.MarketData)
	signalCh := make(chan *strategy.Signal, 10)
	return NewStrategyWorker(log, strat, marketDataCh, signalCh)
}

func TestCheckStopLoss_NoPositionReader(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	// No position reader set — must return nil
	if got := w.checkStopLoss("XRP_JPY", 220.0); got != nil {
		t.Fatalf("expected nil when no position reader, got %+v", got)
	}
}

func TestCheckStopLoss_NoOpenPositions(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	w.SetPositionReader(&mockPositionReader{positions: []domain.Position{}})

	if got := w.checkStopLoss("XRP_JPY", 220.0); got != nil {
		t.Fatalf("expected nil for empty positions, got %+v", got)
	}
}

func TestCheckStopLoss_StopLossNotTriggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 230.0, Status: "OPEN"},
		},
	})

	// 1% below 230.0 = 227.70; current price 228.0 is above stop → no trigger
	if got := w.checkStopLoss("XRP_JPY", 228.0); got != nil {
		t.Fatalf("stop loss should not trigger at 228.0 (stop=227.7), got %+v", got)
	}
}

func TestCheckStopLoss_StopLossTriggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 230.0, Status: "OPEN"},
		},
	})

	// 1% below 230.0 = 227.70; current price 227.5 is below stop → trigger
	got := w.checkStopLoss("XRP_JPY", 227.5)
	if got == nil {
		t.Fatal("expected SELL signal when stop loss triggers, got nil")
	}
	if got.Action != strategy.SignalSell {
		t.Fatalf("expected SignalSell, got %v", got.Action)
	}
	if got.Symbol != "XRP_JPY" {
		t.Fatalf("expected symbol XRP_JPY, got %v", got.Symbol)
	}
	if got.Metadata["reason"] != "stop_loss" {
		t.Fatalf("expected reason=stop_loss, got %v", got.Metadata["reason"])
	}
}

func TestCheckStopLoss_ExactlyAtStopPrice(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 230.0, Status: "OPEN"},
		},
	})

	// exactly at stop price (230 * 0.99 = 227.7) must trigger
	got := w.checkStopLoss("XRP_JPY", 227.7)
	if got == nil {
		t.Fatal("expected SELL signal at exact stop price, got nil")
	}
}

func TestCheckStopLoss_ZeroStopLossPct(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 0.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 230.0, Status: "OPEN"},
		},
	})

	// stop_loss_pct=0 means disabled — should not trigger regardless of price
	if got := w.checkStopLoss("XRP_JPY", 100.0); got != nil {
		t.Fatalf("stop loss should be disabled when pct=0, got %+v", got)
	}
}

func TestCheckStopLoss_MultiplePositions_OnlyOneBreached(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 1.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 229.0, Status: "OPEN"},  // stop 226.71
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 224.75, Status: "OPEN"}, // stop 222.50
		},
	})

	// 226.5 is below 229 stop (226.71) → should trigger on first position
	got := w.checkStopLoss("XRP_JPY", 226.5)
	if got == nil {
		t.Fatal("expected SELL signal, got nil")
	}
	entryPrice, _ := got.Metadata["entry_price"].(float64)
	if entryPrice != 229.0 {
		t.Fatalf("expected entry_price=229.0, got %v", entryPrice)
	}
}

func TestGetStopLossPct_NotConfigured(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{})
	if pct := w.getStopLossPct(); pct != 0 {
		t.Fatalf("expected 0 when stop_loss_pct not in config, got %v", pct)
	}
}

func TestGetStopLossPct_Configured(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"stop_loss_pct": 2.5})
	if pct := w.getStopLossPct(); pct != 2.5 {
		t.Fatalf("expected 2.5, got %v", pct)
	}
}

// TestExecuteStrategy_StopLossOverride verifies that a HOLD signal from the
// strategy is overridden by a stop-loss SELL when the position is underwater.
func TestExecuteStrategy_StopLossOverride(t *testing.T) {
	log, _ := logger.New(&logger.Config{
		Level: "error", Format: "json", Output: "file", FilePath: "/dev/null",
	})

	strat := &mockStrategyWithConfig{
		cfg: map[string]any{"stop_loss_pct": 1.0},
	}
	marketDataCh := make(chan domain.MarketData)
	signalCh := make(chan *strategy.Signal, 10)
	w := NewStrategyWorker(log, strat, marketDataCh, signalCh)
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", EntryPrice: 230.0, Status: "OPEN"},
		},
	})

	md := &domain.MarketData{
		Symbol:    "XRP_JPY",
		Price:     227.0, // below 230 * 0.99 = 227.7
		Timestamp: time.Now(),
	}

	w.executeStrategy(t.Context(), md, []strategy.MarketData{{Symbol: "XRP_JPY", Price: 227.0}})

	select {
	case sig := <-signalCh:
		if sig.Action != strategy.SignalSell {
			t.Fatalf("expected SELL (stop loss), got %v", sig.Action)
		}
		if sig.Metadata["reason"] != "stop_loss" {
			t.Fatalf("expected reason=stop_loss, got %v", sig.Metadata["reason"])
		}
	default:
		t.Fatal("no signal sent to channel — stop loss override failed")
	}
}
