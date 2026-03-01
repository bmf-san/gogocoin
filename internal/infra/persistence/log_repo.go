package persistence

import (
	"encoding/json"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// LogRepository implements domain.LogRepository over *DB.
type LogRepository struct{ db *DB }

// NewLogRepository creates a LogRepository backed by db.
func NewLogRepository(db *DB) *LogRepository { return &LogRepository{db: db} }

// Compile-time check.
var _ domain.LogRepository = (*LogRepository)(nil)

// SaveLog persists a log entry to the logs table.
func (r *LogRepository) SaveLog(entry *domain.LogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	fieldsJSON, err := json.Marshal(entry.Fields)
	if err != nil {
		fieldsJSON = []byte("{}")
	}
	_, err = r.db.db.Exec(
		`INSERT INTO logs (level, category, message, fields, timestamp) VALUES (?, ?, ?, ?, ?)`,
		entry.Level, entry.Category, entry.Message, string(fieldsJSON), entry.Timestamp,
	)
	return err
}

// GetRecentLogsWithFilters returns up to limit log entries filtered by level and
// category (empty string = no filter).
func (r *LogRepository) GetRecentLogsWithFilters(limit int, level, category string) ([]domain.LogEntry, error) {
	query := `SELECT id, level, category, message, fields, timestamp FROM logs`
	var args []interface{}
	where := ""
	if level != "" {
		where += " level = ?"
		args = append(args, level)
	}
	if category != "" {
		if where != "" {
			where += " AND"
		}
		where += " category = ?"
		args = append(args, category)
	}
	if where != "" {
		query += " WHERE" + where
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	logs := make([]domain.LogEntry, 0, 100)
	for rows.Next() {
		var l domain.LogEntry
		var id int
		var fieldsJSON string
		if err := rows.Scan(&id, &l.Level, &l.Category, &l.Message, &fieldsJSON, &l.Timestamp); err != nil {
			return nil, err
		}
		if fieldsJSON != "" {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
				l.Fields = make(map[string]interface{})
			} else {
				l.Fields = fields
			}
		} else {
			l.Fields = make(map[string]interface{})
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
