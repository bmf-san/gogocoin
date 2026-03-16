package engine

import (
	pkgstrategy "github.com/bmf-san/gogocoin/v1/pkg/strategy"
)

// Option is a functional option for engine configuration.
type Option func(*engineConfig)

type engineConfig struct {
	registry   *pkgstrategy.Registry
	configPath string
}

func newEngineConfig() *engineConfig {
	return &engineConfig{
		registry:   pkgstrategy.NewRegistry(),
		configPath: "./configs/config.yaml",
	}
}

// WithStrategy registers a strategy constructor under name.
// The name must match the value of trading.strategy.name in config.yaml.
func WithStrategy(name string, ctor pkgstrategy.Constructor) Option {
	return func(c *engineConfig) {
		c.registry.Register(name, ctor)
	}
}

// WithConfigPath overrides the default config file path ("./configs/config.yaml").
func WithConfigPath(path string) Option {
	return func(c *engineConfig) {
		c.configPath = path
	}
}
