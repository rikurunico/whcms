package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyEnvFileMissingFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	err := config.ApplyEnvFile(filepath.Join(dir, "does-not-exist.env"))
	require.NoError(t, err)
}

func TestApplyEnvFileSetsUnsetVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.env")
	require.NoError(t, os.WriteFile(path, []byte(""+
		"# a comment\n"+
		"\n"+
		"WHCMS_TEST_ENVFILE_A=hello\n"+
		"export WHCMS_TEST_ENVFILE_B=world\n"+
		"WHCMS_TEST_ENVFILE_DSN=postgres://root:postgres@localhost:5432/whmcs?sslmode=disable\n"+
		"WHCMS_TEST_ENVFILE_QUOTED=\"quoted value\"\n",
	), 0o600))

	t.Cleanup(func() {
		for _, k := range []string{
			"WHCMS_TEST_ENVFILE_A", "WHCMS_TEST_ENVFILE_B",
			"WHCMS_TEST_ENVFILE_DSN", "WHCMS_TEST_ENVFILE_QUOTED",
		} {
			os.Unsetenv(k)
		}
	})

	require.NoError(t, config.ApplyEnvFile(path))
	assert.Equal(t, "hello", os.Getenv("WHCMS_TEST_ENVFILE_A"))
	assert.Equal(t, "world", os.Getenv("WHCMS_TEST_ENVFILE_B"))
	assert.Equal(t, "postgres://root:postgres@localhost:5432/whmcs?sslmode=disable", os.Getenv("WHCMS_TEST_ENVFILE_DSN"))
	assert.Equal(t, "quoted value", os.Getenv("WHCMS_TEST_ENVFILE_QUOTED"))
}

// TestReadEnvFileStripsInlineComments covers a real bug: a line like
// "DUITKU_BASE_URL=            # empty -> derived from DUITKU_ENV" (this
// repo's own .env/.env.example style for documenting a blank default inline)
// used to have the trailing comment become part of the VALUE - because
// parseEnvLine's original TrimSpace-only parsing has no notion of "#"
// starting a comment at all. That silently sent every outbound Duitku call
// to `#<comment text><path>` as a base URL. Confirmed 2026-07-27 via
// integration_logs showing `unsupported protocol scheme ""` with the literal
// comment text URL-encoded into the request.
func TestReadEnvFileStripsInlineComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.env")
	require.NoError(t, os.WriteFile(path, []byte(""+
		"DUITKU_BASE_URL=            # empty -> derived from DUITKU_ENV; set to mockserver URL in dev/E2E\n"+
		"WHCMS_TEST_ENVFILE_TRAILING=value here   # trailing comment\n"+
		"WHCMS_TEST_ENVFILE_HASH_GLUED=abc#not-a-comment\n"+
		"WHCMS_TEST_ENVFILE_QUOTED_HASH=\"abc # still inside quotes\"\n",
	), 0o600))

	got, err := config.ReadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "", got["DUITKU_BASE_URL"], "blank value + inline comment must collapse to empty, not the comment text")
	assert.Equal(t, "value here", got["WHCMS_TEST_ENVFILE_TRAILING"], "trailing comment stripped, value kept")
	assert.Equal(t, "abc#not-a-comment", got["WHCMS_TEST_ENVFILE_HASH_GLUED"], "a '#' with no preceding whitespace is literal data, not a comment")
	assert.Equal(t, "abc # still inside quotes", got["WHCMS_TEST_ENVFILE_QUOTED_HASH"], "a '#' inside quotes is never stripped")
}

func TestApplyEnvFileRealEnvWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.env")
	require.NoError(t, os.WriteFile(path, []byte("WHCMS_TEST_ENVFILE_WINS=from-file\n"), 0o600))

	t.Setenv("WHCMS_TEST_ENVFILE_WINS", "from-real-env")

	require.NoError(t, config.ApplyEnvFile(path))
	assert.Equal(t, "from-real-env", os.Getenv("WHCMS_TEST_ENVFILE_WINS"),
		"a real env var must never be overridden by the file")
}

func TestWriteEnvFileCreatesAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "app.env")

	require.NoError(t, config.WriteEnvFile(path, map[string]string{
		"DATABASE_URL": "postgres://root:postgres@localhost:5432/whmcs?sslmode=disable",
		"JWT_SECRET":   "s3cr3t",
	}))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	got, err := config.ReadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "postgres://root:postgres@localhost:5432/whmcs?sslmode=disable", got["DATABASE_URL"])
	assert.Equal(t, "s3cr3t", got["JWT_SECRET"])
}

func TestWriteEnvFileMergesWithExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.env")

	require.NoError(t, config.WriteEnvFile(path, map[string]string{
		"KEEP_ME":      "original",
		"OVERWRITE_ME": "old",
	}))
	require.NoError(t, config.WriteEnvFile(path, map[string]string{
		"OVERWRITE_ME": "new",
	}))

	got, err := config.ReadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", got["KEEP_ME"], "keys not touched by the second write must survive")
	assert.Equal(t, "new", got["OVERWRITE_ME"])
}
