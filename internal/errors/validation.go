package errors

import "fmt"

// Validation error constructors

// NewInvalidSignalError creates an invalid signal error
func NewInvalidSignalError(reason string) *Error {
	return New(ErrInvalidSignal, fmt.Sprintf("invalid signal: %s", reason))
}

// NewInvalidConfigError creates an invalid configuration error
func NewInvalidConfigError(field, reason string) *Error {
	return New(ErrInvalidConfig, fmt.Sprintf("invalid config field %s: %s", field, reason)).
		WithContext("field", field)
}

// NewMissingConfigError creates a missing configuration error
func NewMissingConfigError(field string) *Error {
	return New(ErrMissingConfig, fmt.Sprintf("missing required config field: %s", field)).
		WithContext("field", field)
}
