package errors

import "fmt"

// Trading-specific error constructors

// NewInsufficientBalanceError creates an insufficient balance error
func NewInsufficientBalanceError(required, available float64, currency string) *Error {
	return New(
		ErrInsufficientBalance,
		fmt.Sprintf("insufficient %s balance: required %.8f, available %.8f", currency, required, available),
	).WithContext("required", required).
		WithContext("available", available).
		WithContext("currency", currency)
}

// NewOrderValidationError creates an order validation error
func NewOrderValidationError(reason string) *Error {
	return New(ErrOrderValidation, reason)
}

// NewOrderExecutionError creates an order execution error
func NewOrderExecutionError(orderID string, err error) *Error {
	return Wrap(err, ErrOrderExecution, fmt.Sprintf("failed to execute order %s", orderID)).
		WithContext("order_id", orderID)
}

// NewOrderNotFoundError creates an order not found error
func NewOrderNotFoundError(orderID string) *Error {
	return New(ErrOrderNotFound, fmt.Sprintf("order %s not found", orderID)).
		WithContext("order_id", orderID)
}
