package persistence

import (
	"database/sql"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// AppStateRepository implements domain.AppStateRepository over *DB.
type AppStateRepository struct{ db *DB }

// NewAppStateRepository creates an AppStateRepository backed by db.
func NewAppStateRepository(db *DB) *AppStateRepository { return &AppStateRepository{db: db} }

// Compile-time check.
var _ domain.AppStateRepository = (*AppStateRepository)(nil)

// SaveAppState upserts a key-value state entry.
func (r *AppStateRepository) SaveAppState(key, value string) error {
	_, err := r.db.db.Exec(`
		INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value)
	return err
}

// GetAppState retrieves a state entry by key (returns "" if not found).
func (r *AppStateRepository) GetAppState(key string) (string, error) {
	var value string
	err := r.db.db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}
