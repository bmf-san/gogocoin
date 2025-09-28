package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogEntry is a log entry structure（database.LogEntryofエイリアス）
type LogEntry struct {
	ID        int       `json:"id"`
	Level     string    `json:"level"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
	Fields    string    `json:"fields"`
	Timestamp time.Time `json:"timestamp"`
}

// DatabaseLogger isinterface for saving logs to database
type DatabaseLogger interface {
	SaveLog(entry interface{}) error
}

// Logger isアプリケーション全体with使用するロガー
type Logger struct {
	*slog.Logger
	categories map[string]*slog.Logger
	config     *Config
	mu         sync.RWMutex
	db         DatabaseLogger
}

// Config isロガーofconfiguration
type Config struct {
	Level      string            `yaml:"level"`
	Format     string            `yaml:"format"`
	Output     string            `yaml:"output"`
	FilePath   string            `yaml:"file_path"`
	MaxSizeMB  int               `yaml:"max_size_mb"`
	MaxBackups int               `yaml:"max_backups"`
	MaxAgeDays int               `yaml:"max_age_days"`
	Categories map[string]string `yaml:"categories"`
}

// LogLevel islogレベルrepresents
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// LogCategory islogカテゴリrepresents
type LogCategory string

const (
	CategorySystem   LogCategory = "system"
	CategoryTrading  LogCategory = "trading"
	CategoryAPI      LogCategory = "api"
	CategoryStrategy LogCategory = "strategy"
	CategoryUI       LogCategory = "ui"
	CategoryData     LogCategory = "data"
)

// String isLogLevelof文字列表現returns
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel is文字列fromLogLevel" "解析する
func ParseLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

// ToSlogLevel isLogLevel" "slog.Levelに変換する
func (l LogLevel) ToSlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New is新しいロガーcreates
func New(config *Config) (*Logger, error) {
	// 出力先" "configuration
	writer, err := createWriter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	// ハンドラー" "作成
	handler, err := createHandler(config, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	// メインロガー" "作成
	mainLogger := slog.New(handler)

	// カテゴリ別ロガー" "作成
	categories := make(map[string]*slog.Logger)

	// 定義済みカテゴリ" "初期化
	allCategories := []LogCategory{
		CategorySystem,
		CategoryTrading,
		CategoryAPI,
		CategoryStrategy,
		CategoryUI,
		CategoryData,
	}

	for _, category := range allCategories {
		categoryStr := string(category)

		// configから該当カテゴリofレベル" "取得（なければdefaultレベル" "使用）
		categoryLevel := config.Level // default
		if configLevel, exists := config.Categories[categoryStr]; exists {
			categoryLevel = configLevel
		}

		catLevel, err := ParseLevel(categoryLevel)
		if err != nil {
			return nil, fmt.Errorf("invalid log level for category %s: %w", categoryStr, err)
		}

		// カテゴリ別ハンドラー" "作成
		catHandler, err := createHandlerWithLevel(config, writer, catLevel)
		if err != nil {
			return nil, fmt.Errorf("failed to create handler for category %s: %w", categoryStr, err)
		}

		categories[categoryStr] = slog.New(catHandler)
	}

	return &Logger{
		Logger:     mainLogger,
		categories: categories,
		config:     config,
		db:         nil, // 後withSetDatabasewithconfiguration
	}, nil
}

// SetDatabase isdatabaseロガーsets
func (l *Logger) SetDatabase(db DatabaseLogger) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.db = db
}

// saveToDatabase islog" "databaseに保存する
func (l *Logger) saveToDatabase(level, category, message string, fields map[string]interface{}) {
	l.mu.RLock()
	db := l.db
	l.mu.RUnlock()

	if db == nil {
		return
	}

	// フィールド" "JSON文字列に変換
	fieldsJSON := ""
	if len(fields) > 0 {
		if data, err := json.Marshal(fields); err == nil {
			fieldsJSON = string(data)
		}
	}

	entry := LogEntry{
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    fieldsJSON,
		Timestamp: time.Now(),
	}

	if err := db.SaveLog(entry); err != nil {
		// Ignore database save errors to avoid infinite loops
		// (log output itself continues)
		_ = err // explicitly ignore
	}
}

// createWriter creates the output writer
func createWriter(config *Config) (io.Writer, error) {
	switch strings.ToLower(config.Output) {
	case "console":
		return os.Stdout, nil
	case "file":
		return createFileWriter(config)
	case "both":
		fileWriter, err := createFileWriter(config)
		if err != nil {
			return nil, err
		}
		return io.MultiWriter(os.Stdout, fileWriter), nil
	default:
		return nil, fmt.Errorf("unsupported log output: %s", config.Output)
	}
}

// createFileWriter isfile出力用ofWritercreates
func createFileWriter(config *Config) (io.Writer, error) {
	// ディレクトリ" "作成
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// simpleなfile出力（ローテーションis外部ツールに委任）
	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return file, nil
}

// createHandler isslog.Handlercreates
func createHandler(config *Config, writer io.Writer) (slog.Handler, error) {
	level, err := ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	return createHandlerWithLevel(config, writer, level)
}

// createHandlerWithLevel is指定されたレベルwithslog.Handlercreates
func createHandlerWithLevel(config *Config, writer io.Writer, level LogLevel) (slog.Handler, error) {
	opts := &slog.HandlerOptions{
		Level: level.ToSlogLevel(),
	}

	switch strings.ToLower(config.Format) {
	case "json":
		return slog.NewJSONHandler(writer, opts), nil
	case "text":
		return slog.NewTextHandler(writer, opts), nil
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}
}

// Category is指定されたカテゴリofロガーgets
func (l *Logger) Category(category string) *ExtendedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var logger *slog.Logger
	if categoryLogger, exists := l.categories[category]; exists {
		logger = categoryLogger.With("category", category)
	} else {
		// カテゴリが存在しない場合isメインロガー" "使用
		logger = l.With("category", category)
	}

	return &ExtendedLogger{
		Logger:   logger,
		parent:   l,
		category: category,
	}
}

// WithFields isフィールド付きofロガーgets
func (l *Logger) WithFields(fields map[string]interface{}) *slog.Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return l.With(args...)
}

// WithField isフィールド付きofロガーgets
func (l *Logger) WithField(key string, value interface{}) *slog.Logger {
	return l.With(key, value)
}

// Trading istrading関連ofロガーgets
func (l *Logger) Trading() *ExtendedLogger {
	return l.Category(string(CategoryTrading))
}

// API isAPI関連ofロガーgets
func (l *Logger) API() *ExtendedLogger {
	return l.Category(string(CategoryAPI))
}

// Strategy isstrategy関連ofロガーgets
func (l *Logger) Strategy() *ExtendedLogger {
	return l.Category(string(CategoryStrategy))
}

// UI isUI関連ofロガーgets
func (l *Logger) UI() *ExtendedLogger {
	return l.Category(string(CategoryUI))
}

// Data isdata関連ofロガーgets
func (l *Logger) Data() *ExtendedLogger {
	return l.Category(string(CategoryData))
}

// System isシステム関連ofロガーgets
func (l *Logger) System() *ExtendedLogger {
	return l.Category(string(CategorySystem))
}

// LogTrade istradinglog" "出力する
func (l *Logger) LogTrade(action, symbol string, price, quantity float64, fields map[string]interface{}) {
	logger := l.Trading().With(
		"action", action,
		"symbol", symbol,
		"price", price,
		"quantity", quantity,
	)

	if fields != nil {
		args := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Info("Trade executed")
}

// LogAPICall isAPI呼び出しlog" "出力する
func (l *Logger) LogAPICall(method, endpoint string, duration int64, statusCode int, err error) {
	logger := l.API().With(
		"method", method,
		"endpoint", endpoint,
		"duration_ms", duration,
		"status_code", statusCode,
	)

	if err != nil {
		logger.Error("API call failed", "error", err)
	} else {
		logger.Info("API call completed")
	}
}

// LogStrategySignal isstrategysignallog" "出力する
func (l *Logger) LogStrategySignal(strategy, symbol, action string, strength float64, metadata map[string]interface{}) {
	logger := l.Strategy().With(
		"strategy", strategy,
		"symbol", symbol,
		"action", action,
		"strength", strength,
	)

	if metadata != nil {
		args := make([]interface{}, 0, len(metadata)*2)
		for k, v := range metadata {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Info("Strategy signal generated")
}

// LogError iserrorlog" "出力する
func (l *Logger) LogError(category string, operation string, err error, fields map[string]interface{}) {
	logger := l.Category(category).With("operation", operation)

	if fields != nil {
		args := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Error("Operation failed", "error", err)
}

// LogPerformance isperformancelog" "出力する
func (l *Logger) LogPerformance(operation string, duration int64, fields map[string]interface{}) {
	logger := l.System().With(
		"operation", operation,
		"duration_ms", duration,
	)

	if fields != nil {
		args := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	if duration > 1000 { // 1秒以上of場合is警告
		logger.Warn("Slow operation detected")
	} else {
		logger.Debug("Operation completed")
	}
}

// LogStartup isアプリケーション起動log" "出力する
func (l *Logger) LogStartup(version, mode string, config map[string]interface{}) {
	logger := l.System().With(
		"version", version,
		"mode", mode,
	)

	if config != nil {
		configJSON, _ := json.Marshal(config)
		logger = logger.With("config", string(configJSON))
	}

	logger.Info("Application started")
}

// LogShutdown isアプリケーション終了log" "出力する
func (l *Logger) LogShutdown(reason string) {
	l.System().With("reason", reason).Info("Application shutting down")
}

// Error iserrorlog" "出力する（compatibilityofため）
func (l *Logger) Error(msg string) {
	l.Logger.Error(msg)
}

// Flush isバッファされたlog" "強制的に出力する
func (l *Logger) Flush() {
	// slogis自動的にフラッシュされるため、特に何もしない
}

// Close isロガー" "クローズする
func (l *Logger) Close() error {
	// 必要に応じてリソース" "クリーンアップ
	return nil
}

// SetLevel is動的にlogレベル" "変更する
func (l *Logger) SetLevel(level string) error {
	logLevel, err := ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// 新しいハンドラー" "作成して置き換える
	writer, err := createWriter(l.config)
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}

	handler, err := createHandlerWithLevel(l.config, writer, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create handler: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.Logger = slog.New(handler)

	// カテゴリ別ロガーも更新
	for category := range l.categories {
		catHandler, err := createHandlerWithLevel(l.config, writer, logLevel)
		if err != nil {
			continue
		}
		l.categories[category] = slog.New(catHandler)
	}

	return nil
}

// GetLevel is現在oflogレベルgets
func (l *Logger) GetLevel() string {
	return l.config.Level
}

// WithError iserror付きofロガーgets（compatibilityofため）
func (l *Logger) WithError(err error) *slog.Logger {
	return l.With("error", err)
}

// ExtendedLogger isslog.Logger" "拡張したラッパー
type ExtendedLogger struct {
	*slog.Logger
	parent   *Logger
	category string
}

// Info isInfoレベルoflog" "出力し、databaseにも保存する
func (el *ExtendedLogger) Info(msg string, args ...interface{}) {
	el.Logger.Info(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("INFO", el.category, msg, fields)
	}
}

// Error isErrorレベルoflog" "出力し、databaseにも保存する
func (el *ExtendedLogger) Error(msg string, args ...interface{}) {
	el.Logger.Error(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("ERROR", el.category, msg, fields)
	}
}

// Warn isWarnレベルoflog" "出力し、databaseにも保存する
func (el *ExtendedLogger) Warn(msg string, args ...interface{}) {
	el.Logger.Warn(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("WARN", el.category, msg, fields)
	}
}

// Debug isDebugレベルoflog" "出力し、databaseにも保存する
func (el *ExtendedLogger) Debug(msg string, args ...interface{}) {
	el.Logger.Debug(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("DEBUG", el.category, msg, fields)
	}
}

// WithError iserror付きofロガーgets
func (el *ExtendedLogger) WithError(err error) *ExtendedLogger {
	return &ExtendedLogger{
		Logger:   el.With("error", err),
		parent:   el.parent,
		category: el.category,
	}
}

// WithField isフィールド付きofロガーgets
func (el *ExtendedLogger) WithField(key string, value interface{}) *ExtendedLogger {
	return &ExtendedLogger{
		Logger:   el.With(key, value),
		parent:   el.parent,
		category: el.category,
	}
}

// argsToMap is可変引数" "マップに変換する
func argsToMap(args []interface{}) map[string]interface{} {
	fields := make(map[string]interface{})
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok && i+1 < len(args) {
			fields[key] = args[i+1]
		}
	}
	return fields
}
