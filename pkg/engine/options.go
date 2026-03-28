package engine

// Option is a functional option for engine configuration.
type Option func(*engineConfig)

type engineConfig struct {
	configPath string
}

func newEngineConfig() *engineConfig {
	return &engineConfig{
		configPath: "./configs/config.yaml",
	}
}

// WithConfigPath overrides the default config file path ("./configs/config.yaml").
func WithConfigPath(path string) Option {
	return func(c *engineConfig) {
		c.configPath = path
	}
}
