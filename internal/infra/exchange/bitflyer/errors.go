package bitflyer

import (
	"errors"
)

// Sentinel errors for bitFlyer API operations
var (
	// ErrMarketSpecNotFound is returned when a market specification cannot be found
	ErrMarketSpecNotFound = errors.New("market specification not found")

	// ErrAPIRateLimitExceeded is returned when the API rate limit is exceeded
	ErrAPIRateLimitExceeded = errors.New("API rate limit exceeded")

	// ErrAPIUnavailable is returned when the bitFlyer API is temporarily unavailable
	ErrAPIUnavailable = errors.New("bitFlyer API unavailable")

	// ErrInvalidSymbol is returned when an invalid or unsupported symbol is provided
	ErrInvalidSymbol = errors.New("invalid or unsupported symbol")

	// ErrClientNotConfigured is returned when operations require a client but none is configured
	ErrClientNotConfigured = errors.New("bitFlyer client not configured")
)

// IsRateLimitError checks if the error is a rate limit error
// It checks both the sentinel error and APIError with rate limit code
func IsRateLimitError(err error) bool {
	if errors.Is(err, ErrAPIRateLimitExceeded) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "RATE_LIMIT_EXCEEDED" || apiErr.StatusCode == 429
	}
	return false
}

// IsMarketSpecNotFoundError checks if the error is a market spec not found error
func IsMarketSpecNotFoundError(err error) bool {
	return errors.Is(err, ErrMarketSpecNotFound)
}

// IsAPIUnavailableError checks if the error is an API unavailable error
func IsAPIUnavailableError(err error) bool {
	if errors.Is(err, ErrAPIUnavailable) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// Consider 5xx status codes as API unavailable
		return apiErr.StatusCode >= 500 && apiErr.StatusCode < 600
	}
	return false
}
