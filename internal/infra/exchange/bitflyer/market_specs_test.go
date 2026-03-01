package bitflyer

import (
	"context"
	"testing"
	"time"
)

func TestMarketSpecificationService_GetMinimumOrderSize(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		want    float64
		wantErr bool
	}{
		{
			name:    "BTC_JPY minimum order size",
			symbol:  "BTC_JPY",
			want:    0.001,
			wantErr: false,
		},
		{
			name:    "ETH_JPY minimum order size",
			symbol:  "ETH_JPY",
			want:    0.01,
			wantErr: false,
		},
		{
			name:    "XRP_JPY minimum order size",
			symbol:  "XRP_JPY",
			want:    1.0,
			wantErr: false,
		},
		{
			name:    "Unknown symbol",
			symbol:  "UNKNOWN_SYMBOL",
			want:    0.0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service without actual client (will use fallback)
			s := NewMarketSpecificationService(nil)

			got, err := s.GetMinimumOrderSize(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMinimumOrderSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetMinimumOrderSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarketSpecificationService_GetSpecification(t *testing.T) {
	s := NewMarketSpecificationService(nil)

	// Populate fallback specs
	s.populateFallbackSpecs()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{
			name:    "Get BTC_JPY specification",
			symbol:  "BTC_JPY",
			wantErr: false,
		},
		{
			name:    "Get ETH_JPY specification",
			symbol:  "ETH_JPY",
			wantErr: false,
		},
		{
			name:    "Unknown symbol",
			symbol:  "UNKNOWN",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.GetSpecification(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpecification() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.ProductCode != tt.symbol {
					t.Errorf("GetSpecification() ProductCode = %v, want %v", got.ProductCode, tt.symbol)
				}
				if got.SizeMin == 0 {
					t.Errorf("GetSpecification() SizeMin should not be 0")
				}
			}
		})
	}
}

func TestMarketSpecificationService_CacheTTL(t *testing.T) {
	s := NewMarketSpecificationService(nil)
	s.ttl = 100 * time.Millisecond // Short TTL for testing

	// Populate cache
	s.populateFallbackSpecs()

	// Get spec - should be cached
	spec1, err := s.GetSpecification("BTC_JPY")
	if err != nil {
		t.Fatalf("GetSpecification() error = %v", err)
	}

	fetchTime1 := spec1.FetchedAt

	// Get again immediately - should use cache
	spec2, err := s.GetSpecification("BTC_JPY")
	if err != nil {
		t.Fatalf("GetSpecification() error = %v", err)
	}

	if spec2.FetchedAt != fetchTime1 {
		t.Errorf("Cache should be used, but got different fetch time")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Get again - should refresh (but without client, will use fallback)
	spec3, err := s.GetSpecification("BTC_JPY")
	if err != nil {
		t.Fatalf("GetSpecification() error = %v", err)
	}

	// Fetch time should be different (refreshed)
	if spec3.FetchedAt == fetchTime1 {
		t.Errorf("Cache should be refreshed after TTL, but got same fetch time")
	}
}

func TestMarketSpecificationService_RefreshCache(t *testing.T) {
	s := NewMarketSpecificationService(nil)

	// Refresh cache (will use fallback without real client)
	err := s.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("RefreshCache() error = %v", err)
	}

	// Verify cache was populated
	if len(s.cache) == 0 {
		t.Errorf("RefreshCache() should populate cache, but cache is empty")
	}

	// Verify known symbols exist
	expectedSymbols := []string{"BTC_JPY", "ETH_JPY", "XRP_JPY"}
	for _, symbol := range expectedSymbols {
		if _, exists := s.cache[symbol]; !exists {
			t.Errorf("RefreshCache() should populate %s, but it's missing", symbol)
		}
	}
}

func TestMarketSpecificationService_ConcurrentAccess(t *testing.T) {
	s := NewMarketSpecificationService(nil)
	s.populateFallbackSpecs()

	// Test concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := s.GetSpecification("BTC_JPY")
			if err != nil {
				t.Errorf("GetSpecification() error = %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
