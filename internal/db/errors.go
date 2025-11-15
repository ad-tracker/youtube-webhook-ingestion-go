package db

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when a requested record is not found.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateKey is returned when attempting to insert a duplicate record.
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
	ErrForeignKeyViolation = errors.New("foreign key violation")

	// ErrImmutableRecord is returned when attempting to modify an immutable record.
	ErrImmutableRecord = errors.New("record is immutable and cannot be modified")
)

// WrapError wraps database errors with additional context and maps them to custom error types.
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Handle pgx specific errors
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	}

	// Handle PostgreSQL errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: %w (constraint: %s)", operation, ErrDuplicateKey, pgErr.ConstraintName)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%s: %w (constraint: %s)", operation, ErrForeignKeyViolation, pgErr.ConstraintName)
		case "P0001": // raise_exception (from our trigger)
			return fmt.Errorf("%s: %w: %s", operation, ErrImmutableRecord, pgErr.Message)
		default:
			return fmt.Errorf("%s: database error [%s]: %w", operation, pgErr.Code, err)
		}
	}

	return fmt.Errorf("%s: %w", operation, err)
}

// IsNotFound returns true if the error is an ErrNotFound error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicateKey returns true if the error is an ErrDuplicateKey error.
func IsDuplicateKey(err error) bool {
	return errors.Is(err, ErrDuplicateKey)
}

// IsForeignKeyViolation returns true if the error is an ErrForeignKeyViolation error.
func IsForeignKeyViolation(err error) bool {
	return errors.Is(err, ErrForeignKeyViolation)
}

// IsImmutableRecord returns true if the error is an ErrImmutableRecord error.
func IsImmutableRecord(err error) bool {
	return errors.Is(err, ErrImmutableRecord)
}
