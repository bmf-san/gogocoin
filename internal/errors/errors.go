package errors

import "fmt"

// ErrorCode represents a typed error code
type ErrorCode string

// Common error codes
const (
	// Trading errors
	ErrInsufficientBalance ErrorCode = "INSUFFICIENT_BALANCE"
	ErrOrderValidation     ErrorCode = "ORDER_VALIDATION"
	ErrOrderExecution      ErrorCode = "ORDER_EXECUTION"
	ErrOrderNotFound       ErrorCode = "ORDER_NOT_FOUND"

	// Market data errors
	ErrMarketDataUnavailable ErrorCode = "MARKET_DATA_UNAVAILABLE"
	ErrWebSocketConnection   ErrorCode = "WEBSOCKET_CONNECTION"
	ErrAPIFailure            ErrorCode = "API_FAILURE"

	// Risk management errors
	ErrRiskLimitExceeded  ErrorCode = "RISK_LIMIT_EXCEEDED"
	ErrDailyLimitExceeded ErrorCode = "DAILY_LIMIT_EXCEEDED"

	// Strategy errors
	ErrStrategyNotFound   ErrorCode = "STRATEGY_NOT_FOUND"
	ErrInvalidSignal      ErrorCode = "INVALID_SIGNAL"
	ErrStrategyExecution  ErrorCode = "STRATEGY_EXECUTION"

	// Database errors
	ErrDatabaseConnection ErrorCode = "DATABASE_CONNECTION"
	ErrDatabaseQuery      ErrorCode = "DATABASE_QUERY"
	ErrRecordNotFound     ErrorCode = "RECORD_NOT_FOUND"

	// Configuration errors
	ErrInvalidConfig      ErrorCode = "INVALID_CONFIG"
	ErrMissingConfig      ErrorCode = "MISSING_CONFIG"

	// System errors
	ErrInternal           ErrorCode = "INTERNAL_ERROR"
	ErrTimeout            ErrorCode = "TIMEOUT"
)

// Error represents a structured error with context
type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new Error
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with a code and message
func Wrap(err error, code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// Is checks if an error has a specific code
func Is(err error, code ErrorCode) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
