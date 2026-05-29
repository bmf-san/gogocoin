package backtest

import (
	"context"
	"io"
)

// Datasource yields Bars in strict chronological order. Implementations must
// return io.EOF (and only io.EOF) when no more data is available.
type Datasource interface {
	// Next returns the next Bar. Returns (Bar{}, io.EOF) when exhausted.
	Next(ctx context.Context) (Bar, error)
	// Close releases any underlying resources. Idempotent.
	Close() error
}

// IsEOF reports whether err signals end of data. Helper for callers.
func IsEOF(err error) bool { return err == io.EOF }
