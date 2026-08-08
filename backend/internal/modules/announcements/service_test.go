package announcements_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/announcements"
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
	repo  *mocks.MockAnnouncementRepo
	audit *mocks.MockAuditLogger
	now   time.Time
	svc   *announcements.Service
}

func newFixture() *fixtures {
	f := &fixtures{
		repo:  &mocks.MockAnnouncementRepo{},
		audit: &mocks.MockAuditLogger{},
		now:   time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC),
	}
	f.svc = announcements.New(announcements.Deps{
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

// announcement builds a published domain announcement (publish time in the
// past relative to the fixed clock) for reuse.
func announcement(mut func(*domain.Announcement)) *domain.Announcement {
	published := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	a := &domain.Announcement{
		ID:          7,
		Title:       "Maintenance",
		Slug:        "maintenance",
		Body:        "We will be down.",
		Published:   true,
		PublishedAt: &published,
	}
	if mut != nil {
		mut(a)
	}
	return a
}

// Public

func TestListPublished(t *testing.T) {
	f := newFixture()
	var gotParams ports.ListParams
	f.repo.ListPublishedFn = func(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
		gotParams = p
		return []domain.Announcement{*announcement(nil)}, 1, nil
	}
	rows, total, err := f.svc.ListPublished(context.Background(), ports.ListParams{Page: 2, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "maintenance", rows[0].Slug)
	assert.Equal(t, 2, gotParams.Page)
}

func TestListPublishedEmptyReturnsNonNilSlice(t *testing.T) {
	f := newFixture()
	rows, total, err := f.svc.ListPublished(context.Background(), ports.ListParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.NotNil(t, rows)
	assert.Empty(t, rows)
}

func TestGetPublishedBySlug(t *testing.T) {
	f := newFixture()
	f.repo.GetBySlugFn = func(ctx context.Context, slug string) (*domain.Announcement, error) {
		return announcement(func(a *domain.Announcement) { a.Slug = slug }), nil
	}
	got, err := f.svc.GetPublishedBySlug(context.Background(), "maintenance")
	require.NoError(t, err)
	assert.Equal(t, "maintenance", got.Slug)
	assert.True(t, got.Published)
}

func TestGetPublishedBySlugNotFoundBranches(t *testing.T) {
	future := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	deleted := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		row  *domain.Announcement
	}{
		{"absent", nil},
		{"deleted", announcement(func(a *domain.Announcement) { a.DeletedAt = &deleted })},
		{"draft", announcement(func(a *domain.Announcement) { a.Published = false })},
		{"published without published_at", announcement(func(a *domain.Announcement) { a.PublishedAt = nil })},
		{"published in the future", announcement(func(a *domain.Announcement) { a.PublishedAt = &future })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.repo.GetBySlugFn = func(ctx context.Context, slug string) (*domain.Announcement, error) {
				return tt.row, nil
			}
			_, err := f.svc.GetPublishedBySlug(context.Background(), "x")
			assertCode(t, err, apperr.CodeNotFound)
		})
	}
}

// Admin

func TestList(t *testing.T) {
	f := newFixture()
	f.repo.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
		return []domain.Announcement{{ID: 1}, {ID: 2}}, 2, nil
	}
	rows, total, err := f.svc.List(context.Background(), ports.ListParams{})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, int64(2), total)
}

func TestGet(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return announcement(func(a *domain.Announcement) { a.ID = id }), nil
	}
	got, err := f.svc.Get(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.ID)
}

func TestGetNotFound(t *testing.T) {
	f := newFixture()
	// MockAnnouncementRepo returns (nil, nil) by default.
	_, err := f.svc.Get(context.Background(), 99)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestCreateGeneratesSlugAndAudits(t *testing.T) {
	f := newFixture()
	var created *domain.Announcement
	f.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		a.ID = 5
		created = a
		return nil
	}
	got, err := f.svc.Create(context.Background(), 42, announcements.AnnouncementInput{
		Title: "New Data Center!",
		Body:  "hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "new-data-center", created.Slug)
	assert.Equal(t, int64(5), got.ID)
	assert.False(t, created.Published)
	assert.Nil(t, created.PublishedAt, "draft has no publish time")
	require.NotNil(t, created.AuthorID)
	assert.Equal(t, int64(42), *created.AuthorID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "announcements.create", f.audit.Entries[0].Action)
	assert.Equal(t, "announcement", f.audit.Entries[0].Entity)
	assert.Equal(t, int64(42), f.audit.Entries[0].ActorUserID)
}

func TestCreateSanitizesBody(t *testing.T) {
	f := newFixture()
	var created *domain.Announcement
	f.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		created = a
		return nil
	}
	_, err := f.svc.Create(context.Background(), 1, announcements.AnnouncementInput{
		Title: "Payload",
		Body:  `<p>hi</p><script>alert(document.cookie)</script>`,
	})
	require.NoError(t, err)
	assert.Equal(t, "<p>hi</p>", created.Body)
}

func TestCreateWithExplicitSlug(t *testing.T) {
	f := newFixture()
	var created *domain.Announcement
	f.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		created = a
		return nil
	}
	_, err := f.svc.Create(context.Background(), 1, announcements.AnnouncementInput{
		Title: "Anything", Slug: "custom-slug",
	})
	require.NoError(t, err)
	assert.Equal(t, "custom-slug", created.Slug)
}

func TestCreatePublishedStampsPublishedAt(t *testing.T) {
	f := newFixture()
	var created *domain.Announcement
	f.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		created = a
		return nil
	}
	_, err := f.svc.Create(context.Background(), 1, announcements.AnnouncementInput{
		Title: "Live now", Published: true,
	})
	require.NoError(t, err)
	assert.True(t, created.Published)
	require.NotNil(t, created.PublishedAt)
	assert.Equal(t, f.now, *created.PublishedAt)
}

func TestCreateAnonymousActorHasNoAuthor(t *testing.T) {
	f := newFixture()
	var created *domain.Announcement
	f.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		created = a
		return nil
	}
	_, err := f.svc.Create(context.Background(), 0, announcements.AnnouncementInput{Title: "System note"})
	require.NoError(t, err)
	assert.Nil(t, created.AuthorID)
}

func TestCreateValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Create(context.Background(), 1, announcements.AnnouncementInput{Title: "x"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestCreateSlugCannotBeDerived(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Create(context.Background(), 1, announcements.AnnouncementInput{Title: "!!!"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdatePatchesFields(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Old", Slug: "old", Body: "old body"}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	got, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{
		Title: ptr("New Title"),
		Body:  ptr("new body"),
	})
	require.NoError(t, err)
	assert.Equal(t, "New Title", saved.Title)
	assert.Equal(t, "new body", saved.Body)
	assert.Equal(t, "old", saved.Slug, "unset fields unchanged")
	assert.Equal(t, "New Title", got.Title)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "announcements.update", f.audit.Entries[0].Action)
}

func TestUpdateSanitizesBody(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Old", Slug: "old", Body: "old body"}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{
		Body: ptr(`<img src=x onerror=alert(1)>`),
	})
	require.NoError(t, err)
	assert.Equal(t, `<img src="x">`, saved.Body)
}

func TestUpdatePublishingStampsPublishedAt(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Draft", Slug: "draft", Published: false}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{
		Published: ptr(true),
	})
	require.NoError(t, err)
	assert.True(t, saved.Published)
	require.NotNil(t, saved.PublishedAt)
	assert.Equal(t, f.now, *saved.PublishedAt)
}

func TestUpdateUnpublishKeepsHistory(t *testing.T) {
	f := newFixture()
	original := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Live", Slug: "live", Published: true, PublishedAt: &original}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{
		Published: ptr(false),
	})
	require.NoError(t, err)
	assert.False(t, saved.Published)
	require.NotNil(t, saved.PublishedAt)
	assert.Equal(t, original, *saved.PublishedAt, "un-publishing keeps the original publish time")
}

func TestUpdateRepublishKeepsOriginalPublishedAt(t *testing.T) {
	f := newFixture()
	original := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Was live", Slug: "s", Published: false, PublishedAt: &original}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{
		Published: ptr(true),
	})
	require.NoError(t, err)
	assert.True(t, saved.Published)
	require.NotNil(t, saved.PublishedAt)
	assert.Equal(t, original, *saved.PublishedAt, "re-publishing keeps the first publish time")
}

func TestUpdateSlug(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "T", Slug: "old"}, nil
	}
	var saved *domain.Announcement
	f.repo.UpdateFn = func(ctx context.Context, a *domain.Announcement) error {
		saved = a
		return nil
	}
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{Slug: ptr("new")})
	require.NoError(t, err)
	assert.Equal(t, "new", saved.Slug)
}

func TestUpdateNotFound(t *testing.T) {
	f := newFixture()
	// default mock returns (nil, nil) -> NOT_FOUND.
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{Title: ptr("New")})
	assertCode(t, err, apperr.CodeNotFound)
}

func TestUpdateValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Update(context.Background(), 1, 7, announcements.AnnouncementUpdateInput{Title: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
}

func TestDeleteSoftDeletesAndAudits(t *testing.T) {
	f := newFixture()
	f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Bye"}, nil
	}
	var deletedID int64
	f.repo.SoftDeleteFn = func(ctx context.Context, id int64) error {
		deletedID = id
		return nil
	}
	require.NoError(t, f.svc.Delete(context.Background(), 1, 7))
	assert.Equal(t, int64(7), deletedID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "announcements.delete", f.audit.Entries[0].Action)
}

func TestDeleteNotFound(t *testing.T) {
	f := newFixture()
	// default mock returns (nil, nil) -> NOT_FOUND before SoftDelete is called.
	deleted := false
	f.repo.SoftDeleteFn = func(ctx context.Context, id int64) error {
		deleted = true
		return nil
	}
	err := f.svc.Delete(context.Background(), 1, 7)
	assertCode(t, err, apperr.CodeNotFound)
	assert.False(t, deleted)
	assert.Empty(t, f.audit.Entries)
}

var errBoom = errors.New("boom")

// TestServiceErrorPropagation drives every repo failure branch: unexpected
// repo errors surface as INTERNAL apperr.
func TestServiceErrorPropagation(t *testing.T) {
	ctx := context.Background()

	existing := func(f *fixtures) {
		f.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
			return &domain.Announcement{ID: id, Title: "T", Slug: "s"}, nil
		}
	}

	tests := []struct {
		name string
		prep func(f *fixtures)
		call func(f *fixtures) error
	}{
		{"ListPublished", func(f *fixtures) {
			f.repo.ListPublishedFn = func(context.Context, ports.ListParams) ([]domain.Announcement, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, _, err := f.svc.ListPublished(ctx, ports.ListParams{}); return err }},
		{"GetPublishedBySlug", func(f *fixtures) {
			f.repo.GetBySlugFn = func(context.Context, string) (*domain.Announcement, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetPublishedBySlug(ctx, "s"); return err }},
		{"List", func(f *fixtures) {
			f.repo.ListFn = func(context.Context, ports.ListParams) ([]domain.Announcement, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, _, err := f.svc.List(ctx, ports.ListParams{}); return err }},
		{"Get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.Get(ctx, 1); return err }},
		{"Create", func(f *fixtures) {
			f.repo.CreateFn = func(context.Context, *domain.Announcement) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Create(ctx, 1, announcements.AnnouncementInput{Title: "A title"})
			return err
		}},
		{"Update get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Update(ctx, 1, 2, announcements.AnnouncementUpdateInput{Title: ptr("A title")})
			return err
		}},
		{"Update save", func(f *fixtures) {
			existing(f)
			f.repo.UpdateFn = func(context.Context, *domain.Announcement) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.Update(ctx, 1, 2, announcements.AnnouncementUpdateInput{Title: ptr("A title")})
			return err
		}},
		{"Delete get", func(f *fixtures) {
			f.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
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
	f.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) {
		return nil, apperr.NotFound("announcement")
	}
	_, err := f.svc.Get(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
}
