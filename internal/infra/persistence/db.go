package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

// DB is the SQLite-backed persistence layer.
// It implements domain.DatabaseLifecycle and domain.TransactionManager.
type DB struct {
	db     *sql.DB
	logger logger.LoggerInterface
}

// Compile-time checks.
var _ domain.DatabaseLifecycle = (*DB)(nil)
var _ domain.TransactionManager = (*DB)(nil)

// NewDB opens and migrates a SQLite database at dbPath.
// Uses WAL+busy_timeout for file-based paths; single-connection for :memory:.
func NewDB(dbPath string, log logger.LoggerInterface) (*DB, error) {
	if log == nil {
		log = logger.NewNopLogger()
	}
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	// Close sqlDB if any subsequent initialization step fails to prevent FD leaks.
	var initOK bool
	defer func() {
		if !initOK {
			sqlDB.Close() //nolint:errcheck
		}
	}()

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	isMemory := dbPath == ":memory:" || strings.Contains(dbPath, "mode=memory")
	if !isMemory {
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
		}
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
	} else {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	sqlDB.SetConnMaxLifetime(0)

	d := &DB{db: sqlDB, logger: log}
	if _, err := sqlDB.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		log.System().WithError(err).Warn("Failed to enable auto_vacuum (non-critical)")
	}
	if err := d.runMigrations(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	initOK = true
	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error { return d.db.Close() }

// Ping checks whether the database connection is alive.
func (d *DB) Ping() error {
	if d.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	return d.db.Ping()
}

// BeginTx starts a new database transaction.
func (d *DB) BeginTx() (domain.Transaction, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &DBTx{tx: tx, logger: d.logger}, nil
}
