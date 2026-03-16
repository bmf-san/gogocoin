package balance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/logger"
)

const (
	// CacheDuration defines how long balance data is cached
	CacheDuration = 60 * time.Second
	// StaleCacheTTL is how long stale cache can be served as fallback when API fails
	StaleCacheTTL = 10 * time.Minute
)

// BalanceService manages balance retrieval, caching, and persistence
type BalanceService struct {
	client *bitflyer.Client
	logger logger.LoggerInterface
	db     domain.BalanceRepository

	cache struct {
		data      []domain.Balance
		timestamp time.Time
		mu        sync.RWMutex
	}

	// fetchMu serializes API fetches (singleflight: only one goroutine fetches at a time)
	fetchMu sync.Mutex
}

// NewBalanceService creates a new BalanceService
func NewBalanceService(client *bitflyer.Client, db domain.BalanceRepository, logger logger.LoggerInterface) *BalanceService {
	return &BalanceService{
		client: client,
		logger: logger,
		db:     db,
	}
}

// GetBalance retrieves balance information with caching
func (s *BalanceService) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	// Check cache first (atomically check both timestamp and data)
	s.cache.mu.RLock()
	cacheTimestamp := s.cache.timestamp
	cacheData := s.cache.data
	s.cache.mu.RUnlock()

	// Check if cache is still valid
	if time.Since(cacheTimestamp) < CacheDuration && len(cacheData) > 0 {
		// Return a copy to prevent external modification
		cachedBalances := make([]domain.Balance, len(cacheData))
		copy(cachedBalances, cacheData)
		cacheAge := time.Since(cacheTimestamp)

		s.logger.Trading().
			WithField("cache_age_sec", int(cacheAge.Seconds())).
			Debug("Returning cached balance data")
		return cachedBalances, nil
	}

	// Cache miss or expired - fetch from API
	// Serialize fetches: only one goroutine calls the API at a time.
	// Others wait on this mutex and then re-check the cache before fetching.
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	// Re-check cache after acquiring the lock: a concurrent goroutine may have
	// already fetched fresh data while we were waiting.
	s.cache.mu.RLock()
	cacheTimestamp = s.cache.timestamp
	cacheData = s.cache.data
	s.cache.mu.RUnlock()
	if time.Since(cacheTimestamp) < CacheDuration && len(cacheData) > 0 {
		cachedBalances := make([]domain.Balance, len(cacheData))
		copy(cachedBalances, cacheData)
		s.logger.Trading().Debug("Returning cached balance data (post-lock re-check)")
		return cachedBalances, nil
	}

	// Wait for rate limiter before making API request
	if s.client == nil {
		return nil, fmt.Errorf("bitflyer client not initialized")
	}
	if err := s.client.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	httpClient := s.client.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	// Call API with retry logic
	var resp *http.GetV1MeGetbalanceResponse

	retryErr := s.client.WithRetry(ctx, "GetBalance", func() error {
		start := time.Now()
		var err error
		resp, err = httpClient.GetV1MeGetbalanceWithResponse(ctx)
		duration := time.Since(start).Milliseconds()

		if err != nil {
			s.logger.LogAPICall("GET", "/v1/me/getbalance", duration, 0, err)
			return err
		}

		s.logger.LogAPICall("GET", "/v1/me/getbalance", duration, resp.HTTPResponse.StatusCode, nil)

		if resp.HTTPResponse.StatusCode != 200 {
			return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
		}

		if resp.JSON200 == nil {
			return fmt.Errorf("empty response body")
		}

		return nil
	})

	if retryErr != nil {
		// On API error, return stale cache if it's not too old (up to StaleCacheTTL)
		s.cache.mu.RLock()
		staleData := s.cache.data
		staleTimestamp := s.cache.timestamp
		s.cache.mu.RUnlock()
		if len(staleData) > 0 && time.Since(staleTimestamp) < StaleCacheTTL {
			s.logger.Trading().
				WithError(retryErr).
				WithField("stale_age_sec", int(time.Since(staleTimestamp).Seconds())).
				Warn("API fetch failed, returning stale cached balance")
			staleCopy := make([]domain.Balance, len(staleData))
			copy(staleCopy, staleData)
			return staleCopy, nil
		}
		return nil, fmt.Errorf("failed to get balance after retries: %w", retryErr)
	}

	var balances []domain.Balance
	for _, bal := range *resp.JSON200 {
		balance := domain.Balance{
			Currency:  safeString(bal.CurrencyCode),
			Amount:    float64(safeFloat32(bal.Amount)),
			Available: float64(safeFloat32(bal.Available)),
			Timestamp: time.Now(),
		}
		balances = append(balances, balance)

		// Debug log (use Debug level to avoid exposing sensitive balance info in production logs)
		s.logger.Trading().
			WithField("currency", balance.Currency).
			WithField("amount", balance.Amount).
			WithField("available", balance.Available).
			Debug("Balance retrieved from bitFlyer API")
	}

	s.logger.Trading().
		WithField("total_currencies", len(balances)).
		Info("All balances retrieved successfully")

	// Update cache (atomically update both timestamp and data).
	// Store a separate copy in the cache so that mutations to the returned
	// slice cannot corrupt the cached data.
	cacheCopy := make([]domain.Balance, len(balances))
	copy(cacheCopy, balances)
	s.cache.mu.Lock()
	s.cache.data = cacheCopy
	s.cache.timestamp = time.Now()
	s.cache.mu.Unlock()

	return balances, nil
}

// InvalidateCache invalidates the balance cache to force fresh data retrieval
func (s *BalanceService) InvalidateCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.timestamp = time.Time{} // Reset to zero value to force refresh
}

// UpdateBalanceToDB updates balance to database
func (s *BalanceService) UpdateBalanceToDB(ctx context.Context) {
	if s.db == nil {
		return
	}

	balances, err := s.GetBalance(ctx)
	if err != nil {
		s.logger.Trading().WithError(err).Error("Failed to get balance for database update")
		return
	}

	for _, bal := range balances {
		if err := s.db.SaveBalance(bal); err != nil {
			s.logger.Trading().WithError(err).
				WithField("currency", bal.Currency).
				Error("Failed to save balance to database")
		}
	}
}

// Helper functions
func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func safeFloat32(ptr *float32) float32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
