package backtest

import "time"

// Position represents one open long position.
type Position struct {
	Symbol     string
	EntryTime  time.Time
	EntryPrice float64 // executed price after slippage
	Quantity   float64
	Notional   float64 // entry price * quantity (post-slippage)
	EntryFee   float64
	EntrySlip  float64 // entry slippage cost in JPY
	TPPrice    float64 // 0 = disabled
	SLPrice    float64 // 0 = disabled
	BarsHeld   int
}

// MarkValue returns the mark-to-market notional of the position at price p.
func (pos Position) MarkValue(p float64) float64 {
	return pos.Quantity * p
}

// Portfolio tracks cash, an optional open long position, equity history, and
// completed trades.
type Portfolio struct {
	cash     float64
	peak     float64 // peak equity for drawdown tracking
	position *Position
	trades   []Trade
	equity   []EquityPoint
}

// NewPortfolio creates a portfolio seeded with initial cash.
func NewPortfolio(initialCash float64) *Portfolio {
	return &Portfolio{
		cash: initialCash,
		peak: initialCash,
	}
}

// Cash returns realized cash.
func (p *Portfolio) Cash() float64 { return p.cash }

// Position returns a pointer to the open position or nil.
func (p *Portfolio) Position() *Position { return p.position }

// Trades returns the list of completed trades (live slice, do not mutate).
func (p *Portfolio) Trades() []Trade { return p.trades }

// Equity returns the equity curve (live slice, do not mutate).
func (p *Portfolio) Equity() []EquityPoint { return p.equity }

// MarkToMarket records an equity sample at price p / time t.
func (p *Portfolio) MarkToMarket(t time.Time, price float64) {
	eq := p.cash
	if p.position != nil {
		eq += p.position.MarkValue(price)
	}
	if eq > p.peak {
		p.peak = eq
	}
	dd := 0.0
	if p.peak > 0 {
		dd = (eq - p.peak) / p.peak
	}
	p.equity = append(p.equity, EquityPoint{
		Timestamp: t,
		Cash:      p.cash,
		Equity:    eq,
		Drawdown:  dd,
	})
}

// OpenLong opens a long position. Caller is responsible for slippage / fee
// calculation; we only update cash and position state here.
func (p *Portfolio) OpenLong(pos Position) {
	p.position = &pos
	p.cash -= pos.Notional + pos.EntryFee + pos.EntrySlip
}

// CloseLong closes the open position at exitPrice with the given fee/slippage.
// Returns the recorded Trade. Panics if no position is open.
func (p *Portfolio) CloseLong(exitTime time.Time, exitPrice, exitFee, exitSlip float64, reason ExitReason) Trade {
	if p.position == nil {
		panic("backtest: CloseLong with no open position")
	}
	pos := p.position
	gross := (exitPrice - pos.EntryPrice) * pos.Quantity
	totalFee := pos.EntryFee + exitFee
	totalSlip := pos.EntrySlip + exitSlip
	net := gross - totalFee - totalSlip
	exitProceeds := exitPrice * pos.Quantity
	p.cash += exitProceeds - exitFee - exitSlip
	tr := Trade{
		Symbol:     pos.Symbol,
		Side:       SideLong,
		EntryTime:  pos.EntryTime,
		EntryPrice: pos.EntryPrice,
		ExitTime:   exitTime,
		ExitPrice:  exitPrice,
		Quantity:   pos.Quantity,
		Notional:   pos.Notional,
		GrossPnL:   gross,
		Fee:        totalFee,
		Slippage:   totalSlip,
		NetPnL:     net,
		Reason:     reason,
		HoldBars:   pos.BarsHeld,
	}
	p.trades = append(p.trades, tr)
	p.position = nil
	return tr
}
