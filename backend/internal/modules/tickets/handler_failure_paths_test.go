package tickets_test

// Handler failure paths: invalid JSON bodies on the non-multipart bind
// paths are rejected with 422, invalid path params never reach the service,
// empty list results serialise as [] rather than null, and service errors
// pass through to the mapped status code on every route.

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/tickets"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bind/formFiles branches

func TestHandlerCreateTicketInvalidJSONBody(t *testing.T) {
	app := clientApp(&fakeSvc{})
	req := httptest.NewRequest("POST", "/api/v1/tickets/", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerReplyJSONBody(t *testing.T) {
	var gotIn tickets.ReplyInput
	svc := &fakeSvc{
		ReplyMineFn: func(ctx context.Context, clientID, ticketID int64, in tickets.ReplyInput, files []tickets.AttachmentUpload) (*tickets.TicketDetail, error) {
			gotIn = in
			return &tickets.TicketDetail{}, nil
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/tickets/11/replies", strings.NewReader(`{"message":"via json"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := clientApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, "via json", gotIn.Message)
}

func TestHandlerReplyInvalidJSONBody(t *testing.T) {
	app := clientApp(&fakeSvc{})
	req := httptest.NewRequest("POST", "/api/v1/tickets/11/replies", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

// Public

func TestHandlerPublicDepartmentsError(t *testing.T) {
	svc := &fakeSvc{
		ListDepartmentsFn: func(ctx context.Context, activeOnly bool) ([]domain.TicketDepartment, error) {
			return nil, apperr.Internal(errBoom)
		},
	}
	app := newApp(svc, httpx.AuthIdentity{}, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/ticket-departments", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestHandlerPublicDepartmentsEmpty(t *testing.T) {
	app := newApp(&fakeSvc{}, httpx.AuthIdentity{}, nil) // ListDepartmentsFn nil -> (nil, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/ticket-departments", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Data)
	assert.Len(t, env.Data.([]any), 0)
}

// Client routes

func TestHandlerMyListError(t *testing.T) {
	svc := &fakeSvc{
		ListMineFn: func(ctx context.Context, clientID int64, p ports.ListParams) ([]domain.Ticket, int64, error) {
			return nil, 0, apperr.Validation("invalid status filter")
		},
	}
	resp, err := clientApp(svc).Test(httptest.NewRequest("GET", "/api/v1/tickets/?status=bogus", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerMyListEmpty(t *testing.T) {
	resp, err := clientApp(&fakeSvc{}).Test(httptest.NewRequest("GET", "/api/v1/tickets/", nil)) // ListMineFn nil -> (nil, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Len(t, env.Data.([]any), 0)
}

func TestHandlerCreateTicketServiceError(t *testing.T) {
	svc := &fakeSvc{
		CreateTicketFn: func(ctx context.Context, clientID int64, in tickets.CreateTicketInput, files []tickets.AttachmentUpload) (*tickets.TicketDetail, error) {
			return nil, apperr.NotFound("department")
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/tickets/",
		strings.NewReader(`{"department_id":3,"subject":"Hi","message":"Text"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := clientApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlerMyReplyInvalidID(t *testing.T) {
	resp, err := clientApp(&fakeSvc{}).Test(httptest.NewRequest("POST", "/api/v1/tickets/abc/replies",
		strings.NewReader(`{"message":"hi"}`)))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerMyReplyServiceError(t *testing.T) {
	svc := &fakeSvc{
		ReplyMineFn: func(ctx context.Context, clientID, ticketID int64, in tickets.ReplyInput, files []tickets.AttachmentUpload) (*tickets.TicketDetail, error) {
			return nil, apperr.Conflict("ticket is closed")
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/tickets/11/replies", strings.NewReader(`{"message":"hi there"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := clientApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
}

func TestHandlerMyCloseInvalidID(t *testing.T) {
	resp, err := clientApp(&fakeSvc{}).Test(httptest.NewRequest("POST", "/api/v1/tickets/abc/close", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerMyCloseServiceError(t *testing.T) {
	svc := &fakeSvc{
		CloseMineFn: func(ctx context.Context, clientID, ticketID int64) (*domain.Ticket, error) {
			return nil, apperr.Conflict("ticket is already closed")
		},
	}
	resp, err := clientApp(svc).Test(httptest.NewRequest("POST", "/api/v1/tickets/11/close", nil))
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
}

func TestHandlerMyAttachmentInvalidID(t *testing.T) {
	resp, err := clientApp(&fakeSvc{}).Test(httptest.NewRequest("GET", "/api/v1/tickets/abc/attachments/0", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerMyAttachmentServiceError(t *testing.T) {
	svc := &fakeSvc{
		AttachmentURLFn: func(ctx context.Context, clientID, ticketID int64, idx int) (string, error) {
			return "", apperr.NotFound("attachment")
		},
	}
	resp, err := clientApp(svc).Test(httptest.NewRequest("GET", "/api/v1/tickets/11/attachments/9", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

// Admin routes

func TestHandlerAdminListError(t *testing.T) {
	svc := &fakeSvc{
		AdminListFn: func(ctx context.Context, f tickets.AdminListFilter) ([]domain.Ticket, int64, error) {
			return nil, 0, apperr.Validation("invalid status filter")
		},
	}
	resp, err := adminApp(svc).Test(httptest.NewRequest("GET", "/api/v1/admin/tickets/?status=bogus", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminDetailServiceError(t *testing.T) {
	svc := &fakeSvc{
		AdminGetFn: func(ctx context.Context, ticketID int64) (*tickets.TicketDetail, error) {
			return nil, apperr.NotFound("ticket")
		},
	}
	resp, err := adminApp(svc).Test(httptest.NewRequest("GET", "/api/v1/admin/tickets/11", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlerAdminAssignInvalidID(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/abc/assign", strings.NewReader(`{"assigned_user_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminAssignServiceError(t *testing.T) {
	svc := &fakeSvc{
		AssignFn: func(ctx context.Context, actorUserID, ticketID int64, in tickets.AssignInput) (*domain.Ticket, error) {
			return nil, apperr.Validation("invalid assignee")
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/11/assign", strings.NewReader(`{"assigned_user_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminReplyInvalidID(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/abc/replies", strings.NewReader(`{"message":"hi there"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminReplyInvalidJSONBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/11/replies", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminReplyServiceError(t *testing.T) {
	svc := &fakeSvc{
		AdminReplyFn: func(ctx context.Context, actorUserID, ticketID int64, in tickets.ReplyInput, files []tickets.AttachmentUpload) (*tickets.TicketDetail, error) {
			return nil, apperr.Conflict("ticket is closed")
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/11/replies", strings.NewReader(`{"message":"hi there"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
}

func TestHandlerAdminSetStatusInvalidID(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/abc/status", strings.NewReader(`{"status":"closed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminSetStatusServiceError(t *testing.T) {
	svc := &fakeSvc{
		SetStatusFn: func(ctx context.Context, actorUserID, ticketID int64, in tickets.SetStatusInput) (*domain.Ticket, error) {
			return nil, apperr.Conflict("ticket already has status closed")
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/admin/tickets/11/status", strings.NewReader(`{"status":"closed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
}

func TestHandlerAdminAttachmentInvalidID(t *testing.T) {
	resp, err := adminApp(&fakeSvc{}).Test(httptest.NewRequest("GET", "/api/v1/admin/tickets/abc/attachments/0", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminAttachmentServiceError(t *testing.T) {
	svc := &fakeSvc{
		AttachmentURLFn: func(ctx context.Context, clientID, ticketID int64, idx int) (string, error) {
			return "", apperr.NotFound("attachment")
		},
	}
	resp, err := adminApp(svc).Test(httptest.NewRequest("GET", "/api/v1/admin/tickets/11/attachments/9", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

// Departments admin

func TestHandlerAdminDepartmentsError(t *testing.T) {
	svc := &fakeSvc{
		ListDepartmentsFn: func(ctx context.Context, activeOnly bool) ([]domain.TicketDepartment, error) {
			return nil, apperr.Internal(errBoom)
		},
	}
	resp, err := adminApp(svc).Test(httptest.NewRequest("GET", "/api/v1/admin/ticket-departments/", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestHandlerAdminDepartmentsEmpty(t *testing.T) {
	resp, err := adminApp(&fakeSvc{}).Test(httptest.NewRequest("GET", "/api/v1/admin/ticket-departments/", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Len(t, env.Data.([]any), 0)
}

func TestHandlerAdminCreateDepartmentServiceError(t *testing.T) {
	svc := &fakeSvc{
		CreateDepartmentFn: func(ctx context.Context, actorUserID int64, in tickets.DepartmentInput) (*domain.TicketDepartment, error) {
			return nil, apperr.Internal(errBoom)
		},
	}
	req := httptest.NewRequest("POST", "/api/v1/admin/ticket-departments/", strings.NewReader(`{"name":"Billing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestHandlerAdminUpdateDepartmentInvalidID(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/api/v1/admin/ticket-departments/abc", strings.NewReader(`{"name":"Billing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminUpdateDepartmentInvalidJSONBody(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/api/v1/admin/ticket-departments/8", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(&fakeSvc{}).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestHandlerAdminUpdateDepartmentServiceError(t *testing.T) {
	svc := &fakeSvc{
		UpdateDepartmentFn: func(ctx context.Context, actorUserID, id int64, in tickets.DepartmentInput) (*domain.TicketDepartment, error) {
			return nil, apperr.NotFound("department")
		},
	}
	req := httptest.NewRequest("PATCH", "/api/v1/admin/ticket-departments/8", strings.NewReader(`{"name":"Billing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp(svc).Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHandlerAdminDeleteDepartmentInvalidID(t *testing.T) {
	resp, err := adminApp(&fakeSvc{}).Test(httptest.NewRequest("DELETE", "/api/v1/admin/ticket-departments/abc", nil))
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}
