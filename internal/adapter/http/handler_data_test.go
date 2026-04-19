package api

import (
	"testing"
	"time"
)

func TestParseSince_Today(t *testing.T) {
	got, err := parseSince("today")
	if err != nil {
		t.Fatalf("parseSince(today) unexpected error: %v", err)
	}
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().In(jst)
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	if !got.Equal(want) {
		t.Errorf("parseSince(today) = %v, want JST start-of-day %v", got, want)
	}
}

func TestParseSince_RFC3339(t *testing.T) {
	in := "2026-04-19T09:15:00+09:00"
	got, err := parseSince(in)
	if err != nil {
		t.Fatalf("parseSince(%q) unexpected error: %v", in, err)
	}
	want, _ := time.Parse(time.RFC3339, in)
	if !got.Equal(want) {
		t.Errorf("parseSince(%q) = %v, want %v", in, got, want)
	}
}

func TestParseSince_Invalid(t *testing.T) {
	if _, err := parseSince("not-a-date"); err == nil {
		t.Error("parseSince(not-a-date) expected error, got nil")
	}
}
