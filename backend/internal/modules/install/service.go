// Package install is the app-level phase of the installation wizard
// (docs/CONTRACTS.md §15): reached once config/DB are up (unlike
// internal/installer's pre-boot bootstrap phase). It creates the first admin
// account and, optionally, a handful of site settings - nothing else, and
// nothing here can ever run again once an admin exists.
package install

import (
	"context"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/service/settings"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// SettingsUpdater is the subset of settings.Service used here (an interface
// so tests can fake it without a real SettingsRepo/TxManager/AuditLogger).
type SettingsUpdater interface {
	Update(ctx context.Context, actorUserID int64, updates map[string]any) error
}

var _ SettingsUpdater = (*settings.Service)(nil)

// Deps are the Service's collaborators.
type Deps struct {
	Users    ports.UserRepo
	Hasher   ports.PasswordHasher
	AuditLog ports.AuditLogger
	Settings SettingsUpdater
	Clock    ports.Clock
}

// Service implements the app-level installation steps.
type Service struct {
	d   Deps
	val *validate.Validator
}

// New builds the install Service.
func New(d Deps) *Service { return &Service{d: d, val: validate.New()} }

// Status reports whether an admin account already exists.
func (s *Service) Status(ctx context.Context) (*StatusResponse, error) {
	installed, err := s.d.Users.ExistsAnyAdmin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &StatusResponse{ConfigReady: true, Installed: installed}, nil
}

// CreateAdmin creates the first admin account. Re-checks ExistsAnyAdmin on
// every call (not just once at startup) so this can never be used to add a
// second admin after the real install has already happened.
func (s *Service) CreateAdmin(ctx context.Context, in CreateAdminRequest) (*domain.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}

	installed, err := s.d.Users.ExistsAnyAdmin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if installed {
		return nil, apperr.Conflict("already installed")
	}

	hash, err := s.d.Hasher.Hash(in.Password)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.d.Clock.Now()
	user := &domain.User{
		Email:           in.Email,
		PasswordHash:    hash,
		Role:            domain.RoleAdmin,
		Status:          domain.UserActive,
		Locale:          "id",
		EmailVerifiedAt: &now,
	}
	if err := s.d.Users.Create(ctx, user); err != nil {
		if e := apperr.From(err); e.Code != apperr.CodeInternal {
			return nil, e
		}
		return nil, apperr.Internal(err)
	}

	s.d.AuditLog.Log(ctx, user.ID, "install.create_admin", "user", user.ID, nil, map[string]any{
		"email": user.Email,
	})
	return user, nil
}

// SaveSettings writes the optional first-run site settings. Blank fields are
// left untouched. Never gates "installed" - safe to call, skip, or retry.
func (s *Service) SaveSettings(ctx context.Context, actorUserID int64, in SiteSettingsRequest) error {
	if err := s.val.Struct(in); err != nil {
		return err
	}
	updates := map[string]any{}
	if in.CompanyName != "" {
		updates["company.name"] = in.CompanyName
	}
	if in.CompanyEmail != "" {
		updates["company.email"] = in.CompanyEmail
	}
	if in.CompanyAddress != "" {
		updates["company.address"] = in.CompanyAddress
	}
	if len(updates) == 0 {
		return nil
	}
	return s.d.Settings.Update(ctx, actorUserID, updates)
}
