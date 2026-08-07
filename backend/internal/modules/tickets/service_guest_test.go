package tickets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateGuestTicket

func TestCreateGuestTicketHappyPath(t *testing.T) {
	e := newFixture()
	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		require.Equal(t, int64(3), id)
		return activeDept(), nil
	}
	e.tickets.NextNumberFn = func(ctx context.Context, scope string) (int64, error) {
		assert.Equal(t, domain.TicketCounterScope, scope)
		return 42, nil
	}
	var created *domain.Ticket
	e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error {
		tk.ID = 11
		created = tk
		return nil
	}
	var reply *domain.TicketReply
	e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error {
		r.ID = 21
		reply = r
		return nil
	}
	var alerts []string
	e.notifier.AlertAdminFn = func(ctx context.Context, subject, message string) error {
		alerts = append(alerts, subject)
		return nil
	}
	// The guest path has no account: it must never look a client up.
	e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
		t.Fatalf("guest path must not look up a client")
		return nil, nil
	}

	ticket, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
		DepartmentID: 3,
		GuestName:    "Jane Guest",
		GuestEmail:   "jane@example.com",
		Subject:      "Cannot access site",
		Message:      "My website is unreachable",
		IP:           "203.0.113.7",
	})
	require.NoError(t, err)
	require.NotNil(t, ticket)

	require.NotNil(t, created)
	assert.Equal(t, "TKT-000042", created.TicketNumber)
	assert.Nil(t, created.ClientID)
	assert.Equal(t, int64(3), created.DepartmentID)
	assert.Equal(t, domain.TicketOpen, created.Status)
	assert.Equal(t, domain.PriorityMedium, created.Priority)
	assert.Equal(t, "Jane Guest", created.GuestName)
	assert.Equal(t, "jane@example.com", created.GuestEmail)
	require.NotNil(t, created.LastReplyAt)
	assert.Equal(t, fixedNow, *created.LastReplyAt)

	require.NotNil(t, reply)
	assert.Equal(t, int64(11), reply.TicketID)
	assert.Nil(t, reply.UserID)
	assert.Equal(t, "Jane Guest", reply.AuthorName)
	assert.Equal(t, "My website is unreachable", reply.Message)
	assert.False(t, reply.IsInternal)

	require.Len(t, alerts, 1)
	assert.Contains(t, alerts[0], "TKT-000042")

	assert.Equal(t, "TKT-000042", ticket.TicketNumber)
}

func TestCreateGuestTicketInvalidDepartment(t *testing.T) {
	tests := []struct {
		name string
		fn   func(ctx context.Context, id int64) (*domain.TicketDepartment, error)
	}{
		{"missing (nil)", func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return nil, nil
		}},
		{"inactive", func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			d := activeDept()
			d.Active = false
			return d, nil
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixture()
			e.tickets.GetDepartmentByIDFn = tc.fn
			createCalled := false
			e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error {
				createCalled = true
				return nil
			}
			_, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
				DepartmentID: 3, GuestName: "Jane", GuestEmail: "jane@example.com",
				Subject: "subject ok", Message: "message ok",
			})
			requireCode(t, err, apperr.CodeValidation)
			assert.False(t, createCalled, "ticket must not be created for an invalid department")
		})
	}
}

func TestCreateGuestTicketDefaultsToFirstActiveDepartment(t *testing.T) {
	e := newFixture()
	// department_id omitted (0): resolve the first active department instead.
	var listedActiveOnly bool
	e.tickets.ListDepartmentsFn = func(ctx context.Context, activeOnly bool) ([]domain.TicketDepartment, error) {
		listedActiveOnly = activeOnly
		return []domain.TicketDepartment{*activeDept(), {ID: 9, Name: "Sales", Active: true}}, nil
	}
	// The default path must NOT look a department up by id.
	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		t.Fatalf("default path must not call GetDepartmentByID")
		return nil, nil
	}
	e.tickets.NextNumberFn = func(ctx context.Context, scope string) (int64, error) { return 7, nil }
	var created *domain.Ticket
	e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error {
		tk.ID = 5
		created = tk
		return nil
	}

	ticket, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
		DepartmentID: 0, GuestName: "Jane", GuestEmail: "jane@example.com",
		Subject: "subject ok", Message: "message ok",
	})
	require.NoError(t, err)
	require.NotNil(t, ticket)
	assert.True(t, listedActiveOnly, "only active departments are considered")
	require.NotNil(t, created)
	assert.Equal(t, int64(3), created.DepartmentID, "routed to the first active department")
}

func TestCreateGuestTicketNoActiveDepartment(t *testing.T) {
	e := newFixture()
	e.tickets.ListDepartmentsFn = func(ctx context.Context, activeOnly bool) ([]domain.TicketDepartment, error) {
		return []domain.TicketDepartment{}, nil
	}
	createCalled := false
	e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error {
		createCalled = true
		return nil
	}
	_, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
		DepartmentID: 0, GuestName: "Jane", GuestEmail: "jane@example.com",
		Subject: "subject ok", Message: "message ok",
	})
	requireCode(t, err, apperr.CodeValidation)
	assert.False(t, createCalled, "no ticket without an active department")
}

func TestCreateGuestTicketListDepartmentsError(t *testing.T) {
	e := newFixture()
	e.tickets.ListDepartmentsFn = func(ctx context.Context, activeOnly bool) ([]domain.TicketDepartment, error) {
		return nil, errors.New("db down")
	}
	_, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
		DepartmentID: 0, GuestName: "Jane", GuestEmail: "jane@example.com",
		Subject: "subject ok", Message: "message ok",
	})
	require.Error(t, err)
}

func TestCreateGuestTicketAlertFailureSwallowed(t *testing.T) {
	e := newFixture()
	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		return activeDept(), nil
	}
	e.tickets.NextNumberFn = func(ctx context.Context, scope string) (int64, error) { return 1, nil }
	e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error { tk.ID = 5; return nil }
	e.notifier.AlertAdminFn = func(ctx context.Context, subject, message string) error {
		return errors.New("smtp down")
	}

	ticket, err := e.service().CreateGuestTicket(context.Background(), ports.GuestTicketInput{
		DepartmentID: 3, GuestName: "Jane", GuestEmail: "jane@example.com",
		Subject: "subject ok", Message: "message ok",
	})
	require.NoError(t, err, "a failed admin alert must not fail the request")
	require.NotNil(t, ticket)
	assert.Equal(t, "TKT-000001", ticket.TicketNumber)
}
