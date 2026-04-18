package strategy

import "context"

// DummyStrategy is a minimal Strategy implementation used by unit tests in
// this package. It embeds *BaseStrategy to inherit lifecycle/metrics plumbing
// and returns a HOLD signal with the configured price.
type DummyStrategy struct {
	*BaseStrategy
}

// NewDummyStrategy creates a DummyStrategy with the given name.
func NewDummyStrategy(name string) *DummyStrategy {
	return &DummyStrategy{BaseStrategy: NewBaseStrategy(name, "test strategy", "0.0.0")}
}

// GenerateSignal returns a HOLD signal echoing the input price.
func (d *DummyStrategy) GenerateSignal(_ context.Context, data *MarketData, _ []MarketData) (*Signal, error) {
	if data == nil {
		return d.CreateSignal("", SignalHold, 0, 0, 0, nil), nil
	}
	return d.CreateSignal(data.Symbol, SignalHold, 0, data.Price, 0, nil), nil
}

// Analyze mirrors GenerateSignal over a batch.
func (d *DummyStrategy) Analyze(data []MarketData) (*Signal, error) {
	if len(data) == 0 {
		return d.CreateSignal("", SignalHold, 0, 0, 0, nil), nil
	}
	latest := data[len(data)-1]
	return d.GenerateSignal(context.Background(), &latest, data)
}

// Initialize stores the config map on the embedded BaseStrategy.
func (d *DummyStrategy) Initialize(config map[string]interface{}) error {
	d.BaseStrategy.config = config
	return nil
}

// UpdateConfig is an alias for Initialize.
func (d *DummyStrategy) UpdateConfig(config map[string]interface{}) error {
	return d.Initialize(config)
}
