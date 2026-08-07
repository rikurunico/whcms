// Package apperr defines the application error type and error codes used
// across all services. Services return *apperr.Error; the HTTP transport
// layer maps codes to status codes in a single Fiber error handler.
package apperr

import (
	"errors"
	"fmt"
)

// Code is a machine-readable error code.
type Code string

// Error codes (CONTRACTS.md §3).
const (
	CodeValidation      Code = "VALIDATION"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeRateLimited     Code = "RATE_LIMITED"
	CodePaymentRequired Code = "PAYMENT_REQUIRED"
	CodeExternal        Code = "EXTERNAL"
	CodeInternal        Code = "INTERNAL"
)

// FieldError describes a per-field validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error is the application error type. It is errors.Is/As compatible.
type Error struct {
	Code    Code         `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
	cause   error
}

// New creates a new *Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf creates a new *Error with a formatted message.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether target is an *Error with the same code.
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// WithDetails returns a copy of e with per-field details attached.
func (e *Error) WithDetails(details ...FieldError) *Error {
	clone := *e
	clone.Details = append(append([]FieldError(nil), e.Details...), details...)
	return &clone
}

// WithCause returns a copy of e wrapping the given cause error.
func (e *Error) WithCause(cause error) *Error {
	clone := *e
	clone.cause = cause
	return &clone
}

// Convenience constructors.

// NotFound returns a NOT_FOUND error for the named entity.
func NotFound(entity string) *Error {
	return Newf(CodeNotFound, "%s not found", entity)
}

// Validation returns a VALIDATION error with the given message.
func Validation(message string, details ...FieldError) *Error {
	return New(CodeValidation, message).WithDetails(details...)
}

// Unauthorized returns an UNAUTHORIZED error.
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }

// Forbidden returns a FORBIDDEN error.
func Forbidden(message string) *Error { return New(CodeForbidden, message) }

// Conflict returns a CONFLICT error.
func Conflict(message string) *Error { return New(CodeConflict, message) }

// Internal returns an INTERNAL error wrapping cause.
func Internal(cause error) *Error {
	return New(CodeInternal, "internal error").WithCause(cause)
}

// External returns an EXTERNAL error (upstream provider failure) wrapping cause.
func External(provider string, cause error) *Error {
	return Newf(CodeExternal, "%s error", provider).WithCause(cause)
}

// From extracts an *Error from err. If err is not an *Error it returns an
// INTERNAL error wrapping err. Returns nil for nil input.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}
