package contact_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/contact"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env bundles the mocked dependencies for one test.
type env struct {
	tickets *mocks.MockGuestTicketCreator
}

func newFixture() *env {
	return &env{tickets: &mocks.MockGuestTicketCreator{}}
}

func (e *env) service() *contact.Service {
	return contact.New(contact.Deps{
		Tickets: e.tickets,
		Clock:   &mocks.MockClock{},
	})
}

func validInput() contact.ContactInput {
	return contact.ContactInput{
		Name:    "Jane Guest",
		Email:   "jane@example.com",
		Subject: "Cannot access site",
		Message: "My website is unreachable",
	}
}

func requireCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, code, apperr.From(err).Code)
}

// Submit

func TestSubmitHappyPath(t *testing.T) {
	e := newFixture()
	var got ports.GuestTicketInput
	e.tickets.CreateGuestTicketFn = func(ctx context.Context, in ports.GuestTicketInput) (*domain.Ticket, error) {
		got = in
		return &domain.Ticket{ID: 11, TicketNumber: "TKT-000042"}, nil
	}

	in := validInput()
	in.DepartmentID = 3
	ticket, err := e.service().Submit(context.Background(), in, "203.0.113.7")
	require.NoError(t, err)
	require.NotNil(t, ticket)
	assert.Equal(t, "TKT-000042", ticket.TicketNumber)

	// The form fields are forwarded verbatim, including the caller IP.
	assert.Equal(t, int64(3), got.DepartmentID)
	assert.Equal(t, "Jane Guest", got.GuestName)
	assert.Equal(t, "jane@example.com", got.GuestEmail)
	assert.Equal(t, "Cannot access site", got.Subject)
	assert.Equal(t, "My website is unreachable", got.Message)
	assert.Equal(t, "203.0.113.7", got.IP)
}

func TestSubmitDefaultsDepartmentToZero(t *testing.T) {
	e := newFixture()
	var got ports.GuestTicketInput
	e.tickets.CreateGuestTicketFn = func(ctx context.Context, in ports.GuestTicketInput) (*domain.Ticket, error) {
		got = in
		return &domain.Ticket{TicketNumber: "TKT-000001"}, nil
	}
	// No department picked: the contact service forwards 0 and lets tickets
	// resolve the first active department.
	_, err := e.service().Submit(context.Background(), validInput(), "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), got.DepartmentID)
}

func TestSubmitValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*contact.ContactInput)
	}{
		{"missing name", func(in *contact.ContactInput) { in.Name = "" }},
		{"short name", func(in *contact.ContactInput) { in.Name = "x" }},
		{"missing email", func(in *contact.ContactInput) { in.Email = "" }},
		{"bad email", func(in *contact.ContactInput) { in.Email = "not-an-email" }},
		{"missing subject", func(in *contact.ContactInput) { in.Subject = "" }},
		{"short subject", func(in *contact.ContactInput) { in.Subject = "hi" }},
		{"missing message", func(in *contact.ContactInput) { in.Message = "" }},
		{"short message", func(in *contact.ContactInput) { in.Message = "x" }},
		{"negative department", func(in *contact.ContactInput) { in.DepartmentID = -1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixture()
			called := false
			e.tickets.CreateGuestTicketFn = func(ctx context.Context, in ports.GuestTicketInput) (*domain.Ticket, error) {
				called = true
				return &domain.Ticket{}, nil
			}
			in := validInput()
			tc.mut(&in)
			_, err := e.service().Submit(context.Background(), in, "")
			requireCode(t, err, apperr.CodeValidation)
			assert.False(t, called, "no ticket must be created on invalid input")
		})
	}
}

func TestSubmitPropagatesValidationFromTickets(t *testing.T) {
	e := newFixture()
	e.tickets.CreateGuestTicketFn = func(ctx context.Context, in ports.GuestTicketInput) (*domain.Ticket, error) {
		return nil, apperr.Validation("invalid department",
			apperr.FieldError{Field: "department_id", Message: "department not found or inactive"})
	}
	in := validInput()
	in.DepartmentID = 99
	_, err := e.service().Submit(context.Background(), in, "")
	requireCode(t, err, apperr.CodeValidation)
}

func TestSubmitWrapsRepoError(t *testing.T) {
	e := newFixture()
	e.tickets.CreateGuestTicketFn = func(ctx context.Context, in ports.GuestTicketInput) (*domain.Ticket, error) {
		return nil, errors.New("db is down")
	}
	_, err := e.service().Submit(context.Background(), validInput(), "")
	requireCode(t, err, apperr.CodeInternal)
}
