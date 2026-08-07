// Package migrations embeds the SQL migration files so the API binary can run
// them on startup regardless of working directory.
package migrations

import "embed"

// FS contains all *.sql migration files.
//
//go:embed *.sql
var FS embed.FS
