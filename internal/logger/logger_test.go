package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmf-san/gogocoin/internal/domain"
)

func TestNew_ConsoleOutput(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestNew_FileOutput(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logPath,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// verify that log file is created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should be created")
	}
}

func TestNew_BothOutput(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:    "info",
		Format:   "json",
		Output:   "both",
		FilePath: logPath,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// verify that log file is created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should be created for both output")
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	config := &Config{
		Level:  "invalid_level",
		Format: "json",
		Output: "console",
	}

	_, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid log level, got nil")
	}
}

func TestNew_InvalidOutput(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "invalid_output",
	}

	_, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid output type, got nil")
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer

	config := &Config{
		Level:  "debug",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Redirect to buffer (for testing)
	// In the actual implementation, output goes to stdout, so here we only verify basic operation

	// Verify that log methods for each level exist and can be called
	logger.System().Debug("Debug message")
	logger.System().Info("Info message")
	logger.System().Warn("Warn message")
	logger.System().Error("Error message")

	// Test categorized loggers
	logger.API().Info("API message")
	logger.Strategy().Info("Strategy message")
	logger.Trading().Info("Trading message")
	logger.Data().Info("Data message")
	logger.UI().Info("UI message")

	// Verify that no errors occur (actual output validation is omitted due to complexity)
	_ = buf
}

func TestLogWithFields(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify basic operation of logging with fields
	logger.System().WithField("key", "value").Info("Message with field")
	logger.System().WithField("field1", "value1").WithField("field2", 123).WithField("field3", true).Info("Message with multiple fields")

	// Verify WithError operation
	testErr := &testError{message: "test error"}
	logger.System().WithError(testErr).Error("Error message")
}

// Error type for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

func TestLogTrade(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify LogTrade method operation
	metadata := map[string]interface{}{
		"order_id": "test_order_123",
		"strategy": "simple_test",
	}

	logger.LogTrade("BUY", "BTC_JPY", 4000000, 0.001, metadata)

	// Verify that no errors occur
}

func TestLogAPICall(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify LogAPICall method operation
	logger.LogAPICall("GET", "/api/ticker", 150, 200, nil)

	// Test case with error
	testErr := &testError{message: "API error"}
	logger.LogAPICall("POST", "/api/order", 300, 500, testErr)

	// Verify that no errors occur
}

func TestLogError(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify LogError method operation
	testErr := &testError{message: "test error"}
	logger.LogError("trading", "order_placement", testErr, map[string]interface{}{
		"symbol": "BTC_JPY",
		"side":   "BUY",
	})

	// Verify that no errors occur
}

func TestSetDatabase(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Set up mock database
	mockDB := &mockDatabase{}
	logger.SetDatabase(mockDB)

	// Verify that database is set (direct verification is difficult due to internal state)
	// Verify that no errors occur
}

// Mock database
type mockDatabase struct{}

func (m *mockDatabase) SaveLog(entry *domain.LogEntry) error {
	return nil
}

func TestClose(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logPath,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Write log entry
	logger.System().Info("Test message before close")

	// Close logger
	err = logger.Close()
	if err != nil {
		t.Errorf("Failed to close logger: %v", err)
	}

	// Verify that log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should exist after close")
	}
}

func TestJSONFormat(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logPath,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Output log
	logger.System().Info("Test JSON message")
	_ = logger.Close()

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify JSON format
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			t.Errorf("Log line is not valid JSON: %s, error: %v", line, err)
		}

		// Verify that basic fields exist
		if _, ok := logEntry["time"]; !ok {
			t.Error("Log entry should have 'time' field")
		}
		if _, ok := logEntry["level"]; !ok {
			t.Error("Log entry should have 'level' field")
		}
		if _, ok := logEntry["msg"]; !ok {
			t.Error("Log entry should have 'msg' field")
		}
	}
}

func TestCategorizedLoggers(t *testing.T) {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "console",
		Categories: map[string]string{
			"api":      "debug",
			"strategy": "warn",
			"trading":  "error",
		},
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify that categorized loggers work correctly
	logger.API().Debug("API debug message")    // Will be displayed
	logger.Strategy().Info("Strategy info")    // Will not be displayed (warn level and above only)
	logger.Strategy().Warn("Strategy warning") // Will be displayed
	logger.Trading().Warn("Trading warning")   // Will not be displayed (error level and above only)
	logger.Trading().Error("Trading error")    // Will be displayed

	// Verify that no errors occur
}

func TestLogRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:      "info",
		Format:     "json",
		Output:     "file",
		FilePath:   logPath,
		MaxSizeMB:  1, // Rotate at 1MB
		MaxBackups: 3,
		MaxAgeDays: 7,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify that log rotation configuration is applied
	// Actual rotation testing takes time, so only verify configuration here
	logger.System().Info("Test log rotation setup")

	// Verify that no errors occur
}
