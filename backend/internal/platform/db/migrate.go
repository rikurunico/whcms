package db

import (
	"errors"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/migrations"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies all embedded migrations to the database (idempotent).
// Called by the API on startup.
func Migrate(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("db: migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("db: migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back the given number of migration steps (dev tooling).
func MigrateDown(databaseURL string, steps int) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("db: migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("db: migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate down: %w", err)
	}
	return nil
}

// migrateURL rewrites postgres:// to the golang-migrate pgx/v5 driver scheme.
func migrateURL(databaseURL string) string {
	const pg, pgql = "postgres://", "postgresql://"
	switch {
	case len(databaseURL) > len(pg) && databaseURL[:len(pg)] == pg:
		return "pgx5://" + databaseURL[len(pg):]
	case len(databaseURL) > len(pgql) && databaseURL[:len(pgql)] == pgql:
		return "pgx5://" + databaseURL[len(pgql):]
	}
	return databaseURL
}
