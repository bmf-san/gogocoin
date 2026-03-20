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

	"github.com/bmf-san/gogocoin/internal/domain"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogEntry is removed - use domain.LogEntry instead

// LoggerInterface defines the interface for application logging
// Following Dependency Inversion Principle, components depend on this interface
// rather than the concrete Logger implementation
type LoggerInterface interface {
	// Category loggers
	System() *ExtendedLogger
	Trading() *ExtendedLogger
	API() *ExtendedLogger
	Strategy() *ExtendedLogger
	UI() *ExtendedLogger
	Data() *ExtendedLogger
	Category(category string) *ExtendedLogger

	// Field helpers
	WithFields(fields map[string]any) *slog.Logger
	WithField(key string, value any) *slog.Logger
	WithError(err error) *slog.Logger

	// Specialized logging methods
	LogTrade(action, symbol string, price, quantity float64, fields map[string]any)
	LogAPICall(method, endpoint string, duration int64, statusCode int, err error)
	LogStrategySignal(strategy, symbol, action string, strength float64, metadata map[string]any)
	LogError(category string, operation string, err error, fields map[string]any)
	LogPerformance(operation string, duration int64, fields map[string]any)
	LogStartup(version string, config map[string]any)
	LogShutdown(reason string)

	// Basic methods
	Error(msg string)

	// Database integration
	SetDatabase(db domain.LogRepository)

	// Lifecycle
	Flush()
	Close() error

	// Configuration
	SetLevel(level string) error
	GetLevel() string
}

// Pools for reducing GC pressure on high-frequency logging operations
var (
	// mapPool provides pooling for map allocations in argsToMap
	mapPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 8) // Pre-allocate for common case
		},
	}

)

// Logger is the logger used throughout the application
type Logger struct {
	*slog.Logger
	categories map[string]*slog.Logger
	config     *Config
	mu         sync.RWMutex
	db         domain.LogRepository
	file       io.Closer // File handle for cleanup (may be *os.File or *lumberjack.Logger)
}

// Verify that Logger implements LoggerInterface at compile time
var _ LoggerInterface = (*Logger)(nil)

// Config represents the logger configuration
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

// LogLevel represents the log level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// LogCategory represents the log category
type LogCategory string

const (
	CategorySystem   LogCategory = "system"
	CategoryTrading  LogCategory = "trading"
	CategoryAPI      LogCategory = "api"
	CategoryStrategy LogCategory = "strategy"
	CategoryUI       LogCategory = "ui"
	CategoryData     LogCategory = "data"
)

// String returns the string representation of LogLevel
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

// ParseLevel parses a LogLevel from a string
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

// ToSlogLevel converts LogLevel to slog.Level
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

// New creates a new logger
func New(config *Config) (*Logger, error) {
	// Configure the output destination
	writer, file, err := createWriter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	// Create handler
	handler, err := createHandler(config, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	// Create main logger
	mainLogger := slog.New(handler)

	// Create category-specific loggers
	categories := make(map[string]*slog.Logger)

	// Initialize predefined categories
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

		// Get the level for this category from config (use default level if not specified)
		categoryLevel := config.Level // default
		if configLevel, exists := config.Categories[categoryStr]; exists {
			categoryLevel = configLevel
		}

		catLevel, err := ParseLevel(categoryLevel)
		if err != nil {
			return nil, fmt.Errorf("invalid log level for category %s: %w", categoryStr, err)
		}

		// Create category-specific handler
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
		db:         nil, // Will be configured later with SetDatabase
		file:       file,
	}, nil
}

// SetDatabase sets the database logger
func (l *Logger) SetDatabase(db domain.LogRepository) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.db = db
}

// saveToDatabase saves a log to the database
func (l *Logger) saveToDatabase(level, category, message string, fields map[string]any) {
	l.mu.RLock()
	db := l.db
	l.mu.RUnlock()

	if db == nil {
		fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Database connection is nil, cannot save log (category: %s, message: %s)\n", category, message)
		return
	}

	// Skip DEBUG level logs to prevent high-frequency logs from filling the database
	// DEBUG logs are still output to console/file for real-time monitoring
	if level == "DEBUG" {
		return
	}

	// Skip saving high-frequency ticker data logs from the data category
	// These occur very frequently and would quickly fill the database
	if category == string(CategoryData) {
		return
	}

	// Create a copy of fields to avoid holding reference to pooled map
	// The original pooled map will be returned to the pool by the caller
	fieldsCopy := make(map[string]any, len(fields))
	for k, v := range fields {
		fieldsCopy[k] = v
	}

	entry := &domain.LogEntry{
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    fieldsCopy, // Use the copy, not the pooled map
		Timestamp: time.Now(),
	}

	if err := db.SaveLog(entry); err != nil {
		// Log database errors to console only (not to DB to avoid infinite loops)
		// This helps identify why logs stop being saved
		fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to save log to database: %v (category: %s, message: %s)\n", err, category, message)
	}
}

// createWriter creates the output writer.
// Returns (writer, closer, error). closer must be called on shutdown when non-nil.
func createWriter(config *Config) (io.Writer, io.Closer, error) {
	switch strings.ToLower(config.Output) {
	case "console":
		return os.Stdout, nil, nil
	case "file":
		return createFileWriter(config)
	case "both":
		fileWriter, closer, err := createFileWriter(config)
		if err != nil {
			return nil, nil, err
		}
		return io.MultiWriter(os.Stdout, fileWriter), closer, nil
	default:
		return nil, nil, fmt.Errorf("unsupported log output: %s", config.Output)
	}
}

// createFileWriter creates a Writer for file output using lumberjack for
// automatic rotation based on max_size_mb, max_backups, and max_age_days.
func createFileWriter(config *Config) (io.Writer, io.Closer, error) {
	maxSize := config.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 50 // 50 MB default
	}
	maxBackups := config.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 3
	}
	maxAge := config.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 7
	}

	rotator := &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
		LocalTime:  false, // use UTC for backup suffix timestamps
	}

	// lumberjack opens the file lazily on first Write. Touch the file now so
	// that it exists immediately (tests and monitoring tools may check for it).
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	if f, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
		_ = f.Close()
	}

	return rotator, rotator, nil
}

// createHandler creates a slog.Handler
func createHandler(config *Config, writer io.Writer) (slog.Handler, error) {
	level, err := ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	return createHandlerWithLevel(config, writer, level)
}

// createHandlerWithLevel creates a slog.Handler with the specified level
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

// Category gets a logger for the specified category
func (l *Logger) Category(category string) *ExtendedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var logger *slog.Logger
	if categoryLogger, exists := l.categories[category]; exists {
		logger = categoryLogger.With("category", category)
	} else {
		// Use main logger if category does not exist
		logger = l.With("category", category)
	}

	return &ExtendedLogger{
		Logger:   logger,
		parent:   l,
		category: category,
	}
}

// WithFields gets a logger with fields
func (l *Logger) WithFields(fields map[string]any) *slog.Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return l.With(args...)
}

// WithField gets a logger with a field
func (l *Logger) WithField(key string, value any) *slog.Logger {
	return l.With(key, value)
}

// Trading gets a logger for trading-related operations
func (l *Logger) Trading() *ExtendedLogger {
	return l.Category(string(CategoryTrading))
}

// API gets a logger for API-related operations
func (l *Logger) API() *ExtendedLogger {
	return l.Category(string(CategoryAPI))
}

// Strategy gets a logger for strategy-related operations
func (l *Logger) Strategy() *ExtendedLogger {
	return l.Category(string(CategoryStrategy))
}

// UI gets a logger for UI-related operations
func (l *Logger) UI() *ExtendedLogger {
	return l.Category(string(CategoryUI))
}

// Data gets a logger for data-related operations
func (l *Logger) Data() *ExtendedLogger {
	return l.Category(string(CategoryData))
}

// System gets a logger for system-related operations
func (l *Logger) System() *ExtendedLogger {
	return l.Category(string(CategorySystem))
}

// LogTrade outputs a trading log
func (l *Logger) LogTrade(action, symbol string, price, quantity float64, fields map[string]any) {
	logger := l.Trading().With(
		"action", action,
		"symbol", symbol,
		"price", price,
		"quantity", quantity,
	)

	if fields != nil {
		args := make([]any, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Info("Trade executed")
}

// LogAPICall outputs an API call log
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

// LogStrategySignal outputs a strategy signal log
func (l *Logger) LogStrategySignal(strategy, symbol, action string, strength float64, metadata map[string]any) {
	logger := l.Strategy().With(
		"strategy", strategy,
		"symbol", symbol,
		"action", action,
		"strength", strength,
	)

	if metadata != nil {
		args := make([]any, 0, len(metadata)*2)
		for k, v := range metadata {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Info("Strategy signal generated")
}

// LogError outputs an error log
func (l *Logger) LogError(category string, operation string, err error, fields map[string]any) {
	logger := l.Category(category).With("operation", operation)

	if fields != nil {
		args := make([]any, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	logger.Error("Operation failed", "error", err)
}

// LogPerformance outputs a performance log
func (l *Logger) LogPerformance(operation string, duration int64, fields map[string]any) {
	logger := l.System().With(
		"operation", operation,
		"duration_ms", duration,
	)

	if fields != nil {
		args := make([]any, 0, len(fields)*2)
		for k, v := range fields {
			args = append(args, k, v)
		}
		logger = logger.With(args...)
	}

	if duration > 1000 { // Warn if operation takes more than 1 second
		logger.Warn("Slow operation detected")
	} else {
		logger.Debug("Operation completed")
	}
}

// LogStartup outputs an application startup log
func (l *Logger) LogStartup(version string, config map[string]any) {
	logger := l.System().With("version", version)

	if config != nil {
		configJSON, _ := json.Marshal(config)
		logger = logger.With("config", string(configJSON))
	}

	logger.Info("Application started")
}

// LogShutdown outputs an application shutdown log
func (l *Logger) LogShutdown(reason string) {
	l.System().With("reason", reason).Info("Application shutting down")
}

// Error outputs an error log
func (l *Logger) Error(msg string) {
	l.Logger.Error(msg)
}

// Flush is a no-op method.
// The underlying slog handler automatically flushes on each write operation.
func (l *Logger) Flush() {
	// No-op: slog automatically flushes on each write
}

// Close closes the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		l.file = nil
	}

	return nil
}

// SetLevel dynamically changes the log level
func (l *Logger) SetLevel(level string) error {
	logLevel, err := ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// Create and replace with a new handler
	writer, file, err := createWriter(l.config)
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}

	handler, err := createHandlerWithLevel(l.config, writer, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create handler: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Close existing file handle if exists
	if l.file != nil {
		_ = l.file.Close()
	}

	l.Logger = slog.New(handler)
	l.file = file
	l.config.Level = level // Update config to reflect the new level

	// Update category-specific loggers as well
	for category := range l.categories {
		catHandler, err := createHandlerWithLevel(l.config, writer, logLevel)
		if err != nil {
			continue
		}
		l.categories[category] = slog.New(catHandler)
	}

	return nil
}

// GetLevel gets the current log level
func (l *Logger) GetLevel() string {
	return l.config.Level
}

// WithError gets a logger with an error
func (l *Logger) WithError(err error) *slog.Logger {
	return l.With("error", err)
}

// ExtendedLogger is a wrapper that extends slog.Logger
type ExtendedLogger struct {
	*slog.Logger
	parent   *Logger
	category string
}

// Info outputs an Info level log and saves it to the database
func (el *ExtendedLogger) Info(msg string, args ...any) {
	el.Logger.Info(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("INFO", el.category, msg, fields)
		// Return map to pool after saveToDatabase completes (sync call)
		mapPool.Put(fields)
	}
}

// Error outputs an Error level log and saves it to the database
func (el *ExtendedLogger) Error(msg string, args ...any) {
	el.Logger.Error(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("ERROR", el.category, msg, fields)
		// Return map to pool after saveToDatabase completes (sync call)
		mapPool.Put(fields)
	}
}

// Warn outputs a Warn level log and saves it to the database
func (el *ExtendedLogger) Warn(msg string, args ...any) {
	el.Logger.Warn(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("WARN", el.category, msg, fields)
		// Return map to pool after saveToDatabase completes (sync call)
		mapPool.Put(fields)
	}
}

// Debug outputs a Debug level log and saves it to the database
func (el *ExtendedLogger) Debug(msg string, args ...any) {
	el.Logger.Debug(msg, args...)
	if el.parent != nil {
		fields := argsToMap(args)
		el.parent.saveToDatabase("DEBUG", el.category, msg, fields)
		// Return map to pool after saveToDatabase completes (sync call)
		mapPool.Put(fields)
	}
}

// WithError gets a logger with an error
func (el *ExtendedLogger) WithError(err error) *ExtendedLogger {
	return &ExtendedLogger{
		Logger:   el.With("error", err),
		parent:   el.parent,
		category: el.category,
	}
}

// WithField gets a logger with a field
func (el *ExtendedLogger) WithField(key string, value any) *ExtendedLogger {
	return &ExtendedLogger{
		Logger:   el.With(key, value),
		parent:   el.parent,
		category: el.category,
	}
}

// argsToMap converts variadic arguments to a map using sync.Pool to reduce GC pressure
// The returned map must be returned to the pool by the caller after use
func argsToMap(args []any) map[string]any {
	// Get map from pool
	fields := mapPool.Get().(map[string]any)

	// Clear any existing entries (maps from pool may contain stale data)
	for k := range fields {
		delete(fields, k)
	}

	// Populate with new data
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok && i+1 < len(args) {
			fields[key] = args[i+1]
		}
	}

	return fields
}
