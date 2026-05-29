package backtest

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// CSVDatasource reads Bars from a CSV file with the header:
//
//	timestamp,symbol,open,high,low,close,volume
//
// Records must be sorted by timestamp ASC. timestamp is parsed using the
// same set of layouts as parseDBTimestamp.
type CSVDatasource struct {
	opts   CSVDatasourceOptions
	f      *os.File
	r      *csv.Reader
	closed bool
}

// CSVDatasourceOptions configures the CSV reader.
type CSVDatasourceOptions struct {
	Path   string
	Symbol string    // when non-empty, rows with a different symbol are skipped
	From   time.Time // inclusive
	To     time.Time // inclusive
}

// NewCSVDatasource opens path and reads/validates the header.
func NewCSVDatasource(opts CSVDatasourceOptions) (*CSVDatasource, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("csv datasource: path is required")
	}
	f, err := os.Open(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("csv open %q: %w", opts.Path, err)
	}
	ds := &CSVDatasource{opts: opts, f: f}
	if err := ds.initReader(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return ds, nil
}

func (c *CSVDatasource) initReader() error {
	c.r = csv.NewReader(c.f)
	c.r.FieldsPerRecord = 7
	c.r.TrimLeadingSpace = true
	header, err := c.r.Read()
	if err != nil {
		return fmt.Errorf("csv header: %w", err)
	}
	expected := []string{"timestamp", "symbol", "open", "high", "low", "close", "volume"}
	if len(header) != len(expected) {
		return fmt.Errorf("csv header: expected %v got %v", expected, header)
	}
	for i, want := range expected {
		if header[i] != want {
			return fmt.Errorf("csv header[%d]: expected %q got %q", i, want, header[i])
		}
	}
	return nil
}

// Reopen rewinds the underlying file to the first data row.
func (c *CSVDatasource) Reopen() error {
	if _, err := c.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.closed = false
	return c.initReader()
}

// Next implements Datasource.
func (c *CSVDatasource) Next(ctx context.Context) (Bar, error) {
	if c.closed {
		return Bar{}, io.EOF
	}
	if err := ctx.Err(); err != nil {
		return Bar{}, err
	}
	for {
		rec, err := c.r.Read()
		if err == io.EOF {
			return Bar{}, io.EOF
		}
		if err != nil {
			return Bar{}, fmt.Errorf("csv read: %w", err)
		}
		bar, err := parseCSVRecord(rec)
		if err != nil {
			return Bar{}, err
		}
		if c.opts.Symbol != "" && bar.Symbol != c.opts.Symbol {
			continue
		}
		if !c.opts.From.IsZero() && bar.Timestamp.Before(c.opts.From) {
			continue
		}
		if !c.opts.To.IsZero() && bar.Timestamp.After(c.opts.To) {
			return Bar{}, io.EOF
		}
		return bar, nil
	}
}

// Close implements Datasource.
func (c *CSVDatasource) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if c.f != nil {
		return c.f.Close()
	}
	return nil
}

func parseCSVRecord(rec []string) (Bar, error) {
	if len(rec) != 7 {
		return Bar{}, fmt.Errorf("csv record: expected 7 fields got %d", len(rec))
	}
	t, err := parseDBTimestamp(rec[0])
	if err != nil {
		return Bar{}, fmt.Errorf("csv timestamp %q: %w", rec[0], err)
	}
	o, err := strconv.ParseFloat(rec[2], 64)
	if err != nil {
		return Bar{}, fmt.Errorf("csv open: %w", err)
	}
	h, err := strconv.ParseFloat(rec[3], 64)
	if err != nil {
		return Bar{}, fmt.Errorf("csv high: %w", err)
	}
	l, err := strconv.ParseFloat(rec[4], 64)
	if err != nil {
		return Bar{}, fmt.Errorf("csv low: %w", err)
	}
	cl, err := strconv.ParseFloat(rec[5], 64)
	if err != nil {
		return Bar{}, fmt.Errorf("csv close: %w", err)
	}
	v, err := strconv.ParseFloat(rec[6], 64)
	if err != nil {
		return Bar{}, fmt.Errorf("csv volume: %w", err)
	}
	return Bar{
		Symbol:    rec[1],
		Timestamp: t,
		Open:      o,
		High:      h,
		Low:       l,
		Close:     cl,
		Volume:    v,
	}, nil
}
