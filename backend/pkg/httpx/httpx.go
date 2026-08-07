// Package httpx contains the HTTP response envelope, pagination helpers and
// the request Identity accessor shared by all transport handlers.
//
// Envelope (CONTRACTS.md §3):
//
//	success: {"data": ..., "meta": {...}|null, "error": null}
//	error:   {"data": null, "error": {"code": "...", "message": "...", "details": [...]}}
package httpx

import (
	"strconv"

	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/gofiber/fiber/v3"
)

// Envelope is the uniform JSON response body.
type Envelope struct {
	Data  any        `json:"data"`
	Meta  *Meta      `json:"meta"`
	Error *ErrorBody `json:"error"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

// ErrorBody is the error part of the envelope.
type ErrorBody struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details []apperr.FieldError `json:"details,omitempty"`
}

// OK writes a 200 response with data (and optional pagination meta).
func OK(c fiber.Ctx, data any, meta ...*Meta) error {
	var m *Meta
	if len(meta) > 0 {
		m = meta[0]
	}
	return c.Status(fiber.StatusOK).JSON(Envelope{Data: data, Meta: m})
}

// Created writes a 201 response with data.
func Created(c fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope{Data: data})
}

// NoContent writes a 204 response with no body.
func NoContent(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// StatusFor maps an apperr code to an HTTP status code.
func StatusFor(code apperr.Code) int {
	switch code {
	case apperr.CodeValidation:
		return fiber.StatusUnprocessableEntity
	case apperr.CodeUnauthorized:
		return fiber.StatusUnauthorized
	case apperr.CodeForbidden:
		return fiber.StatusForbidden
	case apperr.CodeNotFound:
		return fiber.StatusNotFound
	case apperr.CodeConflict:
		return fiber.StatusConflict
	case apperr.CodeRateLimited:
		return fiber.StatusTooManyRequests
	case apperr.CodePaymentRequired:
		return fiber.StatusPaymentRequired
	case apperr.CodeExternal:
		return fiber.StatusBadGateway
	default:
		return fiber.StatusInternalServerError
	}
}

// Fail writes an error envelope from any error. *apperr.Error values map to
// their HTTP status; anything else becomes 500 INTERNAL.
func Fail(c fiber.Ctx, err error) error {
	e := apperr.From(err)
	body := &ErrorBody{Code: string(e.Code), Message: e.Message, Details: e.Details}
	if e.Code == apperr.CodeInternal {
		// Never leak internals to the client.
		body.Message = "internal error"
	}
	return c.Status(StatusFor(e.Code)).JSON(Envelope{Error: body})
}

// Pagination defaults and limits (CONTRACTS.md §3). The frontend page-size
// selector offers 10/25/50/100/500/1000; per_page is clamped to
// [1, MaxPerPage] here regardless of which of those values is sent.
const (
	DefaultPage    = 1
	DefaultPerPage = 10
	MaxPerPage     = 1000
)

// Page is a parsed pagination request.
type Page struct {
	Page    int
	PerPage int
}

// Offset returns the SQL OFFSET for the page.
func (p Page) Offset() int { return (p.Page - 1) * p.PerPage }

// Limit returns the SQL LIMIT for the page.
func (p Page) Limit() int { return p.PerPage }

// Meta builds pagination meta with the given total row count.
func (p Page) Meta(total int64) *Meta {
	return &Meta{Page: p.Page, PerPage: p.PerPage, Total: total}
}

// ParsePage reads ?page=&per_page= with defaults 1/10 and clamps per_page to
// [1, 1000]. Invalid values fall back to defaults.
func ParsePage(c fiber.Ctx) Page {
	p := Page{Page: DefaultPage, PerPage: DefaultPerPage}
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v >= 1 {
		p.Page = v
	}
	if v, err := strconv.Atoi(c.Query("per_page")); err == nil && v >= 1 {
		if v > MaxPerPage {
			v = MaxPerPage
		}
		p.PerPage = v
	}
	return p
}
