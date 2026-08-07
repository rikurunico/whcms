package validate_test

import (
	"errors"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registerDTO struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required,max=50"`
	Role      string `json:"role" validate:"omitempty,oneof=admin staff client"`
	Age       int    `json:"age" validate:"omitempty,gte=18"`
}

func fieldMap(t *testing.T, err error) map[string]string {
	t.Helper()
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeValidation, ae.Code)
	m := map[string]string{}
	for _, d := range ae.Details {
		m[d.Field] = d.Message
	}
	return m
}

func TestStructValid(t *testing.T) {
	v := validate.New()
	err := v.Struct(registerDTO{
		Email:     "user@example.com",
		Password:  "supersecret",
		FirstName: "Budi",
		Role:      "client",
		Age:       30,
	})
	assert.NoError(t, err)
}

func TestStructCollectsFieldErrors(t *testing.T) {
	v := validate.New()
	err := v.Struct(registerDTO{
		Email:    "not-an-email",
		Password: "short",
		Role:     "superuser",
		Age:      12,
	})
	fields := fieldMap(t, err)

	assert.Equal(t, "must be a valid email address", fields["email"])
	assert.Equal(t, "must be at least 8 characters", fields["password"])
	assert.Equal(t, "is required", fields["first_name"])
	assert.Equal(t, "must be one of: admin, staff, client", fields["role"])
	assert.Equal(t, "must be at least 18", fields["age"])
}

func TestStructUsesJSONTagNames(t *testing.T) {
	type dto struct {
		FirstName string `json:"first_name" validate:"required"`
		NoTag     string `validate:"required"`
	}
	v := validate.New()
	fields := fieldMap(t, v.Struct(dto{}))
	assert.Contains(t, fields, "first_name", "json tag name used")
	assert.Contains(t, fields, "no_tag", "untagged falls back to snake_cased Go name")
}

func TestStructNested(t *testing.T) {
	type item struct {
		UnitPrice int `json:"unit_price" validate:"gt=0"`
	}
	type order struct {
		Items []item `json:"items" validate:"required,min=1,dive"`
	}
	v := validate.New()
	fields := fieldMap(t, v.Struct(order{Items: []item{{UnitPrice: 0}}}))
	assert.Equal(t, "must be greater than 0", fields["items[0].unit_price"])
}

func TestStructNonStructInput(t *testing.T) {
	v := validate.New()
	err := v.Struct(42)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeInternal, ae.Code)
}

func TestStructUsesGoNameWhenJSONTagDashed(t *testing.T) {
	type dto struct {
		Secret string `json:"-" validate:"required"`
	}
	v := validate.New()
	fields := fieldMap(t, v.Struct(dto{}))
	assert.Contains(t, fields, "secret", "json:\"-\" falls back to snake_cased Go name")
}

func TestStructMessagesCoverRemainingTags(t *testing.T) {
	type dto struct {
		MinNum int    `json:"min_num" validate:"min=5"`
		MaxStr string `json:"max_str" validate:"max=3"`
		MaxNum int    `json:"max_num" validate:"max=5"`
		Code   string `json:"code" validate:"len=5"`
		LtNum  int    `json:"lt_num" validate:"lt=10"`
		LteNum int    `json:"lte_num" validate:"lte=10"`
		Site   string `json:"site" validate:"omitempty,url"`
		ID     string `json:"id" validate:"omitempty,uuid"`
		Alpha  string `json:"alpha" validate:"alpha"`
	}
	v := validate.New()
	fields := fieldMap(t, v.Struct(dto{
		MinNum: 1,
		MaxStr: "abcdef",
		MaxNum: 10,
		Code:   "abc",
		LtNum:  10,
		LteNum: 11,
		Site:   "not a url",
		ID:     "not-a-uuid",
		Alpha:  "123",
	}))

	assert.Equal(t, "must be at least 5", fields["min_num"], "min on non-string kind")
	assert.Equal(t, "must be at most 3 characters", fields["max_str"], "max on string kind")
	assert.Equal(t, "must be at most 5", fields["max_num"], "max on non-string kind")
	assert.Equal(t, "must be exactly 5 characters", fields["code"])
	assert.Equal(t, "must be less than 10", fields["lt_num"])
	assert.Equal(t, "must be at most 10", fields["lte_num"])
	assert.Equal(t, "must be a valid URL", fields["site"])
	assert.Equal(t, "must be a valid UUID", fields["id"])
	assert.Equal(t, "failed validation: alpha", fields["alpha"], "unmapped tag falls back to default message")
}
