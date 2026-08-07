package networkstatus_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/networkstatus"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures

// fixtures bundles the fakes plus the service under test.
type fixtures struct {
	repo  *mocks.MockNetworkStatusRepo
	audit *mocks.MockAuditLogger
	now   time.Time
	svc   *networkstatus.Service
}

func newFixture() *fixtures {
	f := &fixtures{
		repo:  &mocks.MockNetworkStatusRepo{},
		audit: &mocks.MockAuditLogger{},
		now:   time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC),
	}
	f.svc = networkstatus.New(networkstatus.Deps{
		Repo:  f.repo,
		Audit: f.audit,
		Clock: &mocks.MockClock{FixedTime: f.now},
	})
	return f
}

func assertCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, code, e.Code)
}

func ptr[T any](v T) *T { return &v }

// Public

func TestPublicListReturnsRows(t *testing.T) {
	f := newFixture()
	f.repo.ListActiveFn = func(ctx context.Context) ([]domain.NetworkIssue, error) {
		return []domain.NetworkIssue{{ID: 1, Title: "Outage", Status: domain.NetworkStatusInvestigating}}, nil
	}
	rows, err := f.svc.PublicList(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), rows[0].ID)
}

func TestPublicListEmptyIsNonNilSlice(t *testing.T) {
	f := newFixture()
	// Default mock returns nil,nil.
	rows, err := f.svc.PublicList(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
}

// Get

func TestGetReturnsIssue(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
		return &domain.NetworkIssue{ID: id, Title: "Issue"}, nil
	}
	n, err := f.svc.Get(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), n.ID)
}

func TestGetNilRowIsNotFound(t *testing.T) {
	f := newFixture()
	// Default mock returns nil,nil.
	_, err := f.svc.Get(context.Background(), 5)
	assertCode(t, err, apperr.CodeNotFound)
}

// List

func TestListReturnsRowsAndTotal(t *testing.T) {
	f := newFixture()
	f.repo.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.NetworkIssue, int64, error) {
		assert.Equal(t, "boom", p.Search)
		return []domain.NetworkIssue{{ID: 1}}, 1, nil
	}
	rows, total, err := f.svc.List(context.Background(), ports.ListParams{Search: "boom"})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

func TestListEmptyIsNonNilSlice(t *testing.T) {
	f := newFixture()
	// Default mock returns nil,0,nil.
	rows, total, err := f.svc.List(context.Background(), ports.ListParams{})
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
	assert.Zero(t, total)
}

// Create

func TestCreateDefaultsAndAudits(t *testing.T) {
	f := newFixture()
	var created *domain.NetworkIssue
	f.repo.CreateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		n.ID = 9
		created = n
		return nil
	}
	n, err := f.svc.Create(context.Background(), 42, networkstatus.IssueInput{Title: "DB outage"})
	require.NoError(t, err)
	assert.Equal(t, int64(9), n.ID)
	assert.Equal(t, domain.NetworkTypeIssue, created.Type)
	assert.Equal(t, domain.NetworkSeverityMinor, created.Severity)
	assert.Equal(t, domain.NetworkStatusInvestigating, created.Status)
	assert.Equal(t, f.now, created.StartsAt, "starts_at defaults to now")
	assert.Nil(t, created.EndsAt)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "networkstatus.issue.create", f.audit.Entries[0].Action)
	assert.Equal(t, "network_issue", f.audit.Entries[0].Entity)
	assert.Equal(t, int64(42), f.audit.Entries[0].ActorUserID)
}

func TestCreateHonorsExplicitFields(t *testing.T) {
	f := newFixture()
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	var created *domain.NetworkIssue
	f.repo.CreateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		created = n
		return nil
	}
	_, err := f.svc.Create(context.Background(), 1, networkstatus.IssueInput{
		Title:    "Planned maintenance",
		Body:     "Upgrading",
		Type:     "scheduled",
		Severity: "major",
		Status:   "scheduled",
		Affected: "web-01",
		StartsAt: &start,
		EndsAt:   &end,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.NetworkTypeScheduled, created.Type)
	assert.Equal(t, domain.NetworkSeverityMajor, created.Severity)
	assert.Equal(t, domain.NetworkStatusScheduled, created.Status)
	assert.Equal(t, "web-01", created.Affected)
	assert.Equal(t, start, created.StartsAt)
	require.NotNil(t, created.EndsAt)
	assert.Equal(t, end, *created.EndsAt)
}

func TestCreateResolvedSetsEndsAt(t *testing.T) {
	f := newFixture()
	var created *domain.NetworkIssue
	f.repo.CreateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		created = n
		return nil
	}
	_, err := f.svc.Create(context.Background(), 1, networkstatus.IssueInput{
		Title: "Already fixed", Status: "resolved",
	})
	require.NoError(t, err)
	require.NotNil(t, created.EndsAt, "resolved entry gets ends_at = now")
	assert.Equal(t, f.now, *created.EndsAt)
}

func TestCreateValidationEmptyTitle(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Create(context.Background(), 1, networkstatus.IssueInput{Title: "x"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestCreateInvalidEnums(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Create(context.Background(), 1, networkstatus.IssueInput{Title: "Bad type", Type: "meteor"})
	assertCode(t, err, apperr.CodeValidation)

	_, err = f.svc.Create(context.Background(), 1, networkstatus.IssueInput{Title: "Bad sev", Severity: "spicy"})
	assertCode(t, err, apperr.CodeValidation)

	_, err = f.svc.Create(context.Background(), 1, networkstatus.IssueInput{Title: "Bad status", Status: "on_fire"})
	assertCode(t, err, apperr.CodeValidation)
}

// Update

func issueFixture(f *fixtures) {
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
		start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		return &domain.NetworkIssue{
			ID: id, Title: "Old", Body: "b", Type: domain.NetworkTypeIssue,
			Severity: domain.NetworkSeverityMinor, Status: domain.NetworkStatusInvestigating,
			Affected: "a", StartsAt: start,
		}, nil
	}
}

func TestUpdatePatchesFields(t *testing.T) {
	f := newFixture()
	issueFixture(f)
	var saved *domain.NetworkIssue
	f.repo.UpdateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		saved = n
		return nil
	}
	newStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	n, err := f.svc.Update(context.Background(), 7, 3, networkstatus.IssueUpdateInput{
		Title:    ptr("New title"),
		Body:     ptr("new body"),
		Type:     ptr("outage"),
		Severity: ptr("critical"),
		Status:   ptr("identified"),
		Affected: ptr("web-02"),
		StartsAt: &newStart,
	})
	require.NoError(t, err)
	assert.Equal(t, "New title", saved.Title)
	assert.Equal(t, "new body", saved.Body)
	assert.Equal(t, domain.NetworkTypeOutage, saved.Type)
	assert.Equal(t, domain.NetworkSeverityCritical, saved.Severity)
	assert.Equal(t, domain.NetworkStatusIdentified, saved.Status)
	assert.Equal(t, "web-02", saved.Affected)
	assert.Equal(t, newStart, saved.StartsAt)
	assert.Equal(t, domain.NetworkStatusIdentified, n.Status)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "networkstatus.issue.update", f.audit.Entries[0].Action)
}

func TestUpdateResolvedSetsEndsAt(t *testing.T) {
	f := newFixture()
	issueFixture(f)
	var saved *domain.NetworkIssue
	f.repo.UpdateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		saved = n
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{Status: ptr("resolved")})
	require.NoError(t, err)
	require.NotNil(t, saved.EndsAt)
	assert.Equal(t, f.now, *saved.EndsAt)
}

func TestUpdateSetAndClearEndsAt(t *testing.T) {
	f := newFixture()
	issueFixture(f)
	end := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	var saved *domain.NetworkIssue
	f.repo.UpdateFn = func(ctx context.Context, n *domain.NetworkIssue) error {
		saved = n
		return nil
	}
	// Set ends_at explicitly.
	_, err := f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{EndsAt: &end})
	require.NoError(t, err)
	require.NotNil(t, saved.EndsAt)
	assert.Equal(t, end, *saved.EndsAt)

	// ClearEnds wins over EndsAt.
	_, err = f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{EndsAt: &end, ClearEnds: true})
	require.NoError(t, err)
	assert.Nil(t, saved.EndsAt)
}

func TestUpdateValidationBadField(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{Title: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdateInvalidEnum(t *testing.T) {
	f := newFixture()
	issueFixture(f)
	_, err := f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{Type: ptr("meteor")})
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdateNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
		return nil, apperr.NotFound("network issue")
	}
	_, err := f.svc.Update(context.Background(), 1, 3, networkstatus.IssueUpdateInput{Title: ptr("New title")})
	assertCode(t, err, apperr.CodeNotFound)
}

// Delete

func TestDeleteSoftDeletes(t *testing.T) {
	f := newFixture()
	issueFixture(f)
	var deletedID int64
	f.repo.SoftDeleteFn = func(ctx context.Context, id int64) error {
		deletedID = id
		return nil
	}
	require.NoError(t, f.svc.Delete(context.Background(), 1, 3))
	assert.Equal(t, int64(3), deletedID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "networkstatus.issue.delete", f.audit.Entries[0].Action)
}

func TestDeleteNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
		return nil, apperr.NotFound("network issue")
	}
	err := f.svc.Delete(context.Background(), 1, 3)
	assertCode(t, err, apperr.CodeNotFound)
	assert.Empty(t, f.audit.Entries)
}

var errBoom = errors.New("boom")

// TestServiceErrorPropagation drives every repo failure branch: unexpected
// repo errors surface as INTERNAL apperr.
func TestServiceErrorPropagation(t *testing.T) {
	ctx := context.Background()

	// existing makes Get succeed so the *save* branch of Update/Delete is what
	// actually fails.
	existing := func(f *fixtures) {
		f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
			return &domain.NetworkIssue{
				ID: id, Title: "T", Type: domain.NetworkTypeIssue,
				Severity: domain.NetworkSeverityMinor, Status: domain.NetworkStatusInvestigating,
			}, nil
		}
	}

	tests := []struct {
		name string
		prep func(f *fixtures)
		call func(f *fixtures) error
	}{
		{"PublicList", func(f *fixtures) {
			f.repo.ListActiveFn = func(context.Context) ([]domain.NetworkIssue, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicList(ctx); return err }},
		{"List", func(f *fixtures) {
			f.repo.ListFn = func(context.Context, ports.ListParams) ([]domain.NetworkIssue, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, _, err := f.svc.List(ctx, ports.ListParams{}); return err }},
		{"Get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.NetworkIssue, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.Get(ctx, 1); return err }},
		{"Create", func(f *fixtures) {
			f.repo.CreateFn = func(context.Context, *domain.NetworkIssue) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Create(ctx, 1, networkstatus.IssueInput{Title: "A title"})
			return err
		}},
		{"Update get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.NetworkIssue, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Update(ctx, 1, 2, networkstatus.IssueUpdateInput{Title: ptr("A title")})
			return err
		}},
		{"Update save", func(f *fixtures) {
			existing(f)
			f.repo.UpdateFn = func(context.Context, *domain.NetworkIssue) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Update(ctx, 1, 2, networkstatus.IssueUpdateInput{Title: ptr("A title")})
			return err
		}},
		{"Delete get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.NetworkIssue, error) { return nil, errBoom }
		}, func(f *fixtures) error { return f.svc.Delete(ctx, 1, 2) }},
		{"Delete soft-delete", func(f *fixtures) {
			existing(f)
			f.repo.SoftDeleteFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.Delete(ctx, 1, 2) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			tt.prep(f)
			assertCode(t, tt.call(f), apperr.CodeInternal)
		})
	}
}

// TestServiceApperrPassthrough confirms an apperr from the repo is passed
// through unchanged (not re-wrapped as INTERNAL).
func TestServiceApperrPassthrough(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.repo.GetByIDFn = func(context.Context, int64) (*domain.NetworkIssue, error) {
		return nil, apperr.NotFound("network issue")
	}
	_, err := f.svc.Get(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
}

// TestServiceInvalidEnumsRejected confirms each enum triple field is validated
// on both create (with defaults filled) and update paths -> VALIDATION.
func TestServiceInvalidEnumsRejected(t *testing.T) {
	ctx := context.Background()

	t.Run("create bad type", func(t *testing.T) {
		f := newFixture()
		_, err := f.svc.Create(ctx, 1, networkstatus.IssueInput{Title: "Bad", Type: "meteor"})
		assertCode(t, err, apperr.CodeValidation)
	})
	t.Run("create bad severity", func(t *testing.T) {
		f := newFixture()
		_, err := f.svc.Create(ctx, 1, networkstatus.IssueInput{Title: "Bad", Severity: "spicy"})
		assertCode(t, err, apperr.CodeValidation)
	})
	t.Run("create bad status", func(t *testing.T) {
		f := newFixture()
		_, err := f.svc.Create(ctx, 1, networkstatus.IssueInput{Title: "Bad", Status: "on_fire"})
		assertCode(t, err, apperr.CodeValidation)
	})
	t.Run("update bad enum", func(t *testing.T) {
		f := newFixture()
		issueFixture(f)
		_, err := f.svc.Update(ctx, 1, 3, networkstatus.IssueUpdateInput{Severity: ptr("spicy")})
		assertCode(t, err, apperr.CodeValidation)
	})
}

// Setting status to 'resolved' stamps ends_at=now on both create and
// update when ends_at is still unset.
func TestServiceResolvedSetsEndsAt(t *testing.T) {
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		f := newFixture()
		var created *domain.NetworkIssue
		f.repo.CreateFn = func(ctx context.Context, n *domain.NetworkIssue) error { created = n; return nil }
		_, err := f.svc.Create(ctx, 1, networkstatus.IssueInput{Title: "Fixed", Status: "resolved"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.EndsAt == nil || !created.EndsAt.Equal(f.now) {
			t.Fatalf("resolved create should set ends_at=now, got %v", created.EndsAt)
		}
	})

	t.Run("update", func(t *testing.T) {
		f := newFixture()
		issueFixture(f)
		var saved *domain.NetworkIssue
		f.repo.UpdateFn = func(ctx context.Context, n *domain.NetworkIssue) error { saved = n; return nil }
		_, err := f.svc.Update(ctx, 1, 3, networkstatus.IssueUpdateInput{Status: ptr("resolved")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if saved.EndsAt == nil || !saved.EndsAt.Equal(f.now) {
			t.Fatalf("resolved update should set ends_at=now, got %v", saved.EndsAt)
		}
	})
}
