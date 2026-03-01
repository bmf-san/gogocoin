package domain

import "time"

// LogEntry represents a log entry
type LogEntry struct {
	Level     string                 `json:"level"`
	Category  string                 `json:"category"`
	Message   string                 `json:"message"`
	Fields    map[string]any `json:"fields"`
	Timestamp time.Time              `json:"timestamp"`
}
