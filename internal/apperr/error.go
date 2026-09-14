// Package apperr defines the CLI's stable application error codes.
package apperr

import (
	"errors"
	"fmt"
)

// Error is an application failure with a machine-readable code.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// New formats a coded error without retaining sensitive response objects.
func New(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// From preserves a coded error and classifies unexpected failures as internal.
func From(err error) *Error {
	var coded *Error
	if errors.As(err, &coded) {
		return coded
	}
	return &Error{Code: "internal", Message: err.Error()}
}
