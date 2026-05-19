package worker

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// observingStrategy records every Analyze call so the test can assert which
// symbols the strategy actually saw.
type observingStrategy struct {
	mockStrategyWithConfig
	seen chan string
}

func (s *observingStrategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) {
	if len(data) > 0 {
		select {
		case s.seen <- data[len(data)-1].Symbol:
		default:
		}
	}
	return &strategy.Signal{Action: strategy.SignalHold}, nil
}

func TestStrategyWorker_SetTradeSymbols_FiltersObserveOnly(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:    "error",
		Format:   "json",
		Output:   "file",
		FilePath: "/dev/null",
	})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	strat := &observingStrategy{
		mockStrategyWithConfig: mockStrategyWithConfig{cfg: map[string]any{}},
		seen:                   make(chan string, 16),
	}
	marketDataCh := make(chan domain.MarketData, 8)
	signalCh := make(chan *strategy.Signal, 16)
	w := NewStrategyWorker(log, strat, marketDataCh, signalCh)
	w.SetTradeSymbols([]string{"XRP_JPY"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx)
		close(done)
	}()

	// Two ticks for the trade symbol, one for an observe-only symbol.
	marketDataCh <- domain.MarketData{Symbol: "XRP_JPY", Price: 220, Timestamp: time.Now()}
	marketDataCh <- domain.MarketData{Symbol: "MONA_JPY", Price: 14, Timestamp: time.Now()}
	marketDataCh <- domain.MarketData{Symbol: "XRP_JPY", Price: 221, Timestamp: time.Now()}

	// Give the worker time to drain the channel.
	deadline := time.After(2 * time.Second)
	gotXRP := 0
	gotOther := 0
loop:
	for {
		select {
		case s := <-strat.seen:
			if s == "XRP_JPY" {
				gotXRP++
				if gotXRP >= 2 {
					break loop
				}
			} else {
				gotOther++
			}
		case <-deadline:
			break loop
		}
	}

	cancel()
	<-done

	if gotXRP != 2 {
		t.Errorf("expected XRP_JPY Analyze calls = 2, got %d", gotXRP)
	}
	if gotOther != 0 {
		t.Errorf("expected observe-only symbol to be filtered, but strategy was invoked %d time(s) with non-trade symbol", gotOther)
	}
}

func TestStrategyWorker_SetTradeSymbols_NilDisablesFilter(t *testing.T) {
	w := newTestStrategyWorker(t, map[string]any{})
	w.SetTradeSymbols([]string{"XRP_JPY"})
	if w.tradeSymbols == nil {
		t.Fatal("expected filter set after non-empty SetTradeSymbols")
	}
	w.SetTradeSymbols(nil)
	if w.tradeSymbols != nil {
		t.Errorf("expected nil filter after SetTradeSymbols(nil), got %v", w.tradeSymbols)
	}
	w.SetTradeSymbols([]string{})
	if w.tradeSymbols != nil {
		t.Errorf("expected nil filter after SetTradeSymbols(empty), got %v", w.tradeSymbols)
	}
}
