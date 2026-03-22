package pnl

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
)

// mockTradingRepo implements domain.TradingRepository for tests.
type mockTradingRepo struct {
	mu            sync.Mutex
	positions     []domain.Position
	savedTrades   []*domain.Trade
	updatedPos    []*domain.Position
	beginTxErr    error
	saveTradeErr  error
	savePosErr    error
	updatePosErr  error
	getOpenPosErr error
}

func (m *mockTradingRepo) BeginTx() (domain.Transaction, error) {
	if m.beginTxErr != nil {
		return nil, m.beginTxErr
	}
	return &mockTx{repo: m}, nil
}

func (m *mockTradingRepo) SaveTrade(t *domain.Trade) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveTradeErr != nil {
		return m.saveTradeErr
	}
	cp := *t
	m.savedTrades = append(m.savedTrades, &cp)
	return nil
}

func (m *mockTradingRepo) GetRecentTrades(limit int) ([]domain.Trade, error) {
	return nil, nil
}

func (m *mockTradingRepo) GetAllTrades() ([]domain.Trade, error) {
	return nil, nil
}

func (m *mockTradingRepo) SavePosition(p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.savePosErr != nil {
		return m.savePosErr
	}
	cp := *p
	m.positions = append(m.positions, cp)
	return nil
}

func (m *mockTradingRepo) GetOpenPositions(symbol, side string) ([]domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getOpenPosErr != nil {
		return nil, m.getOpenPosErr
	}
	var out []domain.Position
	for _, p := range m.positions {
		if p.Symbol == symbol && p.Side == side && p.Status == "OPEN" {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *mockTradingRepo) UpdatePosition(p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updatePosErr != nil {
		return m.updatePosErr
	}
	for i, pos := range m.positions {
		if pos.OrderID == p.OrderID {
			m.positions[i] = *p
			break
		}
	}
	cp := *p
	m.updatedPos = append(m.updatedPos, &cp)
	return nil
}

func (m *mockTradingRepo) CloseOpenPositions(symbol, side string) error { return nil }

func (m *mockTradingRepo) SaveBalance(b domain.Balance) error { return nil }

// mockTx delegates to repo so assertions see the same slices.
type mockTx struct{ repo *mockTradingRepo }

func (t *mockTx) Commit() error                           { return nil }
func (t *mockTx) Rollback() error                         { return nil }
func (t *mockTx) SaveTrade(tr *domain.Trade) error        { return t.repo.SaveTrade(tr) }
func (t *mockTx) SavePosition(p *domain.Position) error   { return t.repo.SavePosition(p) }
func (t *mockTx) UpdatePosition(p *domain.Position) error { return t.repo.UpdatePosition(p) }

// ── helpers ─────────────────────────────────────────────────────────────────

func newLogger(t *testing.T) logger.LoggerInterface {
	t.Helper()
	return logger.NewNopLogger()
}

func newResult(side string, size, avgPrice, fee float64, orderID string) *domain.OrderResult {
	now := time.Now()
	return &domain.OrderResult{
		OrderID:      orderID,
		Symbol:       "XRP_JPY",
		Side:         side,
		Type:         "MARKET",
		FilledSize:   size,
		AveragePrice: avgPrice,
		Fee:          fee,
		Status:       "COMPLETED",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func floatEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

// ── BUY tests ────────────────────────────────────────────────────────────────

func TestCalculateAndSave_BUY_CreatesTrade(t *testing.T) {
	repo := &mockTradingRepo{}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	pnl, err := calc.CalculateAndSave(newResult("BUY", 1.0, 200.0, 0.5, "ORDER-BUY-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// PnL for BUY = negative fee
	if pnl != -0.5 {
		t.Errorf("BUY pnl = %.4f, want -0.5", pnl)
	}
	if len(repo.savedTrades) != 1 {
		t.Fatalf("want 1 saved trade, got %d", len(repo.savedTrades))
	}
	tr := repo.savedTrades[0]
	if tr.Side != "BUY" {
		t.Errorf("trade.Side = %q, want BUY", tr.Side)
	}
	if tr.Status != "COMPLETED" {
		t.Errorf("trade.Status = %q, want COMPLETED", tr.Status)
	}
	if tr.StrategyName != "scalping" {
		t.Errorf("trade.StrategyName = %q, want scalping", tr.StrategyName)
	}
	if len(repo.positions) != 1 {
		t.Fatalf("want 1 position, got %d", len(repo.positions))
	}
	pos := repo.positions[0]
	if pos.Status != "OPEN" {
		t.Errorf("position.Status = %q, want OPEN", pos.Status)
	}
	if pos.EntryPrice != 200.0 {
		t.Errorf("position.EntryPrice = %.2f, want 200.0", pos.EntryPrice)
	}
}

// ── SELL tests ───────────────────────────────────────────────────────────────

func TestCalculateAndSave_SELL_PnLWithFIFO(t *testing.T) {
	repo := &mockTradingRepo{
		positions: []domain.Position{
			{
				Symbol: "XRP_JPY", Side: "BUY",
				Size: 2.0, RemainingSize: 2.0, UsedSize: 0,
				EntryPrice: 200.0, Status: "OPEN", OrderID: "ORDER-BUY-1",
			},
		},
	}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	// Sell 1.0 XRP at 220 JPY, fee 0.2
	pnl, err := calc.CalculateAndSave(newResult("SELL", 1.0, 220.0, 0.2, "ORDER-SELL-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected PnL: sell revenue - cost - fees = 220*1 - 200*1 - 0.2 = 19.8
	want := 19.8
	if !floatEq(pnl, want) {
		t.Errorf("SELL pnl = %.6f, want %.6f", pnl, want)
	}
	if len(repo.updatedPos) == 0 {
		t.Fatal("no positions were updated")
	}
	// Position should be partially used (1 used, 1 remaining)
	updated := repo.updatedPos[0]
	if updated.UsedSize != 1.0 {
		t.Errorf("position.UsedSize = %.2f, want 1.0", updated.UsedSize)
	}
	if updated.RemainingSize != 1.0 {
		t.Errorf("position.RemainingSize = %.2f, want 1.0", updated.RemainingSize)
	}
}

func TestCalculateAndSave_SELL_FullyClosePosition(t *testing.T) {
	repo := &mockTradingRepo{
		positions: []domain.Position{
			{
				Symbol: "XRP_JPY", Side: "BUY",
				Size: 1.0, RemainingSize: 1.0, UsedSize: 0,
				EntryPrice: 100.0, Status: "OPEN", OrderID: "ORDER-BUY-1",
			},
		},
	}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	pnl, err := calc.CalculateAndSave(newResult("SELL", 1.0, 150.0, 0.5, "ORDER-SELL-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// PnL = 150*1 - 100*1 - 0.5 = 49.5
	want := 49.5
	if !floatEq(pnl, want) {
		t.Errorf("pnl = %.6f, want %.6f", pnl, want)
	}
	if len(repo.updatedPos) == 0 {
		t.Fatal("no positions updated")
	}
	if repo.updatedPos[0].Status != "CLOSED" {
		t.Errorf("position status = %q, want CLOSED", repo.updatedPos[0].Status)
	}
}

func TestCalculateAndSave_SELL_NoOpenPositions(t *testing.T) {
	repo := &mockTradingRepo{} // empty positions
	calc := NewCalculator(repo, newLogger(t), "scalping")

	pnl, err := calc.CalculateAndSave(newResult("SELL", 1.0, 200.0, 0.5, "ORDER-SELL-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No cost to match → PnL = -fee
	want := -0.5
	if pnl != want {
		t.Errorf("pnl = %.4f, want %.4f", pnl, want)
	}
}

func TestCalculateAndSave_SELL_MultiplePositionsFIFO(t *testing.T) {
	// Two open positions — FIFO should match oldest first
	repo := &mockTradingRepo{
		positions: []domain.Position{
			{Symbol: "XRP_JPY", Side: "BUY", Size: 1.0, RemainingSize: 1.0, EntryPrice: 100.0, Status: "OPEN", OrderID: "ORDER-BUY-1"},
			{Symbol: "XRP_JPY", Side: "BUY", Size: 1.0, RemainingSize: 1.0, EntryPrice: 200.0, Status: "OPEN", OrderID: "ORDER-BUY-2"},
		},
	}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	// Sell 2.0 at 250 JPY, fee 0.0
	pnl, err := calc.CalculateAndSave(newResult("SELL", 2.0, 250.0, 0.0, "ORDER-SELL-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cost = 100*1 + 200*1 = 300, revenue = 250*2 = 500, pnl = 200
	want := 200.0
	if !floatEq(pnl, want) {
		t.Errorf("pnl = %.6f, want %.6f", pnl, want)
	}
	if len(repo.updatedPos) != 2 {
		t.Errorf("expected 2 positions updated, got %d", len(repo.updatedPos))
	}
}

// ── edge cases ───────────────────────────────────────────────────────────────

func TestCalculateAndSave_TransactionFallback_SaveStillSucceeds(t *testing.T) {
	repo := &mockTradingRepo{
		beginTxErr: errors.New("tx unavailable"),
	}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	_, err := calc.CalculateAndSave(newResult("BUY", 1.0, 200.0, 0.3, "ORDER-BUY-FALLBACK"))
	if err != nil {
		t.Fatalf("unexpected error on fallback path: %v", err)
	}
	if len(repo.savedTrades) != 1 {
		t.Errorf("expected 1 trade saved via fallback, got %d", len(repo.savedTrades))
	}
}

func TestCalculateAndSave_NilDB_ReturnsZero(t *testing.T) {
	calc := NewCalculator(nil, newLogger(t), "scalping")
	pnl, err := calc.CalculateAndSave(newResult("BUY", 1.0, 200.0, 0.5, "ORDER-BUY-NIL"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pnl != 0 {
		t.Errorf("pnl = %.4f, want 0 when db is nil", pnl)
	}
}

func TestSetStrategyName(t *testing.T) {
	repo := &mockTradingRepo{}
	calc := NewCalculator(repo, newLogger(t), "original")
	calc.SetStrategyName("updated")

	result := newResult("BUY", 1.0, 200.0, 0.0, "ORDER-STRATEGY")
	if _, err := calc.CalculateAndSave(result); err != nil {
		t.Fatal(err)
	}
	if len(repo.savedTrades) == 0 {
		t.Fatal("no trade saved")
	}
	if repo.savedTrades[0].StrategyName != "updated" {
		t.Errorf("strategyName = %q, want 'updated'", repo.savedTrades[0].StrategyName)
	}
}

func TestCalculateAndSave_GetOpenPositionsError_SellFallsBackToNegFee(t *testing.T) {
	repo := &mockTradingRepo{
		getOpenPosErr: fmt.Errorf("db error"),
	}
	calc := NewCalculator(repo, newLogger(t), "scalping")

	result := newResult("SELL", 1.0, 200.0, 0.5, "ORDER-SELL-ERR")
	// The calculator propagates the error from prepareSellData
	_, err := calc.CalculateAndSave(result)
	if err == nil {
		t.Fatal("expected error when GetOpenPositions fails, got nil")
	}
}
