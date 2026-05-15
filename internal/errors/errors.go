// Package errors provides unk-specific error types.
package errors

import (
	stderrors "errors"
	"fmt"
	"os"
	"strings"
)

// userError is a user-facing error with an optional set of detail lines.
type userError struct {
	msg     string
	details []string
}

func (e *userError) Error() string { return e.msg }

// Option is a functional option for configuring a userError.
type Option func(*userError)

// WithDetails appends bullet-point detail lines to the error message.
func WithDetails(details ...string) Option {
	return func(e *userError) { e.details = append(e.details, details...) }
}

// NewUserError creates a user-facing error. Optional detail lines can be
// supplied via WithDetails.
func NewUserError(msg string, opts ...Option) error {
	e := &userError{msg: msg}
	for _, o := range opts {
		o(e)
	}
	return e
}

// FormatCLIError renders an error for display at the top-level CLI boundary.
func FormatCLIError(err error) string {
	var ue *userError
	if isUserError(err, &ue) {
		lines := []string{fmt.Sprintf("unk: %s", ue.msg)}
		if len(ue.details) > 0 {
			lines = append(lines, "")
			lines = append(lines, ue.details...)
		}
		return strings.Join(lines, "\n") + "\n"
	}

	if os.Getenv("HUNK_DEBUG") == "1" {
		return fmt.Sprintf("unk: %+v\n", err)
	}
	return fmt.Sprintf("unk: %s\n", err.Error())
}

// isUserError reports whether err (or any error it wraps) is a *userError,
// and populates target if non-nil. Uses errors.As so wrapped errors are matched.
func isUserError(err error, target **userError) bool {
	return stderrors.As(err, target)
}
