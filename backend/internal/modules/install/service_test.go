package install_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/install"
	"github.com/tsdlamongan/whcms/backend/internal/platform/clock"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSettings is a minimal install.SettingsUpdater fake - install doesn't
// need a real settings.Service (with its own SettingsRepo/TxManager/
// AuditLogger) just to verify which keys it forwards.
type fakeSettings struct {
	UpdateFn func(ctx context.Context, actorUserID int64, updates map[string]any) error
	calls    []map[string]any
}

func (f *fakeSettings) Update(ctx context.Context, actorUserID int64, updates map[string]any) error {
	f.calls = append(f.calls, updates)
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, actorUserID, updates)
	}
	return nil
}

func newService(users *mocks.MockUserRepo, settingsFake *fakeSettings) *install.Service {
	return install.New(install.Deps{
		Users:    users,
		Hasher:   &mocks.MockPasswordHasher{},
		AuditLog: &mocks.MockAuditLogger{},
		Settings: settingsFake,
		Clock:    clock.Fixed{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
}

func TestStatus(t *testing.T) {
	for _, installed := range []bool{true, false} {
		users := &mocks.MockUserRepo{
			ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return installed, nil },
		}
		svc := newService(users, &fakeSettings{})
		resp, err := svc.Status(context.Background())
		require.NoError(t, err)
		assert.True(t, resp.ConfigReady)
		assert.Equal(t, installed, resp.Installed)
	}
}

func TestStatusPropagatesRepoError(t *testing.T) {
	users := &mocks.MockUserRepo{
		ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return false, assert.AnError },
	}
	svc := newService(users, &fakeSettings{})
	_, err := svc.Status(context.Background())
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

func TestCreateAdminRejectsInvalidInput(t *testing.T) {
	users := &mocks.MockUserRepo{
		ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return false, nil },
	}
	svc := newService(users, &fakeSettings{})
	_, err := svc.CreateAdmin(context.Background(), install.CreateAdminRequest{Email: "not-an-email", Password: "short"})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

func TestCreateAdminRejectsWhenAlreadyInstalled(t *testing.T) {
	var createCalled bool
	users := &mocks.MockUserRepo{
		ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return true, nil },
		CreateFn:         func(ctx context.Context, u *domain.User) error { createCalled = true; return nil },
	}
	svc := newService(users, &fakeSettings{})
	_, err := svc.CreateAdmin(context.Background(), install.CreateAdminRequest{
		Email: "admin@example.test", Password: "correcthorsebattery",
	})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.False(t, createCalled, "must never create a second admin once one exists")
}

func TestCreateAdminSucceeds(t *testing.T) {
	var created *domain.User
	users := &mocks.MockUserRepo{
		ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return false, nil },
		CreateFn: func(ctx context.Context, u *domain.User) error {
			u.ID = 42
			created = u
			return nil
		},
	}
	svc := newService(users, &fakeSettings{})
	user, err := svc.CreateAdmin(context.Background(), install.CreateAdminRequest{
		Email: "  Admin@Example.TEST  ", Password: "correcthorsebattery",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, int64(42), user.ID)
	assert.Equal(t, "admin@example.test", created.Email, "email is lowercased/trimmed")
	assert.Equal(t, domain.RoleAdmin, created.Role)
	assert.Equal(t, domain.UserActive, created.Status)
	require.NotNil(t, created.EmailVerifiedAt)
	assert.True(t, created.EmailVerifiedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.NotEmpty(t, created.PasswordHash)
}

func TestCreateAdminPropagatesCreateError(t *testing.T) {
	users := &mocks.MockUserRepo{
		ExistsAnyAdminFn: func(ctx context.Context) (bool, error) { return false, nil },
		CreateFn: func(ctx context.Context, u *domain.User) error {
			return apperr.Conflict("email already registered")
		},
	}
	svc := newService(users, &fakeSettings{})
	_, err := svc.CreateAdmin(context.Background(), install.CreateAdminRequest{
		Email: "admin@example.test", Password: "correcthorsebattery",
	})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestSaveSettingsSkipsWhenAllBlank(t *testing.T) {
	settingsFake := &fakeSettings{}
	svc := newService(&mocks.MockUserRepo{}, settingsFake)
	err := svc.SaveSettings(context.Background(), 0, install.SiteSettingsRequest{})
	require.NoError(t, err)
	assert.Empty(t, settingsFake.calls, "must not call Settings.Update when nothing was submitted")
}

func TestSaveSettingsForwardsOnlyNonBlankFields(t *testing.T) {
	settingsFake := &fakeSettings{}
	svc := newService(&mocks.MockUserRepo{}, settingsFake)
	err := svc.SaveSettings(context.Background(), 7, install.SiteSettingsRequest{
		CompanyName: "Acme Hosting",
	})
	require.NoError(t, err)
	require.Len(t, settingsFake.calls, 1)
	assert.Equal(t, map[string]any{"company.name": "Acme Hosting"}, settingsFake.calls[0])
}

func TestSaveSettingsPropagatesError(t *testing.T) {
	settingsFake := &fakeSettings{UpdateFn: func(ctx context.Context, actorUserID int64, updates map[string]any) error {
		return assert.AnError
	}}
	svc := newService(&mocks.MockUserRepo{}, settingsFake)
	err := svc.SaveSettings(context.Background(), 0, install.SiteSettingsRequest{CompanyEmail: "billing@acme.test"})
	assert.ErrorIs(t, err, assert.AnError)
}
