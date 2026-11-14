// Package domain holds the entities and invariants of the platform. It has no
// I/O: no database, no HTTP, no filesystem. Every rule that must hold regardless
// of the caller lives here.
package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when an aggregate does not exist (or is not
	// visible to the caller; the two are deliberately indistinguishable).
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a uniqueness or concurrency constraint fails.
	ErrConflict = errors.New("conflict")
	// ErrRateLimited is a per-tenant limit (HTTP 429).
	ErrRateLimited = errors.New("rate limited")
	// ErrInvalidTransition is returned by the task state machine.
	ErrInvalidTransition = errors.New("invalid transition")
	// ErrValidation is the sentinel for every field-level validation failure.
	ErrValidation = errors.New("validation failed")
)

// ValidationError names the field that failed and why.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Is lets callers match any validation failure with errors.Is(err, ErrValidation).
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
