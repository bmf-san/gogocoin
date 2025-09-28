package app

import "fmt"

// AppError represents an application-level error
type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Error constructors

// ErrServiceNotInitialized creates an error for uninitialized service
func ErrServiceNotInitialized(serviceName string) error {
	return &AppError{
		Code:    "SERVICE_NOT_INITIALIZED",
		Message: fmt.Sprintf("service %s is not initialized", serviceName),
	}
}

// ErrServiceInitializationFailed creates an error for service initialization failure
func ErrServiceInitializationFailed(serviceName string, err error) error {
	return &AppError{
		Code:    "SERVICE_INIT_FAILED",
		Message: fmt.Sprintf("failed to initialize %s", serviceName),
		Err:     err,
	}
}

// ErrApplicationAlreadyRunning creates an error when application is already running
func ErrApplicationAlreadyRunning() error {
	return &AppError{
		Code:    "APP_ALREADY_RUNNING",
		Message: "application is already running",
	}
}

// ErrApplicationNotRunning creates an error when application is not running
func ErrApplicationNotRunning() error {
	return &AppError{
		Code:    "APP_NOT_RUNNING",
		Message: "application is not running",
	}
}
