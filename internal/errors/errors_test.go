package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrInsufficientBalance, "not enough funds")

	if err.Code != ErrInsufficientBalance {
		t.Errorf("Expected code %s, got %s", ErrInsufficientBalance, err.Code)
	}

	if err.Message != "not enough funds" {
		t.Errorf("Expected message 'not enough funds', got '%s'", err.Message)
	}

	expected := "[INSUFFICIENT_BALANCE] not enough funds"
	if err.Error() != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(cause, ErrOrderExecution, "failed to execute")

	if err.Code != ErrOrderExecution {
		t.Errorf("Expected code %s, got %s", ErrOrderExecution, err.Code)
	}

	if err.Cause != cause {
		t.Error("Expected wrapped error to have cause")
	}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should return true for wrapped error")
	}
}

func TestWithContext(t *testing.T) {
	err := New(ErrOrderValidation, "invalid order").
		WithContext("order_id", "12345").
		WithContext("symbol", "BTC_JPY")

	if err.Context["order_id"] != "12345" {
		t.Error("Expected order_id in context")
	}

	if err.Context["symbol"] != "BTC_JPY" {
		t.Error("Expected symbol in context")
	}
}

func TestIs(t *testing.T) {
	err := New(ErrInsufficientBalance, "test")

	if !Is(err, ErrInsufficientBalance) {
		t.Error("Is should return true for matching error code")
	}

	if Is(err, ErrOrderValidation) {
		t.Error("Is should return false for non-matching error code")
	}

	// Test with non-Error type
	regularErr := errors.New("regular error")
	if Is(regularErr, ErrInsufficientBalance) {
		t.Error("Is should return false for non-Error types")
	}
}

func TestNewInsufficientBalanceError(t *testing.T) {
	err := NewInsufficientBalanceError(100.0, 50.0, "JPY")

	if err.Code != ErrInsufficientBalance {
		t.Errorf("Expected code %s, got %s", ErrInsufficientBalance, err.Code)
	}

	if err.Context["required"] != 100.0 {
		t.Error("Expected required amount in context")
	}

	if err.Context["available"] != 50.0 {
		t.Error("Expected available amount in context")
	}

	if err.Context["currency"] != "JPY" {
		t.Error("Expected currency in context")
	}
}

func TestNewOrderValidationError(t *testing.T) {
	err := NewOrderValidationError("invalid size")

	if err.Code != ErrOrderValidation {
		t.Errorf("Expected code %s, got %s", ErrOrderValidation, err.Code)
	}

	if err.Message != "invalid size" {
		t.Errorf("Expected message 'invalid size', got '%s'", err.Message)
	}
}

func TestNewAPIFailureError(t *testing.T) {
	cause := errors.New("connection timeout")
	err := NewAPIFailureError("/api/orders", 500, cause)

	if err.Code != ErrAPIFailure {
		t.Errorf("Expected code %s, got %s", ErrAPIFailure, err.Code)
	}

	if err.Context["endpoint"] != "/api/orders" {
		t.Error("Expected endpoint in context")
	}

	if err.Context["status_code"] != 500 {
		t.Error("Expected status_code in context")
	}

	if err.Cause != cause {
		t.Error("Expected cause to be wrapped")
	}
}
