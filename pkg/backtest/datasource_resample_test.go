package backtest

import (
	"context"
	"io"
	"testing"
	"time"
)

// staticDS yields a pre-built slice of bars.
type staticDS struct {
	bars []Bar
	i    int
}

func (s *staticDS) Next(_ context.Context) (Bar, error) {
	if s.i >= len(s.bars) {
		return Bar{}, io.EOF
	}
	b := s.bars[s.i]
	s.i++
	return b, nil
}
func (s *staticDS) Close() error { return nil }

func mkBar(tsRFC3339 string, o, h, l, c, v float64) Bar {
	t, err := time.Parse(time.RFC3339, tsRFC3339)
	if err != nil {
		panic(err)
	}
	return Bar{
		Symbol: "XRP_JPY", Timestamp: t, Open: o, High: h, Low: l, Close: c, Volume: v,
	}
}

func TestResampling_1h(t *testing.T) {
	// Three 20-min bars in 00:00-00:60 (UTC), then one in 01:00-02:00.
	bars := []Bar{
		mkBar("2026-01-01T00:00:00Z", 100, 105, 99, 102, 1.0),
		mkBar("2026-01-01T00:20:00Z", 102, 110, 101, 108, 2.0),
		mkBar("2026-01-01T00:40:00Z", 108, 109, 95, 96, 3.0),
		mkBar("2026-01-01T01:10:00Z", 96, 97, 90, 91, 5.0),
	}
	ds, err := NewResamplingDatasource(&staticDS{bars: bars}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()

	ctx := context.Background()
	got := []Bar{}
	for {
		b, err := ds.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, b)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 aggregated bars, got %d: %+v", len(got), got)
	}
	// Bucket 0: 00:00 — Open 100 (first), High 110, Low 95, Close 96, Volume 6.
	if g := got[0]; g.Open != 100 || g.High != 110 || g.Low != 95 || g.Close != 96 || g.Volume != 6.0 {
		t.Errorf("bucket0 wrong: %+v", g)
	}
	if !got[0].Timestamp.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("bucket0 ts: %v", got[0].Timestamp)
	}
	// Bucket 1: 01:00 — single bar passes through.
	if g := got[1]; g.Open != 96 || g.High != 97 || g.Low != 90 || g.Close != 91 || g.Volume != 5.0 {
		t.Errorf("bucket1 wrong: %+v", g)
	}
	if !got[1].Timestamp.Equal(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)) {
		t.Errorf("bucket1 ts: %v", got[1].Timestamp)
	}
}

func TestResampling_PassThrough_WhenZeroPeriod(t *testing.T) {
	_, err := NewResamplingDatasource(&staticDS{}, 0)
	if err == nil {
		t.Fatal("expected error for zero period")
	}
}

func TestResampling_EmptySource(t *testing.T) {
	ds, err := NewResamplingDatasource(&staticDS{}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()
	_, err = ds.Next(context.Background())
	if err != io.EOF {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestParseBarPeriod(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"", 0, false},
		{"1h", time.Hour, false},
		{"4h", 4 * time.Hour, false},
		{"5m", 5 * time.Minute, false},
		{"20s", 20 * time.Second, false},
		{"-1h", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		got, err := ParseBarPeriod(c.in)
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("%q: got %v want %v", c.in, got, c.want)
		}
	}
}
