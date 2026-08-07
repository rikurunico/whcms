//go:build integration

// Integration tests against the real local Postgres (whmcs DB, migrations
// applied). Rows use uuid-suffixed keys and are cleaned up; seeded templates
// are read-only here. Run: go test -tags integration ./internal/modules/notifications/
package notifications_test

import (
	"context"
	"os"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/notifications"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *db.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable"
	}
	d, err := db.Connect(context.Background(), url)
	require.NoError(t, err, "integration tests need the local whmcs database")
	t.Cleanup(d.Close)
	return d
}

func TestTemplateRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := notifications.NewTemplateRepo(testDB(t))
	key := "it_tpl_" + uuid.NewString()[:8]
	t.Cleanup(func() {
		_ = repo.Delete(ctx, key, "id")
		_ = repo.Delete(ctx, key, "en")
	})

	// Insert via Upsert.
	tpl := &domain.EmailTemplate{
		Key: key, Locale: "id",
		Subject:  "Halo {{.Name}}",
		BodyHTML: "<p>Halo {{.Name}}</p>",
		BodyText: "Halo {{.Name}}",
	}
	require.NoError(t, repo.Upsert(ctx, tpl))
	assert.NotZero(t, tpl.ID)
	assert.False(t, tpl.CreatedAt.IsZero())

	// Get returns the stored row.
	got, err := repo.Get(ctx, key, "id")
	require.NoError(t, err)
	assert.Equal(t, tpl.ID, got.ID)
	assert.Equal(t, "Halo {{.Name}}", got.Subject)

	// Upsert on the same (key, locale) updates in place.
	tpl2 := &domain.EmailTemplate{
		Key: key, Locale: "id",
		Subject:  "Updated {{.Name}}",
		BodyHTML: "<p>Updated</p>",
		BodyText: "Updated",
	}
	require.NoError(t, repo.Upsert(ctx, tpl2))
	assert.Equal(t, tpl.ID, tpl2.ID, "same row updated, not duplicated")
	got, err = repo.Get(ctx, key, "id")
	require.NoError(t, err)
	assert.Equal(t, "Updated {{.Name}}", got.Subject)

	// Second locale is a distinct row; List contains both.
	en := &domain.EmailTemplate{Key: key, Locale: "en", Subject: "Hi", BodyHTML: "<p>Hi</p>"}
	require.NoError(t, repo.Upsert(ctx, en))
	assert.NotEqual(t, tpl.ID, en.ID)

	all, err := repo.List(ctx)
	require.NoError(t, err)
	var found int
	for _, row := range all {
		if row.Key == key {
			found++
		}
	}
	assert.Equal(t, 2, found)

	// Delete one locale; the other survives.
	require.NoError(t, repo.Delete(ctx, key, "en"))
	_, err = repo.Get(ctx, key, "en")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = repo.Get(ctx, key, "id")
	require.NoError(t, err)
}

func TestTemplateRepoGetMissing(t *testing.T) {
	repo := notifications.NewTemplateRepo(testDB(t))
	_, err := repo.Get(context.Background(), "does_not_exist_"+uuid.NewString()[:8], "id")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestTemplateRepoDeleteMissing(t *testing.T) {
	repo := notifications.NewTemplateRepo(testDB(t))
	err := repo.Delete(context.Background(), "does_not_exist_"+uuid.NewString()[:8], "id")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestTemplateRepoReadsSeededTemplates(t *testing.T) {
	repo := notifications.NewTemplateRepo(testDB(t))
	got, err := repo.Get(context.Background(), "admin_alert", "id")
	require.NoError(t, err)
	assert.Contains(t, got.Subject, "{{.Subject}}")
}
