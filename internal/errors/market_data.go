package errors

import "fmt"

// Market data error constructors

// NewMarketDataUnavailableError creates a market data unavailable error
func NewMarketDataUnavailableError(symbol string, err error) *Error {
	return Wrap(err, ErrMarketDataUnavailable, fmt.Sprintf("market data unavailable for %s", symbol)).
		WithContext("symbol", symbol)
}

// NewWebSocketConnectionError creates a WebSocket connection error
func NewWebSocketConnectionError(err error) *Error {
	return Wrap(err, ErrWebSocketConnection, "WebSocket connection failed")
}

// NewAPIFailureError creates an API failure error
func NewAPIFailureError(endpoint string, statusCode int, err error) *Error {
	return Wrap(err, ErrAPIFailure, fmt.Sprintf("API call failed: %s (status %d)", endpoint, statusCode)).
		WithContext("endpoint", endpoint).
		WithContext("status_code", statusCode)
}
