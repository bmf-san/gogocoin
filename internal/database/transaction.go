package database

import (
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// TxFunc is a function that executes within a transaction
type TxFunc func(tx domain.Transaction) error

// WithTransaction executes a function within a transaction with automatic rollback on error
//
// This implements the RAII (Resource Acquisition Is Initialization) pattern for transaction management,
// ensuring that transactions are always properly cleaned up regardless of how the function exits.
//
// Behavior:
//   - The transaction is automatically rolled back if the function returns an error
//   - The transaction is automatically rolled back if the function panics (panic is re-thrown after rollback)
//   - The transaction is committed only if the function completes successfully (returns nil)
//   - If commit fails, an automatic rollback is attempted
//
// Benefits over manual transaction management:
//   - Prevents transaction leaks by guaranteeing cleanup
//   - Eliminates common bugs from forgetting to rollback on error paths
//   - Handles panic recovery to prevent orphaned transactions
//   - Makes transaction boundaries explicit and easy to audit
//   - Reduces boilerplate code
//
// Example usage:
//
//	err := WithTransaction(db, logger, func(tx domain.Transaction) error {
//	    if err := tx.SavePosition(position); err != nil {
//	        return err // Automatic rollback
//	    }
//	    if err := tx.SaveTrade(trade); err != nil {
//	        return err // Automatic rollback
//	    }
//	    return nil // Automatic commit
//	})
//
// This replaces the error-prone manual pattern:
//
//	tx, err := db.BeginTx()
//	if err != nil {
//	    return err
//	}
//	defer func() {
//	    if err != nil {
//	        tx.Rollback() // Easy to forget or implement incorrectly
//	    }
//	}()
//	// ... operations ...
//	return tx.Commit()
func WithTransaction(db domain.TransactionManager, logger *logger.Logger, fn TxFunc) error {
	tx, err := db.BeginTx()
	if err != nil {
		return err
	}

	// Track whether transaction was completed to prevent double rollback
	committed := false

	// Ensure rollback on panic or error
	defer func() {
		if r := recover(); r != nil {
			// Panic occurred - rollback and re-panic
			if !committed {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && logger != nil {
					logger.System().WithError(rollbackErr).Error("Failed to rollback transaction after panic")
				}
			}
			panic(r) // Re-throw panic after rollback
		}
	}()

	// Execute the function
	err = fn(tx)
	if err != nil {
		// Function returned error - rollback
		if rollbackErr := tx.Rollback(); rollbackErr != nil && logger != nil {
			logger.System().WithError(rollbackErr).Error("Failed to rollback transaction after error")
		}
		return err
	}

	// Success - commit transaction
	if err = tx.Commit(); err != nil {
		// Commit failed - attempt rollback (though it may fail too)
		if rollbackErr := tx.Rollback(); rollbackErr != nil && logger != nil {
			logger.System().WithError(rollbackErr).Error("Failed to rollback transaction after commit failure")
		}
		return err
	}

	committed = true
	return nil
}
