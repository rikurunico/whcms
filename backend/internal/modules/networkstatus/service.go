// Package networkstatus implements the network status module: scheduled
// maintenance, incidents and outages shown on the public status page plus the
// admin CRUD behind them.
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET    /network-status                  public, active/recent issues (rate limited)
//	GET    /network-status/:id              public, one issue by id
//	GET    /admin/network-issues            ?search&status (paginated)          [perm: network]
//	POST   /admin/network-issues                                                [perm: network]
//	GET    /admin/network-issues/:id                                            [perm: network]
//	PATCH  /admin/network-issues/:id                                            [perm: network]
//	DELETE /admin/network-issues/:id        soft delete                         [perm: network]
package networkstatus

import (
	"context"
	"errors"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// Deps are the service dependencies (wired by the composition root).
type Deps struct {
	Repo  ports.NetworkStatusRepo
	Audit ports.AuditLogger
	Clock ports.Clock
}

// Service implements the network status use-cases.
type Service struct {
	d   Deps
	val *validate.Validator
}

// New builds the network status Service.
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

// checkEnums validates the type/severity/status enum triple via their Valid()
// methods, collecting every offending field into one VALIDATION error.
func checkEnums(typ domain.NetworkIssueType, sev domain.NetworkIssueSeverity, st domain.NetworkIssueStatus) error {
	var details []apperr.FieldError
	if !typ.Valid() {
		details = append(details, apperr.FieldError{Field: "type", Message: "unknown network issue type"})
	}
	if !sev.Valid() {
		details = append(details, apperr.FieldError{Field: "severity", Message: "unknown severity"})
	}
	if !st.Valid() {
		details = append(details, apperr.FieldError{Field: "status", Message: "unknown status"})
	}
	if len(details) > 0 {
		return apperr.Validation("invalid network issue", details...)
	}
	return nil
}

// Public

// PublicList returns the active/recent status entries (unresolved, plus
// anything resolved within the last week), newest first.
func (s *Service) PublicList(ctx context.Context) ([]domain.NetworkIssue, error) {
	rows, err := s.d.Repo.ListActive(ctx)
	if err != nil {
		return nil, wrap(err)
	}
	if rows == nil {
		rows = []domain.NetworkIssue{}
	}
	return rows, nil
}

// Admin

// List returns a page of status entries (admin; search on title/body/affected,
// status filters an exact status value).
func (s *Service) List(ctx context.Context, p ports.ListParams) ([]domain.NetworkIssue, int64, error) {
	rows, total, err := s.d.Repo.List(ctx, p)
	if err != nil {
		return nil, 0, wrap(err)
	}
	if rows == nil {
		rows = []domain.NetworkIssue{}
	}
	return rows, total, nil
}

// Get returns one non-deleted status entry (shared by the public and admin
// detail routes).
func (s *Service) Get(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
	n, err := s.d.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	if n == nil {
		return nil, apperr.NotFound("network issue")
	}
	return n, nil
}

// Create creates a status entry. Empty enum fields fall back to defaults,
// StartsAt defaults to now, and a resolved entry gets EndsAt = now if unset.
func (s *Service) Create(ctx context.Context, actorUserID int64, in IssueInput) (*domain.NetworkIssue, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	typ := domain.NetworkIssueType(in.Type)
	if typ == "" {
		typ = domain.NetworkTypeIssue
	}
	sev := domain.NetworkIssueSeverity(in.Severity)
	if sev == "" {
		sev = domain.NetworkSeverityMinor
	}
	st := domain.NetworkIssueStatus(in.Status)
	if st == "" {
		st = domain.NetworkStatusInvestigating
	}
	if err := checkEnums(typ, sev, st); err != nil {
		return nil, err
	}
	n := &domain.NetworkIssue{
		Title:    in.Title,
		Body:     in.Body,
		Type:     typ,
		Severity: sev,
		Status:   st,
		Affected: in.Affected,
	}
	if in.StartsAt != nil && !in.StartsAt.IsZero() {
		n.StartsAt = *in.StartsAt
	} else {
		n.StartsAt = s.d.Clock.Now()
	}
	if in.EndsAt != nil {
		n.EndsAt = in.EndsAt
	}
	if n.Status == domain.NetworkStatusResolved && n.EndsAt == nil {
		now := s.d.Clock.Now()
		n.EndsAt = &now
	}
	if err := s.d.Repo.Create(ctx, n); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "networkstatus.issue.create", "network_issue", n.ID, nil, n)
	return n, nil
}

// Update patches a status entry. Moving it to resolved fills EndsAt with now
// when it is still unset.
func (s *Service) Update(ctx context.Context, actorUserID, id int64, in IssueUpdateInput) (*domain.NetworkIssue, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	n, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *n
	if in.Title != nil {
		n.Title = *in.Title
	}
	if in.Body != nil {
		n.Body = *in.Body
	}
	if in.Type != nil {
		n.Type = domain.NetworkIssueType(*in.Type)
	}
	if in.Severity != nil {
		n.Severity = domain.NetworkIssueSeverity(*in.Severity)
	}
	if in.Status != nil {
		n.Status = domain.NetworkIssueStatus(*in.Status)
	}
	if in.Affected != nil {
		n.Affected = *in.Affected
	}
	if in.StartsAt != nil && !in.StartsAt.IsZero() {
		n.StartsAt = *in.StartsAt
	}
	if in.ClearEnds {
		n.EndsAt = nil
	} else if in.EndsAt != nil {
		n.EndsAt = in.EndsAt
	}
	if err := checkEnums(n.Type, n.Severity, n.Status); err != nil {
		return nil, err
	}
	if n.Status == domain.NetworkStatusResolved && n.EndsAt == nil {
		now := s.d.Clock.Now()
		n.EndsAt = &now
	}
	if err := s.d.Repo.Update(ctx, n); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "networkstatus.issue.update", "network_issue", n.ID, before, n)
	return n, nil
}

// Delete soft-deletes a status entry.
func (s *Service) Delete(ctx context.Context, actorUserID, id int64) error {
	n, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.d.Repo.SoftDelete(ctx, id); err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "networkstatus.issue.delete", "network_issue", id, n, nil)
	return nil
}
