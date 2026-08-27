package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
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

func TestGetApiPositions_Empty(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/positions", nil)
	w := httptest.NewRecorder()
	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []Position
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (an empty account must not look like a holding)", len(got))
	}
}

func TestGetApiPositions_ReturnsOpenAndPartialOnly(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	now := time.Now()
	for _, p := range []domain.Position{
		{Symbol: "XRP_JPY", Side: "BUY", Size: 18.5, RemainingSize: 18.5, EntryPrice: 219.0,
			Status: "OPEN", OrderID: "POS-OPEN", CreatedAt: now, UpdatedAt: now},
		{Symbol: "XLM_JPY", Side: "BUY", Size: 20, UsedSize: 5, RemainingSize: 15, EntryPrice: 28.6,
			Status: "PARTIAL", OrderID: "POS-PARTIAL", CreatedAt: now, UpdatedAt: now},
		{Symbol: "XRP_JPY", Side: "BUY", Size: 10, RemainingSize: 0, EntryPrice: 200.0,
			Status: "CLOSED", OrderID: "POS-CLOSED", CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.SavePosition(&p); err != nil {
			t.Fatalf("SavePosition(%s): %v", p.OrderID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/positions", nil)
	w := httptest.NewRecorder()
	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []Position
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (OPEN + PARTIAL, not CLOSED)", len(got))
	}
	for _, p := range got {
		if p.OrderId != nil && *p.OrderId == "POS-CLOSED" {
			t.Error("CLOSED position must not be reported as held")
		}
	}
}

// The caller subtracts remaining_size from the exchange balance to work out
// which units it is allowed to touch. A rounded size would silently mis-state
// that boundary, so the wire format must preserve the value exactly.
func TestGetApiPositions_SizePreservesPrecision(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	const size = 289.57764321
	now := time.Now()
	p := domain.Position{
		Symbol: "XLM_JPY", Side: "BUY", Size: size, RemainingSize: size, EntryPrice: 28.6,
		Status: "OPEN", OrderID: "POS-PRECISION", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.SavePosition(&p); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/positions", nil)
	w := httptest.NewRecorder()
	testHandler(server).ServeHTTP(w, req)

	var got []Position
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].RemainingSize == nil || *got[0].RemainingSize != size {
		t.Errorf("remaining_size = %v, want exactly %v", got[0].RemainingSize, size)
	}
}
