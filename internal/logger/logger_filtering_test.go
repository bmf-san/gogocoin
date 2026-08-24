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

// TestSaveToDatabase_BuffersUntilDatabaseIsSet covers the startup window. The
// database is attached after the logger is constructed, so migration results and
// other boot-time entries reach saveToDatabase before there is anywhere to store
// them. They must be held and replayed rather than discarded.
func TestSaveToDatabase_BuffersUntilDatabaseIsSet(t *testing.T) {
	l, err := New(&Config{Level: "DEBUG", Format: "json", Output: "console"})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	l.saveToDatabase("INFO", "system", "Migration executed", nil)
	l.saveToDatabase("DEBUG", "system", "noisy debug", nil)
	l.saveToDatabase("INFO", string(CategoryData), "ticker", nil)

	mock := &mockLogRepository{}
	l.SetDatabase(mock)

	if len(mock.savedEntries) != 1 {
		t.Fatalf("flushed %d entries; want 1", len(mock.savedEntries))
	}
	if got := mock.savedEntries[0].Message; got != "Migration executed" {
		t.Errorf("flushed message = %q; want %q", got, "Migration executed")
	}

	// The buffer must not be replayed a second time.
	l.SetDatabase(mock)
	if len(mock.savedEntries) != 1 {
		t.Errorf("re-flushed on second SetDatabase: %d entries", len(mock.savedEntries))
	}
}

// TestSaveToDatabase_BufferIsBounded guards against unbounded growth when a
// database is never attached.
func TestSaveToDatabase_BufferIsBounded(t *testing.T) {
	l, err := New(&Config{Level: "DEBUG", Format: "json", Output: "console"})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	for i := 0; i < maxPendingLogEntries+50; i++ {
		l.saveToDatabase("INFO", "system", "entry", nil)
	}

	if len(l.pending) != maxPendingLogEntries {
		t.Errorf("buffered %d entries; want the cap of %d", len(l.pending), maxPendingLogEntries)
	}
}
