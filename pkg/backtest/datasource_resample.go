package backtest

import (
	"context"
	"fmt"
	"io"
	"time"
)

// ResamplingDatasource wraps another Datasource and aggregates its bars
// into larger time-based bars (e.g. 20s ticks → 1h bars).
//
// Period boundaries are aligned to UTC (e.g. a 1h period emits bars at
// 00:00, 01:00, ... in UTC). The output bar timestamp is the START of the
// period (left-labeled). OHLCV are aggregated with the conventional rules:
//
//	Open   = Open of the first input bar in the window
//	High   = max High over the window
//	Low    = min Low over the window
//	Close  = Close of the last input bar in the window
//	Volume = sum of Volume over the window
//
// Trailing partial windows (i.e. the last bucket when EOF arrives) are
// emitted as a final bar.
type ResamplingDatasource struct {
	inner  Datasource
	period time.Duration

	// current bucket state
	have    bool
	bucket  time.Time // start (UTC) of the current bucket
	cur     Bar
	pending *Bar // bar held back because it belongs to the next bucket

	closed bool
	done   bool
}

// NewResamplingDatasource returns a Datasource that aggregates inner's bars
// into period-sized buckets. period must be > 0.
func NewResamplingDatasource(inner Datasource, period time.Duration) (*ResamplingDatasource, error) {
	if inner == nil {
		return nil, fmt.Errorf("resampling datasource: inner is required")
	}
	if period <= 0 {
		return nil, fmt.Errorf("resampling datasource: period must be > 0")
	}
	return &ResamplingDatasource{inner: inner, period: period}, nil
}

// bucketStart returns the UTC-aligned start of the bucket containing t.
func bucketStart(t time.Time, period time.Duration) time.Time {
	u := t.UTC()
	ns := u.UnixNano()
	bucketNs := period.Nanoseconds()
	startNs := ns - (ns % bucketNs)
	if ns < 0 && ns%bucketNs != 0 {
		startNs -= bucketNs
	}
	return time.Unix(0, startNs).UTC()
}

// Next returns the next aggregated Bar, or io.EOF when exhausted.
func (r *ResamplingDatasource) Next(ctx context.Context) (Bar, error) {
	if r.closed {
		return Bar{}, io.EOF
	}
	for {
		if err := ctx.Err(); err != nil {
			return Bar{}, err
		}
		// Pull next input bar (either pending from the prior call, or new).
		var (
			b   Bar
			err error
		)
		if r.pending != nil {
			b = *r.pending
			r.pending = nil
		} else if r.done {
			// EOF already seen; flush whatever bucket we have.
			if r.have {
				out := r.cur
				r.have = false
				return out, nil
			}
			return Bar{}, io.EOF
		} else {
			b, err = r.inner.Next(ctx)
			if err != nil {
				if err == io.EOF {
					r.done = true
					if r.have {
						out := r.cur
						r.have = false
						return out, nil
					}
					return Bar{}, io.EOF
				}
				return Bar{}, err
			}
		}

		bb := bucketStart(b.Timestamp, r.period)

		if !r.have {
			// Start a new bucket from this bar.
			r.bucket = bb
			r.cur = Bar{
				Symbol:    b.Symbol,
				Timestamp: bb,
				Open:      b.Open,
				High:      b.High,
				Low:       b.Low,
				Close:     b.Close,
				Volume:    b.Volume,
			}
			r.have = true
			continue
		}

		if bb.Equal(r.bucket) {
			// Same bucket → fold in.
			if b.High > r.cur.High {
				r.cur.High = b.High
			}
			if b.Low < r.cur.Low {
				r.cur.Low = b.Low
			}
			r.cur.Close = b.Close
			r.cur.Volume += b.Volume
			continue
		}

		// New bucket starts → emit current, stash this bar as pending.
		out := r.cur
		r.have = false
		bCopy := b
		r.pending = &bCopy
		return out, nil
	}
}

// Close closes the inner datasource. Idempotent.
func (r *ResamplingDatasource) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.inner != nil {
		return r.inner.Close()
	}
	return nil
}

// ParseBarPeriod parses strings like "20s", "1m", "5m", "1h", "4h".
// An empty string returns 0 with no error (caller decides default).
func ParseBarPeriod(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid bar_period %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("bar_period must be > 0, got %s", s)
	}
	return d, nil
}
