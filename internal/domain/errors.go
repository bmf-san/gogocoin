package domain

import "fmt"

// Error types for better error handling and classification

// ErrType represents error category
type ErrType string

const (
	// ErrTypeValidation indicates validation errors
	ErrTypeValidation ErrType = "validation"
	// ErrTypeNotFound indicates resource not found errors
	ErrTypeNotFound ErrType = "not_found"
	// ErrTypeConflict indicates conflict errors (e.g., duplicate resource)
	ErrTypeConflict ErrType = "conflict"
	// ErrTypeRateLimit indicates rate limit exceeded
	ErrTypeRateLimit ErrType = "rate_limit"
	// ErrTypeNetwork indicates network-related errors
	ErrTypeNetwork ErrType = "network"
	// ErrTypeDatabase indicates database errors
	ErrTypeDatabase ErrType = "database"
	// ErrTypeExternal indicates external service errors
	ErrTypeExternal ErrType = "external"
	// ErrTypeInternal indicates internal server errors
	ErrTypeInternal ErrType = "internal"
)

// Error is a custom error type with additional context
type Error struct {
	Type    ErrType // Error category
	Message string  // Human-readable error message
	Err     error   // Underlying error (optional)
	Context map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new Error
func NewError(errType ErrType, message string, err error) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Err:     err,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	e.Context[key] = value
	return e
}

// Common errors
var (
	// ErrInsufficientBalance indicates insufficient balance
	ErrInsufficientBalance = NewError(ErrTypeValidation, "insufficient balance", nil)

	// ErrInvalidOrderSize indicates invalid order size
	ErrInvalidOrderSize = NewError(ErrTypeValidation, "invalid order size", nil)

	// ErrOrderNotFound indicates order not found
	ErrOrderNotFound = NewError(ErrTypeNotFound, "order not found", nil)

	// ErrRateLimitExceeded indicates rate limit exceeded
	ErrRateLimitExceeded = NewError(ErrTypeRateLimit, "rate limit exceeded", nil)

	// ErrNetworkUnavailable indicates network unavailability
	ErrNetworkUnavailable = NewError(ErrTypeNetwork, "network unavailable", nil)
)
