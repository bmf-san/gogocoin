package logger

import (
	"io"
	"log/slog"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// NopLogger is a no-op logger for use in unit tests.
// All log output is discarded; no database writes are performed.
// Satisfies LoggerInterface at compile time.
type NopLogger struct {
	el *ExtendedLogger
}

// Compile-time check: NopLogger must implement LoggerInterface.
var _ LoggerInterface = (*NopLogger)(nil)

// NewNopLogger creates a new NopLogger that discards all output.
func NewNopLogger() *NopLogger {
	el := &ExtendedLogger{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		parent:   nil,
		category: "",
	}
	return &NopLogger{el: el}
}

// --- Category loggers ---

func (n *NopLogger) System() *ExtendedLogger           { return n.el }
func (n *NopLogger) Trading() *ExtendedLogger          { return n.el }
func (n *NopLogger) API() *ExtendedLogger              { return n.el }
func (n *NopLogger) Strategy() *ExtendedLogger         { return n.el }
func (n *NopLogger) UI() *ExtendedLogger               { return n.el }
func (n *NopLogger) Data() *ExtendedLogger             { return n.el }
func (n *NopLogger) Category(_ string) *ExtendedLogger { return n.el }

// --- Field helpers ---

func (n *NopLogger) WithFields(_ map[string]any) *slog.Logger { return n.el.Logger }
func (n *NopLogger) WithField(_ string, _ any) *slog.Logger   { return n.el.Logger }
func (n *NopLogger) WithError(_ error) *slog.Logger           { return n.el.Logger }

// --- Specialised logging methods ---

func (n *NopLogger) LogTrade(_ string, _ string, _, _ float64, _ map[string]any) {
}
func (n *NopLogger) LogAPICall(_ string, _ string, _ int64, _ int, _ error) {}
func (n *NopLogger) LogStrategySignal(_ string, _ string, _ string, _ float64, _ map[string]any) {
}
func (n *NopLogger) LogError(_ string, _ string, _ error, _ map[string]any) {}
func (n *NopLogger) LogPerformance(_ string, _ int64, _ map[string]any)     {}
func (n *NopLogger) LogStartup(_ string, _ map[string]any)                  {}
func (n *NopLogger) LogShutdown(_ string)                                   {}

// --- Basic ---

func (n *NopLogger) Error(_ string) {}

// --- Database integration ---

func (n *NopLogger) SetDatabase(_ domain.LogRepository) {}

// --- Lifecycle ---

func (n *NopLogger) Flush()                  {}
func (n *NopLogger) Close() error            { return nil }
func (n *NopLogger) SetLevel(_ string) error { return nil }
func (n *NopLogger) GetLevel() string        { return "nop" }
