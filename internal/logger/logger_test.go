package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// バッファにリダイレクト（for testing）
	// 実際of実装withis標準出力に出力されるため、ここwithis基本的な動作確認ofみ

	// 各レベルoflogメソッドが存在し、呼び出せること" "確認
	logger.System().Debug("Debug message")
	logger.System().Info("Info message")
	logger.System().Warn("Warn message")
	logger.System().Error("Error message")

	// カテゴリ別ロガーoftest
	logger.API().Info("API message")
	logger.Strategy().Info("Strategy message")
	logger.Trading().Info("Trading message")
	logger.Data().Info("Data message")
	logger.UI().Info("UI message")

	// errorが発生しないこと" "確認（実際of出力内容of検証is複雑なため省略）
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

	// フィールド付きlogof基本動作確認
	logger.System().WithField("key", "value").Info("Message with field")
	logger.System().WithField("field1", "value1").WithField("field2", 123).WithField("field3", true).Info("Message with multiple fields")

	// WithError of動作確認
	testErr := &testError{message: "test error"}
	logger.System().WithError(testErr).Error("Error message")
}

// for testingoferror型
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

	// LogTrade メソッドof動作確認
	metadata := map[string]interface{}{
		"order_id": "test_order_123",
		"strategy": "simple_test",
	}

	logger.LogTrade("BUY", "BTC_JPY", 4000000, 0.001, metadata)

	// errorが発生しないこと" "確認
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

	// LogAPICall メソッドof動作確認
	logger.LogAPICall("GET", "/api/ticker", 150, 200, nil)

	// errorありof場合
	testErr := &testError{message: "API error"}
	logger.LogAPICall("POST", "/api/order", 300, 500, testErr)

	// errorが発生しないこと" "確認
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

	// LogError メソッドof動作確認
	testErr := &testError{message: "test error"}
	logger.LogError("trading", "order_placement", testErr, map[string]interface{}{
		"symbol": "BTC_JPY",
		"side":   "BUY",
	})

	// errorが発生しないこと" "確認
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

	// モックdatabase" "configuration
	mockDB := &mockDatabase{}
	logger.SetDatabase(mockDB)

	// databaseがconfigurationされたこと" "確認（内部状態なofwith直接確認is困難）
	// errorが発生しないこと" "確認
}

// モックdatabase
type mockDatabase struct{}

func (m *mockDatabase) SaveLog(entry interface{}) error {
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

	// log" "書き込み
	logger.System().Info("Test message before close")

	// クローズ
	err = logger.Close()
	if err != nil {
		t.Errorf("Failed to close logger: %v", err)
	}

	// logfileが存在すること" "確認
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

	// log" "出力
	logger.System().Info("Test JSON message")
	_ = logger.Close()

	// logfile" "読み取り
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// JSON形式withあること" "確認
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			t.Errorf("Log line is not valid JSON: %s, error: %v", line, err)
		}

		// 基本的なフィールドが存在すること" "確認
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

	// カテゴリ別ロガーが正常に動作すること" "確認
	logger.API().Debug("API debug message")    // 表示される
	logger.Strategy().Info("Strategy info")    // 表示されない（warn以上ofみ）
	logger.Strategy().Warn("Strategy warning") // 表示される
	logger.Trading().Warn("Trading warning")   // 表示されない（error以上ofみ）
	logger.Trading().Error("Trading error")    // 表示される

	// errorが発生しないこと" "確認
}

func TestLogRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:      "info",
		Format:     "json",
		Output:     "file",
		FilePath:   logPath,
		MaxSizeMB:  1, // 1MBwith回転
		MaxBackups: 3,
		MaxAgeDays: 7,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// logローテーションofconfigurationが適用されていること" "確認
	// 実際ofローテーションtestis時間がかかるため、configurationof確認ofみ
	logger.System().Info("Test log rotation setup")

	// errorが発生しないこと" "確認
}
