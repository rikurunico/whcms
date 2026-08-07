package db

// Internal (white-box) unit tests for the unexported migrateURL helper. Pure
// string logic, no database required - always runs, tag or no tag.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"postgres scheme rewritten", "postgres://user:pass@host:5432/db", "pgx5://user:pass@host:5432/db"},
		{"postgresql scheme rewritten", "postgresql://user:pass@host:5432/db", "pgx5://user:pass@host:5432/db"},
		{"unknown scheme passed through", "mysql://user:pass@host:3306/db", "mysql://user:pass@host:3306/db"},
		{"too short to match either prefix", "pg", "pg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, migrateURL(tc.in))
		})
	}
}
