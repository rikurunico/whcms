// Package validate wraps go-playground/validator and converts violations into
// friendly per-field VALIDATION apperr details (CONTRACTS.md §3).
package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/go-playground/validator/v10"
)

// Validator wraps a configured validator instance.
type Validator struct {
	v *validator.Validate
}

// New builds a Validator that reports json-tag field names when present.
func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name // empty -> validator falls back to the Go field name
	})
	return &Validator{v: v}
}

// Struct validates dto and returns nil or a VALIDATION *apperr.Error with
// per-field details.
func (val *Validator) Struct(dto any) error {
	err := val.v.Struct(dto)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return apperr.Internal(invalid)
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return apperr.Internal(err)
	}
	details := make([]apperr.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		details = append(details, apperr.FieldError{
			Field:   fieldName(fe),
			Message: message(fe),
		})
	}
	return apperr.Validation("validation failed", details...)
}

func fieldName(fe validator.FieldError) string {
	// Namespace is like "CreateOrderDTO.Items[0].Cycle" - drop the root struct.
	ns := fe.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		ns = ns[i+1:]
	}
	return toSnake(ns)
}

// toSnake converts CamelCase path segments to snake_case (Email -> email,
// FirstName -> first_name, Items[0].UnitPrice -> items[0].unit_price).
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				if prev != '.' && prev != '[' && !(prev >= 'A' && prev <= 'Z') {
					b.WriteByte('_')
				}
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "gt":
		return fmt.Sprintf("must be greater than %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "lt":
		return fmt.Sprintf("must be less than %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "url":
		return "must be a valid URL"
	case "uuid":
		return "must be a valid UUID"
	default:
		return fmt.Sprintf("failed validation: %s", fe.Tag())
	}
}
