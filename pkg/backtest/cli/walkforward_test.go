package cli

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/pkg/backtest"
)

func TestResolveWalkForwardRange(t *testing.T) {
	t.Parallel()

	day := func(s string) time.Time {
		t.Helper()
		v, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return v
	}

	tests := []struct {
		name         string
		cfgFrom      string
		cfgTo        string
		fromFlag     string
		toFlag       string
		wantFrom     string
		wantTo       string
		wantErrorMsg bool
	}{
		{
			name:     "config supplies both ends",
			cfgFrom:  "2026-03-29",
			cfgTo:    "2026-08-26",
			wantFrom: "2026-03-29",
			wantTo:   "2026-08-26",
		},
		{
			name:     "flags win over config",
			cfgFrom:  "2026-03-29",
			cfgTo:    "2026-08-26",
			fromFlag: "2026-05-01",
			toFlag:   "2026-06-01",
			wantFrom: "2026-05-01",
			wantTo:   "2026-06-01",
		},
		{
			// Without a dataset we cannot probe, so an unset end must surface as
			// an error rather than silently becoming the zero time.
			name:         "unset end falls through to the dataset probe",
			cfgFrom:      "2026-03-29",
			wantErrorMsg: true,
		},
		{
			name:         "malformed flag is rejected",
			fromFlag:     "29-03-2026",
			wantErrorMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &backtest.Config{}
			cfg.Data.From = tt.cfgFrom
			cfg.Data.To = tt.cfgTo

			// combos is empty, so any path that needs the dataset errors out
			// instead of running a backtest.
			from, to, err := resolveWalkForwardRange(context.Background(), cfg, "scenario",
				tt.fromFlag, tt.toFlag, nil)
			if tt.wantErrorMsg {
				if err == nil {
					t.Fatalf("expected an error, got %s..%s", from, to)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !from.Equal(day(tt.wantFrom)) {
				t.Errorf("from = %s, want %s", from.Format("2006-01-02"), tt.wantFrom)
			}
			if !to.Equal(day(tt.wantTo)) {
				t.Errorf("to = %s, want %s", to.Format("2006-01-02"), tt.wantTo)
			}
		})
	}
}

func TestTruncateToDay(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 8, 26, 23, 59, 59, 999, time.UTC)
	got := truncateToDay(in)
	want := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("truncateToDay(%s) = %s, want %s", in, got, want)
	}
}

func TestWriteWalkForwardAggregate(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "aggregate.csv")
	if err := writeWalkForwardAggregate(path, 6, 7, 0, -288.3895); err != nil {
		t.Fatalf("writeWalkForwardAggregate: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := [][]string{
		{"windows", "trades", "win_rate", "net_pnl"},
		{"6", "7", "0.0000", "-288.3895"},
	}
	if len(records) != len(want) {
		t.Fatalf("got %d rows, want %d", len(records), len(want))
	}
	for i := range want {
		for j := range want[i] {
			if records[i][j] != want[i][j] {
				t.Errorf("row %d col %d = %q, want %q", i, j, records[i][j], want[i][j])
			}
		}
	}
}
