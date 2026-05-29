package backtest

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite driver
)

// SQLiteDatasource reads Bars from the gogocoin `market_data` table.
//
// Schema (see gogocoin internal/infra/persistence/schema/001_initial.sql):
//
//	market_data(symbol TEXT, timestamp DATETIME, open REAL, high REAL,
//	            low REAL, close REAL, volume REAL, ...)
//
// SQLite stores timestamps as RFC3339-ish strings with a numeric offset
// (e.g. `2026-04-04 00:00:10.743+09:00`). We try a small list of layouts
// to be tolerant of historical variation.
type SQLiteDatasource struct {
	db     *sql.DB
	rows   *sql.Rows
	closed bool
}

// SQLiteDatasourceOptions configures the query bounds.
type SQLiteDatasourceOptions struct {
	Path   string
	Symbol string
	From   time.Time // inclusive; zero = no lower bound
	To     time.Time // inclusive; zero = no upper bound
}

// NewSQLiteDatasource opens path and prepares an iterator over bars matching
// opts. Caller must Close() the returned datasource.
func NewSQLiteDatasource(opts SQLiteDatasourceOptions) (*SQLiteDatasource, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("sqlite datasource: path is required")
	}
	if opts.Symbol == "" {
		return nil, fmt.Errorf("sqlite datasource: symbol is required")
	}
	// Open read-only to avoid contention with a running gogocoin process.
	// `immutable=1` skips WAL journal setup which would otherwise require
	// write access to the DB file/directory.
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", opts.Path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %q: %w", opts.Path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite ping %q: %w", opts.Path, err)
	}

	q := `SELECT symbol, timestamp, open, high, low, close, volume
	      FROM market_data
	      WHERE symbol = ?`
	args := []interface{}{opts.Symbol}
	if !opts.From.IsZero() {
		q += " AND timestamp >= ?"
		args = append(args, opts.From.Format(time.RFC3339))
	}
	if !opts.To.IsZero() {
		q += " AND timestamp <= ?"
		args = append(args, opts.To.Format(time.RFC3339))
	}
	q += " ORDER BY timestamp ASC"

	rows, err := db.Query(q, args...)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite query: %w", err)
	}
	return &SQLiteDatasource{db: db, rows: rows}, nil
}

// Next implements Datasource.
func (s *SQLiteDatasource) Next(ctx context.Context) (Bar, error) {
	if s.closed {
		return Bar{}, io.EOF
	}
	if err := ctx.Err(); err != nil {
		return Bar{}, err
	}
	if !s.rows.Next() {
		if err := s.rows.Err(); err != nil {
			return Bar{}, err
		}
		return Bar{}, io.EOF
	}
	var (
		symbol        string
		tsStr         string
		o, h, l, c, v float64
	)
	if err := s.rows.Scan(&symbol, &tsStr, &o, &h, &l, &c, &v); err != nil {
		return Bar{}, fmt.Errorf("sqlite scan: %w", err)
	}
	t, err := parseDBTimestamp(tsStr)
	if err != nil {
		return Bar{}, fmt.Errorf("sqlite parse timestamp %q: %w", tsStr, err)
	}
	return Bar{
		Symbol:    symbol,
		Timestamp: t,
		Open:      o,
		High:      h,
		Low:       l,
		Close:     c,
		Volume:    v,
	}, nil
}

// Close implements Datasource.
func (s *SQLiteDatasource) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var firstErr error
	if s.rows != nil {
		if err := s.rows.Close(); err != nil {
			firstErr = err
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// parseDBTimestamp parses the timestamp formats observed in gogocoin DBs.
// Order matters: try the most specific first.
func parseDBTimestamp(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp layout")
}
