package utils

import (
	"testing"
	"time"
)

func TestLoadJST_Succeeds(t *testing.T) {
	loc, err := LoadJST()
	if err != nil {
		t.Fatalf("LoadJST error: %v", err)
	}
	if loc == nil {
		t.Fatal("LoadJST returned nil location")
	}
	// Asia/Tokyo offset is always +09:00, regardless of season.
	_, offset := time.Now().In(loc).Zone()
	if offset != 9*3600 {
		t.Errorf("expected offset 9h, got %ds", offset)
	}
}

func TestNowInJST_HasJSTOffset(t *testing.T) {
	now := NowInJST()
	_, offset := now.Zone()
	if offset != 9*3600 && offset != 0 { // 0 = UTC fallback
		t.Errorf("expected +9h or UTC fallback, got %ds", offset)
	}
}

func TestTodayInJST_FormatYYYYMMDD(t *testing.T) {
	got := TodayInJST()
	if _, err := time.Parse("2006-01-02", got); err != nil {
		t.Errorf("TodayInJST format invalid %q: %v", got, err)
	}
}

func TestToJST_ConvertsUTCToJST(t *testing.T) {
	utc := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	jst := ToJST(utc)
	if !jst.Equal(utc) {
		t.Error("ToJST must preserve the instant (same moment)")
	}
	_, offset := jst.Zone()
	if offset != 9*3600 && offset != 0 {
		t.Errorf("ToJST zone offset=%ds, want 9h or UTC fallback", offset)
	}
	// Component check: 00:00 UTC == 09:00 JST same date
	if offset == 9*3600 {
		if h, m, s := jst.Clock(); h != 9 || m != 0 || s != 0 {
			t.Errorf("expected 09:00:00 JST, got %02d:%02d:%02d", h, m, s)
		}
	}
}

func TestToJST_IdempotentOnJSTInput(t *testing.T) {
	loc, err := LoadJST()
	if err != nil {
		t.Skip("JST unavailable on this system")
	}
	orig := time.Date(2026, 4, 18, 12, 34, 56, 0, loc)
	got := ToJST(orig)
	if !got.Equal(orig) {
		t.Error("ToJST on JST input must return equal instant")
	}
}
