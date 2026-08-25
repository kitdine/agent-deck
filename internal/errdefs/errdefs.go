// Package errdefs provides shared error carriers without coupling domain packages.
package errdefs

// NotFound carries a stable error code and an already-redacted message.
type NotFound struct {
	Code    string
	Message string
	cause   error
}

// NewNotFound constructs a not-found error whose cause remains matchable but is
// never included in the rendered message.
func NewNotFound(code, message string, cause error) *NotFound {
	return &NotFound{Code: code, Message: message, cause: cause}
}

func (e *NotFound) Error() string { return e.Message }

func (e *NotFound) Unwrap() error { return e.cause }
