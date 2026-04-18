package strategy

import (
	"context"
	"testing"
	"time"
)

// ── BaseStrategy metadata ────────────────────────────────────────────────────

func TestBaseStrategy_Metadata(t *testing.T) {
	bs := NewBaseStrategy("scalping", "desc", "1.0.0")
	if bs.Name() != "scalping" {
		t.Errorf("Name()=%q", bs.Name())
	}
	if bs.Description() != "desc" {
		t.Errorf("Description()=%q", bs.Description())
	}
	if bs.Version() != "1.0.0" {
		t.Errorf("Version()=%q", bs.Version())
	}
	if bs.IsRunning() {
		t.Error("IsRunning must be false initially")
	}
	if bs.GetConfig() == nil {
		t.Error("GetConfig returned nil map")
	}
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

func TestBaseStrategy_StartStop(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	ctx := context.Background()

	if err := bs.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !bs.IsRunning() || !bs.GetStatus().IsRunning {
		t.Error("after Start, IsRunning must be true")
	}
	if bs.GetStatus().StartTime.IsZero() {
		t.Error("StartTime must be set by Start")
	}

	if err := bs.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if bs.IsRunning() || bs.GetStatus().IsRunning {
		t.Error("after Stop, IsRunning must be false")
	}
}

func TestBaseStrategy_Reset_ClearsCounters(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	bs.UpdateSignalCount(SignalBuy)
	bs.UpdateMetrics(100, true)

	if err := bs.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if bs.GetStatus().TotalSignals != 0 {
		t.Errorf("TotalSignals after Reset=%d", bs.GetStatus().TotalSignals)
	}
	if bs.GetMetrics().TotalTrades != 0 {
		t.Errorf("TotalTrades after Reset=%d", bs.GetMetrics().TotalTrades)
	}
	if bs.GetStatus().SignalsByAction == nil {
		t.Error("SignalsByAction must be re-initialized (non-nil) after Reset")
	}
}

// ── Default overrides ────────────────────────────────────────────────────────

func TestBaseStrategy_DefaultOverrides(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	if bs.GetStopLossPrice(1000) != 0 {
		t.Error("GetStopLossPrice default must be 0")
	}
	if bs.GetTakeProfitPrice(1000) != 0 {
		t.Error("GetTakeProfitPrice default must be 0")
	}
	if bs.GetBaseNotional("BTC_JPY") != 0 {
		t.Error("GetBaseNotional default must be 0")
	}
	cfg := bs.GetAutoScaleConfig()
	if cfg.Enabled {
		t.Error("AutoScale default must be disabled")
	}
	if cfg.BalancePct != 80.0 {
		t.Errorf("AutoScale default BalancePct=%v, want 80", cfg.BalancePct)
	}
	// Coverage for no-op default methods
	bs.RecordTrade()
	bs.InitializeDailyTradeCount(5)
}

// ── UpdateSignalCount ────────────────────────────────────────────────────────

func TestBaseStrategy_UpdateSignalCount(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	bs.UpdateSignalCount(SignalBuy)
	bs.UpdateSignalCount(SignalBuy)
	bs.UpdateSignalCount(SignalSell)

	st := bs.GetStatus()
	if st.TotalSignals != 3 {
		t.Errorf("TotalSignals=%d, want 3", st.TotalSignals)
	}
	if st.SignalsByAction[SignalBuy] != 2 {
		t.Errorf("BUY count=%d, want 2", st.SignalsByAction[SignalBuy])
	}
	if st.SignalsByAction[SignalSell] != 1 {
		t.Errorf("SELL count=%d, want 1", st.SignalsByAction[SignalSell])
	}
	if st.LastSignalTime.IsZero() {
		t.Error("LastSignalTime must be set")
	}
}

// ── UpdateMetrics ────────────────────────────────────────────────────────────

func TestBaseStrategy_UpdateMetrics_WinLoss(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	bs.UpdateMetrics(100, true)
	bs.UpdateMetrics(50, true)
	bs.UpdateMetrics(-30, false)
	bs.UpdateMetrics(-70, false)

	m := bs.GetMetrics()
	if m.TotalTrades != 4 {
		t.Errorf("TotalTrades=%d", m.TotalTrades)
	}
	if m.WinningTrades != 2 || m.LosingTrades != 2 {
		t.Errorf("wins=%d losses=%d", m.WinningTrades, m.LosingTrades)
	}
	if m.WinRate != 50.0 {
		t.Errorf("WinRate=%v", m.WinRate)
	}
	if m.TotalProfit != 50 { // 100+50-30-70
		t.Errorf("TotalProfit=%v", m.TotalProfit)
	}
	if m.AverageProfit != 12.5 {
		t.Errorf("AverageProfit=%v", m.AverageProfit)
	}
	if m.MaxProfit != 100 {
		t.Errorf("MaxProfit=%v", m.MaxProfit)
	}
	if m.MaxLoss != -70 {
		t.Errorf("MaxLoss=%v", m.MaxLoss)
	}
	// ProfitFactor = MaxProfit / -MaxLoss = 100/70 ≈ 1.428
	if m.ProfitFactor < 1.4 || m.ProfitFactor > 1.45 {
		t.Errorf("ProfitFactor=%v", m.ProfitFactor)
	}
}

// ── ValidateSignal ───────────────────────────────────────────────────────────

func TestBaseStrategy_ValidateSignal_Valid(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	sig := &Signal{Symbol: "BTC_JPY", Action: SignalBuy, Strength: 0.5, Price: 1000, Quantity: 0.01}
	if err := bs.ValidateSignal(sig); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestBaseStrategy_ValidateSignal_Errors(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	cases := []struct {
		name string
		sig  *Signal
	}{
		{"nil", nil},
		{"empty_symbol", &Signal{Action: SignalBuy, Strength: 0.5, Price: 1000}},
		{"bad_action", &Signal{Symbol: "X", Action: "NOPE", Strength: 0.5, Price: 1000}},
		{"strength_low", &Signal{Symbol: "X", Action: SignalBuy, Strength: -0.1, Price: 1000}},
		{"strength_high", &Signal{Symbol: "X", Action: SignalBuy, Strength: 1.1, Price: 1000}},
		{"zero_price", &Signal{Symbol: "X", Action: SignalBuy, Strength: 0.5, Price: 0}},
		{"negative_qty", &Signal{Symbol: "X", Action: SignalBuy, Strength: 0.5, Price: 1, Quantity: -1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := bs.ValidateSignal(c.sig); err == nil {
				t.Error("expected error")
			}
		})
	}
}

// ── CreateSignal ─────────────────────────────────────────────────────────────

func TestBaseStrategy_CreateSignal(t *testing.T) {
	bs := NewBaseStrategy("x", "x", "x")
	before := time.Now().Add(-time.Second)
	sig := bs.CreateSignal("BTC_JPY", SignalBuy, 0.8, 1234, 0.01, map[string]interface{}{"k": "v"})
	if sig.Symbol != "BTC_JPY" || sig.Action != SignalBuy {
		t.Errorf("symbol/action mismatch: %+v", sig)
	}
	if sig.Timestamp.Before(before) {
		t.Error("Timestamp must be ≥ now-1s")
	}
	if sig.Metadata["k"] != "v" {
		t.Errorf("Metadata not set")
	}
}
