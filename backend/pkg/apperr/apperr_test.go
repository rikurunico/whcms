package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAndError(t *testing.T) {
	e := apperr.New(apperr.CodeNotFound, "client not found")
	assert.Equal(t, apperr.CodeNotFound, e.Code)
	assert.Equal(t, "NOT_FOUND: client not found", e.Error())
}

func TestNewf(t *testing.T) {
	e := apperr.Newf(apperr.CodeConflict, "invoice %d already paid", 42)
	assert.Equal(t, "CONFLICT: invoice 42 already paid", e.Error())
}

func TestWithCauseUnwrap(t *testing.T) {
	cause := errors.New("connection refused")
	e := apperr.Internal(cause)
	assert.ErrorIs(t, e, cause)
	assert.Contains(t, e.Error(), "connection refused")
	assert.Equal(t, cause, e.Unwrap())
}

func TestErrorsIsMatchesByCode(t *testing.T) {
	a := apperr.New(apperr.CodeNotFound, "a")
	b := apperr.New(apperr.CodeNotFound, "b")
	c := apperr.New(apperr.CodeConflict, "c")
	assert.True(t, errors.Is(a, b))
	assert.False(t, errors.Is(a, c))
	assert.False(t, errors.Is(a, errors.New("plain")))
}

func TestErrorsAsThroughWrapping(t *testing.T) {
	inner := apperr.Validation("bad input")
	wrapped := fmt.Errorf("handler: %w", inner)
	var target *apperr.Error
	require.True(t, errors.As(wrapped, &target))
	assert.Equal(t, apperr.CodeValidation, target.Code)
}

func TestWithDetailsDoesNotMutateOriginal(t *testing.T) {
	orig := apperr.New(apperr.CodeValidation, "invalid")
	withD := orig.WithDetails(apperr.FieldError{Field: "email", Message: "required"})
	assert.Empty(t, orig.Details)
	require.Len(t, withD.Details, 1)
	assert.Equal(t, "email", withD.Details[0].Field)

	more := withD.WithDetails(apperr.FieldError{Field: "name", Message: "required"})
	assert.Len(t, withD.Details, 1)
	assert.Len(t, more.Details, 2)
}

func TestConvenienceConstructors(t *testing.T) {
	tests := []struct {
		name string
		err  *apperr.Error
		code apperr.Code
	}{
		{"NotFound", apperr.NotFound("invoice"), apperr.CodeNotFound},
		{"Validation", apperr.Validation("bad"), apperr.CodeValidation},
		{"Unauthorized", apperr.Unauthorized("no token"), apperr.CodeUnauthorized},
		{"Forbidden", apperr.Forbidden("nope"), apperr.CodeForbidden},
		{"Conflict", apperr.Conflict("dupe"), apperr.CodeConflict},
		{"Internal", apperr.Internal(errors.New("x")), apperr.CodeInternal},
		{"External", apperr.External("duitku", errors.New("500")), apperr.CodeExternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
		})
	}
	assert.Equal(t, "invoice not found", apperr.NotFound("invoice").Message)
}

func TestFrom(t *testing.T) {
	assert.Nil(t, apperr.From(nil))

	e := apperr.Forbidden("no")
	assert.Same(t, e, apperr.From(e))

	wrapped := fmt.Errorf("svc: %w", e)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(wrapped).Code)

	plain := errors.New("boom")
	got := apperr.From(plain)
	assert.Equal(t, apperr.CodeInternal, got.Code)
	assert.ErrorIs(t, got, plain)
}
