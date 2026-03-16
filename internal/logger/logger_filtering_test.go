package logger

import (
	"testing"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// mockLogRepository is a test double for domain.LogRepository that counts SaveLog calls.
type mockLogRepository struct {
	savedEntries []*domain.LogEntry
}

func (m *mockLogRepository) SaveLog(entry *domain.LogEntry) error {
	m.savedEntries = append(m.savedEntries, entry)
	return nil
}

// newTestLogger creates a Logger backed by the provided mock repository.
func newTestLogger(mock domain.LogRepository) *Logger {
	cfg := &Config{
		Level:  "DEBUG",
		Format: "json",
		Output: "console",
	}
	l, err := New(cfg)
	if err != nil {
		panic("newTestLogger: " + err.Error())
	}
	l.SetDatabase(mock)
	return l
}

func TestSaveToDatabase_SkipsDebugLogs(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		shouldSave bool
	}{
		{name: "DEBUG logs should be skipped", level: "DEBUG", shouldSave: false},
		{name: "INFO logs should be saved", level: "INFO", shouldSave: true},
		{name: "WARN logs should be saved", level: "WARN", shouldSave: true},
		{name: "ERROR logs should be saved", level: "ERROR", shouldSave: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLogRepository{}
			l := newTestLogger(mock)

			l.saveToDatabase(tt.level, "system", "test message", nil)

			saved := len(mock.savedEntries) > 0
			if saved != tt.shouldSave {
				t.Errorf("level=%s: expected shouldSave=%v, got saved=%v",
					tt.level, tt.shouldSave, saved)
			}
		})
	}
}

func TestSaveToDatabase_SkipsDataCategory(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		shouldSave bool
	}{
		{name: "data category should be skipped", category: string(CategoryData), shouldSave: false},
		{name: "system category should be saved", category: "system", shouldSave: true},
		{name: "trading category should be saved", category: "trading", shouldSave: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLogRepository{}
			l := newTestLogger(mock)

			l.saveToDatabase("INFO", tt.category, "test message", nil)

			saved := len(mock.savedEntries) > 0
			if saved != tt.shouldSave {
				t.Errorf("category=%s: expected shouldSave=%v, got saved=%v",
					tt.category, tt.shouldSave, saved)
			}
		})
	}
}
