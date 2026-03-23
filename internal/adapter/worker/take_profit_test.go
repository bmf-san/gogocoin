package worker

import (
	"testing"

	"github.com/bmf-san/gogocoin/internal/domain"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

func TestCheckTakeProfit_NoPositionReader(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	// No position reader set — must return nil
	if got := w.checkTakeProfit("ETH_JPY", 340000.0); got != nil {
		t.Fatalf("expected nil when no position reader, got %+v", got)
	}
}

func TestCheckTakeProfit_NoOpenPositions(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{positions: []domain.Position{}})

	if got := w.checkTakeProfit("ETH_JPY", 340000.0); got != nil {
		t.Fatalf("expected nil for empty positions, got %+v", got)
	}
}

func TestCheckTakeProfit_NotTriggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"},
		},
	})

	// 2% above 330000 = 336600; current price 335000 is below TP → no trigger
	if got := w.checkTakeProfit("ETH_JPY", 335000.0); got != nil {
		t.Fatalf("take profit should not trigger at 335000 (tp=336600), got %+v", got)
	}
}

func TestCheckTakeProfit_Triggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"},
		},
	})

	// 2% above 330000 = 336600; current price 340000 is above TP → trigger
	got := w.checkTakeProfit("ETH_JPY", 340000.0)
	if got == nil {
		t.Fatal("expected SELL signal when take profit triggers, got nil")
	}
	if got.Action != strategy.SignalSell {
		t.Fatalf("expected SignalSell, got %v", got.Action)
	}
	if got.Symbol != "ETH_JPY" {
		t.Fatalf("expected symbol ETH_JPY, got %v", got.Symbol)
	}
	if got.Metadata["reason"] != "take_profit" {
		t.Fatalf("expected reason=take_profit, got %v", got.Metadata["reason"])
	}
}

func TestCheckTakeProfit_ExactlyAtTakePrice(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"},
		},
	})

	// exactly at take price (330000 * 1.02 = 336600) must trigger
	got := w.checkTakeProfit("ETH_JPY", 336600.0)
	if got == nil {
		t.Fatal("expected SELL signal at exact take price, got nil")
	}
}

func TestCheckTakeProfit_ZeroTakeProfitPct(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 0.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"},
		},
	})

	// take_profit_pct=0 means disabled
	if got := w.checkTakeProfit("ETH_JPY", 999999.0); got != nil {
		t.Fatalf("take profit should be disabled when pct=0, got %+v", got)
	}
}

func TestCheckTakeProfit_PartialPosition_Triggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330792.0, Status: "PARTIAL"},
		},
	})

	// 2% above 330792 = 337407.84; current price 340362 is above TP → trigger
	got := w.checkTakeProfit("ETH_JPY", 340362.0)
	if got == nil {
		t.Fatal("expected SELL signal for PARTIAL position above take price, got nil")
	}
	if got.Action != strategy.SignalSell {
		t.Fatalf("expected SignalSell, got %v", got.Action)
	}
	if got.Metadata["reason"] != "take_profit" {
		t.Fatalf("expected reason=take_profit, got %v", got.Metadata["reason"])
	}
}

func TestCheckTakeProfit_MultiplePositions_FirstTriggered(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{"take_profit_pct": 2.0})
	w.SetPositionReader(&mockPositionReader{
		positions: []domain.Position{
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 330000.0, Status: "OPEN"}, // tp 336600
			{Symbol: "ETH_JPY", Side: "BUY", EntryPrice: 335000.0, Status: "OPEN"}, // tp 341700
		},
	})

	// 340000 > 336600 (first) but < 341700 (second) → triggers on first position
	got := w.checkTakeProfit("ETH_JPY", 340000.0)
	if got == nil {
		t.Fatal("expected SELL signal, got nil")
	}
	entryPrice, _ := got.Metadata["entry_price"].(float64)
	if entryPrice != 330000.0 {
		t.Fatalf("expected entry_price=330000.0, got %v", entryPrice)
	}
}
