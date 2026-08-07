package tickets_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/tickets"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Service failure paths: repo/storage failures must surface as errors and
// never half-apply state.

var errBoom = errors.New("boom")

func TestCreateTicketRepoFailures(t *testing.T) {
	in := tickets.CreateTicketInput{DepartmentID: 3, Subject: "subject ok", Message: "message ok"}

	t.Run("client lookup fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return nil, errBoom }
		_, err := e.service().CreateTicket(context.Background(), 7, in, nil)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("counter fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		e.tickets.NextNumberFn = func(ctx context.Context, scope string) (int64, error) { return 0, errBoom }
		_, err := e.service().CreateTicket(context.Background(), 7, in, nil)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("insert fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		e.tickets.CreateFn = func(ctx context.Context, tk *domain.Ticket) error { return errBoom }
		_, err := e.service().CreateTicket(context.Background(), 7, in, nil)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("reply insert fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { return errBoom }
		_, err := e.service().CreateTicket(context.Background(), 7, in, nil)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("settings errors surface as internal", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.settings.GetJSONFn = func(ctx context.Context, key string, out any) error { return errBoom }
		_, err := e.service().CreateTicket(context.Background(), 7, in,
			[]tickets.AttachmentUpload{upload("a.png", "x")})
		requireCode(t, err, apperr.CodeInternal)

		e.settings.GetJSONFn = nil
		e.settings.GetIntFn = func(ctx context.Context, key string, def int) (int, error) { return 0, errBoom }
		_, err = e.service().CreateTicket(context.Background(), 7, in,
			[]tickets.AttachmentUpload{upload("a.png", "x")})
		requireCode(t, err, apperr.CodeInternal)
	})
}

func TestGetMineRepoError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().GetMine(context.Background(), 7, 11)
	require.ErrorIs(t, err, errBoom)
}

func TestGetMineNilTicketIs404(t *testing.T) {
	// mock default returns (nil, nil): treat as not found
	_, err := newFixture().service().GetMine(context.Background(), 7, 11)
	requireCode(t, err, apperr.CodeNotFound)
}

func TestDetailErrors(t *testing.T) {
	t.Run("replies fail", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.tickets.ListRepliesFn = func(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.TicketReply, error) {
			return nil, errBoom
		}
		_, err := e.service().AdminGet(context.Background(), 11)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("department fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return nil, errBoom
		}
		_, err := e.service().AdminGet(context.Background(), 11)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestDetailSkipsGarbageAttachments(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		return activeDept(), nil
	}
	e.tickets.ListRepliesFn = func(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.TicketReply, error) {
		return []domain.TicketReply{{ID: 1, Message: "x", Attachments: json.RawMessage(`not-json`)}}, nil
	}
	detail, err := e.service().AdminGet(context.Background(), 11)
	require.NoError(t, err)
	require.Len(t, detail.Replies, 1)
	assert.Empty(t, detail.Replies[0].Attachments)
}

func TestReplyMineErrors(t *testing.T) {
	t.Run("attachment validation", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		_, err := e.service().ReplyMine(context.Background(), 7, 11,
			tickets.ReplyInput{Message: "hello"}, []tickets.AttachmentUpload{upload("bad.exe", "x")})
		requireCode(t, err, apperr.CodeValidation)
	})
	t.Run("client lookup fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return nil, errBoom }
		_, err := e.service().ReplyMine(context.Background(), 7, 11, tickets.ReplyInput{Message: "hello"}, nil)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("add reply fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { return errBoom }
		_, err := e.service().ReplyMine(context.Background(), 7, 11, tickets.ReplyInput{Message: "hello"}, nil)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("update fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		e.tickets.UpdateFn = func(ctx context.Context, t *domain.Ticket) error { return errBoom }
		_, err := e.service().ReplyMine(context.Background(), 7, 11, tickets.ReplyInput{Message: "hello"}, nil)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestCloseMineUpdateError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	e.tickets.UpdateFn = func(ctx context.Context, t *domain.Ticket) error { return errBoom }
	_, err := e.service().CloseMine(context.Background(), 7, 11)
	require.ErrorIs(t, err, errBoom)
}

func TestAssignErrors(t *testing.T) {
	t.Run("user lookup fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, errBoom }
		_, err := e.service().Assign(context.Background(), 1, 11, tickets.AssignInput{AssignedUserID: 55})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("nil user rejected", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		_, err := e.service().Assign(context.Background(), 1, 11, tickets.AssignInput{AssignedUserID: 55})
		requireCode(t, err, apperr.CodeValidation)
	})
	t.Run("update fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleStaff}, nil
		}
		e.tickets.UpdateFn = func(ctx context.Context, t *domain.Ticket) error { return errBoom }
		_, err := e.service().Assign(context.Background(), 1, 11, tickets.AssignInput{AssignedUserID: 55})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("negative id validation", func(t *testing.T) {
		_, err := newFixture().service().Assign(context.Background(), 1, 11, tickets.AssignInput{AssignedUserID: -1})
		requireCode(t, err, apperr.CodeValidation)
	})
}

func TestAdminReplyErrors(t *testing.T) {
	t.Run("actor lookup fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, errBoom }
		_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"}, nil)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("add reply fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleStaff, Email: "s@example.com"}, nil
		}
		e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { return errBoom }
		_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"}, nil)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("nil client id skips notification", func(t *testing.T) {
		e := newFixture()
		tk := openTicket()
		tk.ClientID = nil
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return tk, nil }
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleAdmin, Email: "a@example.com"}, nil
		}
		sent := false
		e.notifier.SendTemplateFn = func(ctx context.Context, userID int64, key string, data map[string]any) error {
			sent = true
			return nil
		}
		_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"}, nil)
		require.NoError(t, err)
		assert.False(t, sent)
	})
	t.Run("nil actor falls back to Staff author", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
			return activeDept(), nil
		}
		var reply *domain.TicketReply
		e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { reply = r; return nil }
		e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
		_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "Staff", reply.AuthorName)
	})
}

func TestSetStatusUpdateError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	e.tickets.UpdateFn = func(ctx context.Context, t *domain.Ticket) error { return errBoom }
	_, err := e.service().SetStatus(context.Background(), 1, 11, tickets.SetStatusInput{Status: "closed"})
	require.ErrorIs(t, err, errBoom)
}

func TestAttachmentURLErrors(t *testing.T) {
	t.Run("replies fail", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.tickets.ListRepliesFn = func(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.TicketReply, error) {
			return nil, errBoom
		}
		_, err := e.service().AttachmentURL(context.Background(), 7, 11, 0)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("presign fails", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		e.tickets.ListRepliesFn = func(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.TicketReply, error) {
			return repliesWithAttachments(), nil
		}
		e.storage.PresignGetFn = func(ctx context.Context, key string, ttl time.Duration, opts ...ports.PresignOption) (string, error) {
			return "", errBoom
		}
		_, err := e.service().AttachmentURL(context.Background(), 7, 11, 0)
		requireCode(t, err, apperr.CodeInternal)
	})
	t.Run("negative idx", func(t *testing.T) {
		e := newFixture()
		e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
		_, err := e.service().AttachmentURL(context.Background(), 7, 11, -1)
		requireCode(t, err, apperr.CodeNotFound)
	})
}

func TestDepartmentRepoErrors(t *testing.T) {
	e := newFixture()
	svc := e.service()

	e.tickets.CreateDepartmentFn = func(ctx context.Context, d *domain.TicketDepartment) error { return errBoom }
	_, err := svc.CreateDepartment(context.Background(), 1, tickets.DepartmentInput{Name: "Sales"})
	require.ErrorIs(t, err, errBoom)

	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		return nil, errBoom
	}
	_, err = svc.UpdateDepartment(context.Background(), 1, 3, tickets.DepartmentInput{Name: "Sales"})
	require.ErrorIs(t, err, errBoom)
	err = svc.DeleteDepartment(context.Background(), 1, 3)
	require.ErrorIs(t, err, errBoom)

	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		return &domain.TicketDepartment{ID: id, Name: "Support", Active: true}, nil
	}
	e.tickets.UpdateDepartmentFn = func(ctx context.Context, d *domain.TicketDepartment) error { return errBoom }
	_, err = svc.UpdateDepartment(context.Background(), 1, 3, tickets.DepartmentInput{Name: "Sales"})
	require.ErrorIs(t, err, errBoom)

	_, err = svc.UpdateDepartment(context.Background(), 1, 3, tickets.DepartmentInput{Name: "x"})
	requireCode(t, err, apperr.CodeValidation)
}

func TestSanitizedObjectKeyStripsPaths(t *testing.T) {
	e := newFixture()
	e.tickets.GetDepartmentByIDFn = func(ctx context.Context, id int64) (*domain.TicketDepartment, error) {
		return activeDept(), nil
	}
	e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
	e.tickets.NextNumberFn = func(ctx context.Context, scope string) (int64, error) { return 1, nil }
	var key string
	e.storage.PutFn = func(ctx context.Context, k string, r io.Reader, size int64, contentType string) error {
		key = k
		return nil
	}
	_, err := e.service().CreateTicket(context.Background(), 7,
		tickets.CreateTicketInput{DepartmentID: 3, Subject: "subject ok", Message: "message ok"},
		[]tickets.AttachmentUpload{upload(`..\..\dir/pass wd.png`, "x")})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(key, "-pass_wd.png"), key)
	assert.NotContains(t, key, "..")
}

func TestReplyMineTicketLookupError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().ReplyMine(context.Background(), 7, 11, tickets.ReplyInput{Message: "hello"}, nil)
	require.ErrorIs(t, err, errBoom)
}

func TestReplyMineStorageFailureAbortsTx(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	e.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) { return testClient(), nil }
	e.storage.PutFn = func(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
		return errors.New("s3 down")
	}
	replied := false
	e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { replied = true; return nil }
	_, err := e.service().ReplyMine(context.Background(), 7, 11, tickets.ReplyInput{Message: "hello"},
		[]tickets.AttachmentUpload{upload("a.png", "x")})
	requireCode(t, err, apperr.CodeInternal)
	assert.False(t, replied, "reply must not be written when upload fails")
}

func TestAdminGetTicketLookupError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().AdminGet(context.Background(), 11)
	require.ErrorIs(t, err, errBoom)
}

func TestAssignTicketLookupError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().Assign(context.Background(), 1, 11, tickets.AssignInput{AssignedUserID: 55})
	require.ErrorIs(t, err, errBoom)
}

func TestAdminReplyValidationError(t *testing.T) {
	_, err := newFixture().service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: ""}, nil)
	requireCode(t, err, apperr.CodeValidation)
}

func TestAdminReplyTicketLookupError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"}, nil)
	require.ErrorIs(t, err, errBoom)
}

func TestAdminReplyAttachmentValidationError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"},
		[]tickets.AttachmentUpload{upload("bad.exe", "x")})
	requireCode(t, err, apperr.CodeValidation)
}

func TestAdminReplyStorageFailureAbortsTx(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return openTicket(), nil }
	e.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: id, Role: domain.RoleStaff, Email: "s@example.com"}, nil
	}
	e.storage.PutFn = func(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
		return errors.New("s3 down")
	}
	replied := false
	e.tickets.AddReplyFn = func(ctx context.Context, r *domain.TicketReply) error { replied = true; return nil }
	_, err := e.service().AdminReply(context.Background(), 5, 11, tickets.ReplyInput{Message: "hi there"},
		[]tickets.AttachmentUpload{upload("a.png", "x")})
	requireCode(t, err, apperr.CodeInternal)
	assert.False(t, replied, "reply must not be written when upload fails")
}

func TestSetStatusTicketLookupError(t *testing.T) {
	e := newFixture()
	e.tickets.GetByIDFn = func(ctx context.Context, id int64) (*domain.Ticket, error) { return nil, errBoom }
	_, err := e.service().SetStatus(context.Background(), 1, 11, tickets.SetStatusInput{Status: "closed"})
	require.ErrorIs(t, err, errBoom)
}
