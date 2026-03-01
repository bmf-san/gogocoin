package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	// create test configuration file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
app:
  name: test-gogocoin
  version: 1.0.0
  environment: test

api:
  endpoint: https://api.bitflyer.com
  websocket_endpoint: wss://ws.lightstream.bitflyer.com/json-rpc
  credentials:
    api_key: test_key
    api_secret: test_secret
  timeout: 30s
  retry_count: 3

trading:
  initial_balance: 100000
  symbols:
    - BTC_JPY
  strategy:
    name: simple_test
  risk_management:
    max_total_loss_percent: 10.0
    max_trade_amount_percent: 5.0
    daily_loss_limit: 5000
    max_position_size_percent: 5.0

data:
  storage:
    type: sqlite
    path: ./test_data

ui:
  host: localhost
  port: 8080

logging:
  level: info
  format: json
  output: file
  file_path: ./test_logs/test.log
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// load configuration
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// verify configuration values
	if config.App.Name != "test-gogocoin" {
		t.Errorf("Expected app name 'test-gogocoin', got '%s'", config.App.Name)
	}

	if config.API.Endpoint != "https://api.bitflyer.com" {
		t.Errorf("Expected API endpoint 'https://api.bitflyer.com', got '%s'", config.API.Endpoint)
	}

	if config.Trading.InitialBalance != 100000 {
		t.Errorf("Expected initial balance 100000, got %f", config.Trading.InitialBalance)
	}

	if len(config.Trading.Symbols) != 1 {
		t.Errorf("Expected 1 symbol, got %d", len(config.Trading.Symbols))
	}
}

func TestLoad_EnvironmentVariableExpansion(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("TEST_API_KEY", "expanded_key")
	_ = os.Setenv("TEST_API_SECRET", "expanded_secret")
	defer func() {
		_ = os.Unsetenv("TEST_API_KEY")
		_ = os.Unsetenv("TEST_API_SECRET")
	}()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
app:
  name: test-gogocoin
  version: 1.0.0

api:
  endpoint: https://api.bitflyer.com
  credentials:
    api_key: "${TEST_API_KEY}"
    api_secret: "${TEST_API_SECRET}"

trading:
  initial_balance: 50000
  symbols:
    - BTC_JPY
  strategy:
    name: simple_test
  risk_management:
    max_total_loss_percent: 15.0
    max_trade_amount_percent: 3.0
    daily_loss_limit: 3000
    max_position_size_percent: 3.0

data:
  storage:
    type: sqlite
    path: ./test_data

ui:
  host: localhost
  port: 8080

logging:
  level: info
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify that environment variables are expanded
	if config.API.Credentials.APIKey != "expanded_key" {
		t.Errorf("Expected API key 'expanded_key', got '%s'", config.API.Credentials.APIKey)
	}

	if config.API.Credentials.APISecret != "expanded_secret" {
		t.Errorf("Expected API secret 'expanded_secret', got '%s'", config.API.Credentials.APISecret)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("nonexistent_config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid_config.yaml")

	invalidContent := `
app:
  name: test
  invalid_yaml: [
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name: "test-gogocoin",
		},
		API: APIConfig{
			Endpoint:          "https://api.bitflyer.com",
			WebSocketEndpoint: "wss://ws.lightstream.bitflyer.com/json-rpc",
			Credentials: CredentialsConfig{
				APIKey:    "test_key",
				APISecret: "test_secret",
			},
			Timeout:    30 * time.Second,
			RetryCount: 3,
		},
		Trading: TradingConfig{
			InitialBalance: 100000,
			Symbols:        []string{"BTC_JPY"},
			Strategy: StrategyConfig{
				Name: "simple_test",
			},
			RiskManagement: RiskManagementConfig{
				MaxTotalLossPercent:   20,
				MaxTradeAmountPercent: 5,
			},
		},
		UI: UIConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Valid config should not return error: %v", err)
	}
}

func TestValidate_EmptySymbols(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name: "test-gogocoin",
		},
		API: APIConfig{
			Endpoint: "https://api.bitflyer.com",
		},
		Trading: TradingConfig{
			InitialBalance: 100000,
			Symbols:        []string{}, // Empty symbols
			Strategy: StrategyConfig{
				Name: "simple_test",
			},
			RiskManagement: RiskManagementConfig{
				MaxTotalLossPercent:   20,
				MaxTradeAmountPercent: 5,
			},
		},
		UI: UIConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for empty symbols, got nil")
	}
}

func TestValidate_EmptyAppName(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name: "",
		},
		API: APIConfig{
			Endpoint: "https://api.bitflyer.com",
		},
		Trading: TradingConfig{
			InitialBalance: 100000,
			Symbols:        []string{"BTC_JPY"},
		},
		UI: UIConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for empty app name, got nil")
	}
}

func TestValidate_NegativeInitialBalance(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name: "test-gogocoin",
		},
		API: APIConfig{
			Endpoint: "https://api.bitflyer.com",
		},
		Trading: TradingConfig{
			InitialBalance: -1000,
			Symbols:        []string{"BTC_JPY"},
		},
		UI: UIConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for negative initial balance, got nil")
	}
}

// TestValidate_WithoutCredentials tests validation fails when API credentials are missing
func TestValidate_WithoutCredentials(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name: "test-gogocoin",
		},
		API: APIConfig{
			Endpoint: "https://api.bitflyer.com",
			Credentials: CredentialsConfig{
				APIKey:    "",
				APISecret: "",
			},
		},
		Trading: TradingConfig{
			InitialBalance: 100000,
			Symbols:        []string{"BTC_JPY"},
		},
		UI: UIConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error when credentials are missing, got nil")
	}
}

func TestGetStrategyParams_UnknownStrategy(t *testing.T) {
	config := &Config{}

	_, err := config.GetStrategyParams("unknown_strategy")
	if err == nil {
		t.Error("Expected error for unknown strategy, got nil")
	}
}

func TestExpandEnvVars(t *testing.T) {
	_ = os.Setenv("TEST_VAR", "test_value")
	defer func() { _ = os.Unsetenv("TEST_VAR") }()

	input := "prefix_${TEST_VAR}_suffix"
	expected := "prefix_test_value_suffix"

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestExpandEnvVars_UndefinedVariable(t *testing.T) {
	input := "prefix_${UNDEFINED_VAR}_suffix"
	expected := "prefix__suffix"

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
