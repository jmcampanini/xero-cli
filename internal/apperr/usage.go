package apperr

import (
	"fmt"
)

// UsageError identifies invalid command usage for process-exit classification.
type UsageError struct{ error }

// Usage formats a human-readable command usage error.
func Usage(format string, args ...any) error { return &UsageError{fmt.Errorf(format, args...)} }

// ApplicationError distinguishes command execution failures from usage errors.
type ApplicationError struct{ Err error }

// Error returns the underlying application error message.
func (e *ApplicationError) Error() string { return e.Err.Error() }
