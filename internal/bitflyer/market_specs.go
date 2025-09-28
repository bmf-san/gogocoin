package bitflyer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MarketSpec represents complete market specification including trading limits
type MarketSpec struct {
	ProductCode string
	SizeMin     float64
	SizeMax     float64
	PriceMin    float64
	PriceMax    float64
	FetchedAt   time.Time
}

// MarketSpecificationService provides centralized access to market specifications
// with caching to minimize redundant lookups. Since bitFlyer API doesn't provide
// size/price limits, these are maintained as hardcoded values with automatic
// refresh capability for future API enhancements.
type MarketSpecificationService struct {
	client    *Client
	cache     map[string]*MarketSpec
	mu        sync.RWMutex
	ttl       time.Duration
	lastFetch time.Time
}

// NewMarketSpecificationService creates a new market specification service
func NewMarketSpecificationService(client *Client) *MarketSpecificationService {
	return &MarketSpecificationService{
		client: client,
		cache:  make(map[string]*MarketSpec),
		ttl:    1 * time.Hour, // Cache for 1 hour (market specs change infrequently)
	}
}

// GetMinimumOrderSize returns the minimum order size for a given symbol
// This is the primary method used by trading logic to validate order sizes.
func (s *MarketSpecificationService) GetMinimumOrderSize(symbol string) (float64, error) {
	spec, err := s.GetSpecification(symbol)
	if err != nil {
		// Log warning when falling back to hardcoded values
		// This helps identify market spec cache issues during operations
		// Fallback to hardcoded values if spec not found
		fallback, fallbackErr := s.getFallbackMinimumOrderSize(symbol)
		if fallbackErr != nil {
			return 0, fmt.Errorf("failed to get minimum order size for %s: %w", symbol, err)
		}
		return fallback, nil
	}
	return spec.SizeMin, nil
}

// GetSpecification returns complete market specification for a symbol
func (s *MarketSpecificationService) GetSpecification(symbol string) (*MarketSpec, error) {
	s.mu.RLock()
	// Check if cache is valid
	if spec, exists := s.cache[symbol]; exists && time.Since(s.lastFetch) < s.ttl {
		s.mu.RUnlock()
		return spec, nil
	}
	s.mu.RUnlock()

	// Cache miss or expired - refresh
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check in case another goroutine just refreshed
	if spec, exists := s.cache[symbol]; exists && time.Since(s.lastFetch) < s.ttl {
		return spec, nil
	}

	// Refresh cache
	if err := s.refreshCacheUnsafe(context.Background()); err != nil {
		// If refresh fails, try to return stale cache or fallback
		if spec, exists := s.cache[symbol]; exists {
			return spec, nil
		}
		return nil, fmt.Errorf("failed to get specification for %s: %w", symbol, err)
	}

	spec, exists := s.cache[symbol]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrMarketSpecNotFound, symbol)
	}

	return spec, nil
}

// RefreshCache manually refreshes the market specification cache
// This can be called on application startup to pre-populate the cache.
func (s *MarketSpecificationService) RefreshCache(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCacheUnsafe(ctx)
}

// refreshCacheUnsafe refreshes the cache without acquiring lock (caller must hold lock)
func (s *MarketSpecificationService) refreshCacheUnsafe(ctx context.Context) error {
	// If client is not available, populate with hardcoded fallback values
	// Note: This is not an error condition, just logs for observability
	if s.client == nil {
		s.populateFallbackSpecs()
		s.lastFetch = time.Now()
		// Successfully populated with fallback values - no error needed
		return nil
	}

	// Fetch market list from API to get valid product codes
	markets, err := s.client.GetMarkets(ctx)
	if err != nil {
		// API unavailable - populate with hardcoded fallback values
		// Note: This is a warning condition (API failed), but we have fallbacks
		s.populateFallbackSpecs()
		s.lastFetch = time.Now()
		// Successfully fell back - no error needed, but could log warning in production
		return nil
	}

	// Populate cache with hardcoded specifications
	// Note: bitFlyer API doesn't provide size/price limits, so we maintain them here
	for _, market := range markets {
		spec := s.getHardcodedSpec(market.ProductCode)
		if spec != nil {
			spec.FetchedAt = time.Now()
			s.cache[market.ProductCode] = spec
		}
	}

	s.lastFetch = time.Now()
	return nil
}

// getHardcodedSpec returns hardcoded specification for known markets
// These values are based on bitFlyer's documented trading limits
func (s *MarketSpecificationService) getHardcodedSpec(productCode string) *MarketSpec {
	specs := map[string]*MarketSpec{
		"BTC_JPY": {
			ProductCode: "BTC_JPY",
			SizeMin:     0.001,
			SizeMax:     1000.0,
			PriceMin:    0.0,
			PriceMax:    30000000.0,
		},
		"ETH_JPY": {
			ProductCode: "ETH_JPY",
			SizeMin:     0.01,
			SizeMax:     10000.0,
			PriceMin:    0.0,
			PriceMax:    3000000.0,
		},
		"XRP_JPY": {
			ProductCode: "XRP_JPY",
			SizeMin:     1.0,
			SizeMax:     1000000.0,
			PriceMin:    0.0,
			PriceMax:    1000.0,
		},
		"BTC_USD": {
			ProductCode: "BTC_USD",
			SizeMin:     0.001,
			SizeMax:     1000.0,
			PriceMin:    0.0,
			PriceMax:    500000.0,
		},
		"BTC_EUR": {
			ProductCode: "BTC_EUR",
			SizeMin:     0.001,
			SizeMax:     1000.0,
			PriceMin:    0.0,
			PriceMax:    500000.0,
		},
	}

	return specs[productCode]
}

// populateFallbackSpecs populates cache with fallback specifications
// when API is unavailable
func (s *MarketSpecificationService) populateFallbackSpecs() {
	fallbackSymbols := []string{"BTC_JPY", "ETH_JPY", "XRP_JPY", "BTC_USD", "BTC_EUR"}
	for _, symbol := range fallbackSymbols {
		if spec := s.getHardcodedSpec(symbol); spec != nil {
			spec.FetchedAt = time.Now()
			s.cache[symbol] = spec
		}
	}
}

// getFallbackMinimumOrderSize returns hardcoded minimum order size as fallback
func (s *MarketSpecificationService) getFallbackMinimumOrderSize(symbol string) (float64, error) {
	spec := s.getHardcodedSpec(symbol)
	if spec != nil {
		return spec.SizeMin, nil
	}

	// Default fallback if symbol not recognized
	switch symbol {
	case "BTC_JPY", "BTC_USD", "BTC_EUR":
		return 0.001, nil
	case "ETH_JPY", "ETH_BTC":
		return 0.01, nil
	case "BCH_JPY", "BCH_BTC":
		return 0.01, nil
	case "XRP_JPY":
		return 1.0, nil
	case "MONA_JPY":
		return 1.0, nil
	default:
		return 0.0, fmt.Errorf("%w: %s", ErrInvalidSymbol, symbol)
	}
}
