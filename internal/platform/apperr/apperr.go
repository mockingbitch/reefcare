// Package apperr defines the typed errors application services return. The HTTP layer
// maps them to status codes and the error envelope in one place (httpserver.WriteAppError).
package apperr

import (
	"errors"
	"fmt"
	"time"
)

// Kind classifies an application error.
type Kind int

// Error kinds, mapped to HTTP statuses by the transport layer.
const (
	KindValidation       Kind = iota + 1 // 422
	KindNotFound                         // 404
	KindConflict                         // 409
	KindUnauthenticated                  // 401
	KindForbidden                        // 403
	KindRateLimited                      // 429
	KindBadRequest                       // 400: malformed request syntax
	KindTooLarge                         // 413
	KindUnsupportedMedia                 // 415
	KindUnavailable                      // 503: temporarily overloaded; safe to retry later
)

// FieldError describes one invalid input field. Code is a stable machine-readable
// value the client localizes; Params carries values for the message (e.g. min, max).
type FieldError struct {
	Field  string
	Code   string
	Params map[string]string
}

// Error is an application error with a stable code safe to show to clients.
type Error struct {
	Kind       Kind
	Code       string
	Message    string
	Fields     []FieldError
	RetryAfter time.Duration
	cause      error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return e.Code + ": " + e.Message
}

// Unwrap returns the underlying cause, if any.
func (e *Error) Unwrap() error { return e.cause }

// WithCause attaches an internal cause for logging. It is never shown to clients.
func (e *Error) WithCause(err error) *Error {
	c := *e
	c.cause = err
	return &c
}

// As returns the *Error in err's chain, if any.
func As(err error) (*Error, bool) {
	var e *Error
	if !errors.As(err, &e) || e == nil {
		return nil, false
	}
	return e, true
}

// IsKind reports whether err is an application error of kind k.
func IsKind(err error, k Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == k
}

// NotFound reports a resource that does not exist or that the caller may not see.
func NotFound() *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: "Resource not found."}
}

// Conflict reports a state conflict such as a stale version or an already resolved item.
func Conflict(code, message string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

// Unauthenticated reports a missing or invalid session or credentials.
func Unauthenticated(code, message string) *Error {
	return &Error{Kind: KindUnauthenticated, Code: code, Message: message}
}

// Forbidden reports an authenticated caller lacking permission.
func Forbidden(code, message string) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

// RateLimited reports too many requests; retryAfter is surfaced as Retry-After.
func RateLimited(retryAfter time.Duration) *Error {
	return &Error{Kind: KindRateLimited, Code: "rate_limited", Message: "Too many requests. Try again later.", RetryAfter: retryAfter}
}

// Unavailable reports temporary overload; retryAfter is surfaced as Retry-After.
func Unavailable(retryAfter time.Duration) *Error {
	return &Error{Kind: KindUnavailable, Code: "server_busy", Message: "The server is busy. Try again shortly.", RetryAfter: retryAfter}
}

// BadRequest reports a syntactically malformed request (bad JSON, unknown field, bad query).
func BadRequest(code, message string, fields ...FieldError) *Error {
	return &Error{Kind: KindBadRequest, Code: code, Message: message, Fields: fields}
}

// Invalid reports a single domain-level validation failure that isn't tied to one field.
func Invalid(code, message string) *Error {
	return &Error{Kind: KindValidation, Code: code, Message: message}
}

// Validator collects field errors so a request reports every problem at once.
//
//	v := apperr.NewValidator()
//	v.Check(name != "", "name", "required", nil)
//	if err := v.Err(); err != nil { return err }
type Validator struct {
	fields []FieldError
}

// NewValidator returns an empty Validator.
func NewValidator() *Validator { return &Validator{} }

// Check records a field error when ok is false. It returns ok for chaining.
func (v *Validator) Check(ok bool, field, code string, params map[string]string) bool {
	if !ok {
		v.Add(field, code, params)
	}
	return ok
}

// Add records a field error unconditionally.
func (v *Validator) Add(field, code string, params map[string]string) {
	v.fields = append(v.fields, FieldError{Field: field, Code: code, Params: params})
}

// HasErrors reports whether any field error was recorded.
func (v *Validator) HasErrors() bool { return len(v.fields) > 0 }

// Err returns a validation error listing all recorded fields, or nil.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: "Some fields are invalid.", Fields: v.fields}
}
