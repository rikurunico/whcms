// Package announcements implements the M-ANNOUNCEMENTS module: portal
// announcements / news posts with a published lifecycle. Draft posts are
// admin-only; published posts (with a publish time in the past) surface on the
// public portal.
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET    /announcements                published, paginated, newest first
//	GET    /announcements/:slug          single published announcement
//	GET    /admin/announcements          ?search&status=published|draft   [perm: announcements]
//	POST   /admin/announcements                                           [perm: announcements]
//	GET    /admin/announcements/:id                                       [perm: announcements]
//	PATCH  /admin/announcements/:id                                       [perm: announcements]
//	DELETE /admin/announcements/:id       soft delete                     [perm: announcements]
package announcements

import (
	"context"
	"errors"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// Deps are the service dependencies (wired by the composition root).
type Deps struct {
	Repo  ports.AnnouncementRepo
	Audit ports.AuditLogger
	Clock ports.Clock
}

// Service implements the announcements use-cases.
type Service struct {
	d   Deps
	val *validate.Validator
}

// New builds the announcements Service.
func New(d Deps) *Service {
	return &Service{d: d, val: validate.New()}
}

// Helpers

// wrap passes *apperr.Error through and wraps anything else as INTERNAL.
func wrap(err error) error {
	if err == nil {
		return nil
	}
	var e *apperr.Error
	if errors.As(err, &e) {
		return e
	}
	return apperr.Internal(err)
}

// slugify lowercases s and replaces every non-alphanumeric run with '-'.
func slugify(s string) string {
	var b strings.Builder
	prevDash := true // trims leading dashes
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// getEntity loads one non-deleted announcement or NOT_FOUND.
func (s *Service) getEntity(ctx context.Context, id int64) (*domain.Announcement, error) {
	a, err := s.d.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	if a == nil {
		return nil, apperr.NotFound("announcement")
	}
	return a, nil
}

// Public

// ListPublished returns published announcements (newest first, paginated).
func (s *Service) ListPublished(ctx context.Context, p ports.ListParams) ([]AnnouncementResponse, int64, error) {
	rows, total, err := s.d.Repo.ListPublished(ctx, p)
	if err != nil {
		return nil, 0, wrap(err)
	}
	return toResponses(rows), total, nil
}

// GetPublishedBySlug returns one published announcement. Draft, deleted,
// absent or not-yet-published announcements are NOT_FOUND.
func (s *Service) GetPublishedBySlug(ctx context.Context, slug string) (*AnnouncementResponse, error) {
	a, err := s.d.Repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, wrap(err)
	}
	if a == nil || a.DeletedAt != nil || !a.Published ||
		a.PublishedAt == nil || a.PublishedAt.After(s.d.Clock.Now()) {
		return nil, apperr.NotFound("announcement")
	}
	r := toResponse(*a)
	return &r, nil
}

// Admin

// List lists announcements (admin; search on title/slug, status published|draft).
func (s *Service) List(ctx context.Context, p ports.ListParams) ([]AnnouncementResponse, int64, error) {
	rows, total, err := s.d.Repo.List(ctx, p)
	if err != nil {
		return nil, 0, wrap(err)
	}
	return toResponses(rows), total, nil
}

// Get returns one announcement (admin).
func (s *Service) Get(ctx context.Context, id int64) (*AnnouncementResponse, error) {
	a, err := s.getEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	r := toResponse(*a)
	return &r, nil
}

// Create creates an announcement. The slug is derived from the title when
// omitted; publishing stamps published_at (via the injected clock).
func (s *Service) Create(ctx context.Context, actorUserID int64, in AnnouncementInput) (*AnnouncementResponse, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Title)
	}
	if slug == "" {
		return nil, apperr.Validation("invalid slug",
			apperr.FieldError{Field: "slug", Message: "cannot be derived from title"})
	}
	a := &domain.Announcement{
		Title:     in.Title,
		Slug:      slug,
		Body:      in.Body,
		Published: in.Published,
	}
	if actorUserID != 0 {
		a.AuthorID = &actorUserID
	}
	if in.Published {
		now := s.d.Clock.Now()
		a.PublishedAt = &now
	}
	if err := s.d.Repo.Create(ctx, a); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "announcements.create", "announcement", a.ID, nil, a)
	r := toResponse(*a)
	return &r, nil
}

// Update patches an announcement. Publishing for the first time stamps
// published_at; un-publishing keeps the original publish time as history.
func (s *Service) Update(ctx context.Context, actorUserID, id int64, in AnnouncementUpdateInput) (*AnnouncementResponse, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	a, err := s.getEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *a
	if in.Title != nil {
		a.Title = *in.Title
	}
	if in.Slug != nil {
		a.Slug = *in.Slug
	}
	if in.Body != nil {
		a.Body = *in.Body
	}
	if in.Published != nil {
		a.Published = *in.Published
		if *in.Published && a.PublishedAt == nil {
			now := s.d.Clock.Now()
			a.PublishedAt = &now
		}
	}
	if err := s.d.Repo.Update(ctx, a); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "announcements.update", "announcement", a.ID, before, a)
	r := toResponse(*a)
	return &r, nil
}

// Delete soft-deletes an announcement.
func (s *Service) Delete(ctx context.Context, actorUserID, id int64) error {
	a, err := s.getEntity(ctx, id)
	if err != nil {
		return err
	}
	if err := s.d.Repo.SoftDelete(ctx, id); err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "announcements.delete", "announcement", id, a, nil)
	return nil
}
