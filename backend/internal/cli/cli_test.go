package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_NoArgs(t *testing.T) {
	err := Run([]string{"whcms"})
	assert.NoError(t, err)
}

func TestRun_Version(t *testing.T) {
	err := Run([]string{"whcms", "version"})
	assert.NoError(t, err)
}

func TestRun_Help(t *testing.T) {
	err := Run([]string{"whcms", "help"})
	assert.NoError(t, err)
}

func TestRun_HelpFlags(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "--version", "-v"} {
		err := Run([]string{"whcms", flag})
		assert.NoError(t, err, "flag %s should not error", flag)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := Run([]string{"whcms", "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestParseInstallFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want installConfig
	}{
		{
			name: "defaults",
			args: []string{},
			want: installConfig{Port: 8080},
		},
		{
			name: "all flags",
			args: []string{"--non-interactive", "--domain", "example.com", "--email", "a@b.com", "--password", "secret123", "--skip-deps", "--port", "9090"},
			want: installConfig{
				NonInteractive: true,
				Domain:         "example.com",
				Email:          "a@b.com",
				Password:       "secret123",
				SkipDeps:       true,
				Port:           9090,
			},
		},
		{
			name: "partial flags",
			args: []string{"--domain", "test.com", "--port", "3000"},
			want: installConfig{
				Domain: "test.com",
				Port:   3000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInstallFlags(tt.args)
			assert.Equal(t, tt.want.Domain, got.Domain)
			assert.Equal(t, tt.want.Email, got.Email)
			assert.Equal(t, tt.want.Password, got.Password)
			assert.Equal(t, tt.want.NonInteractive, got.NonInteractive)
			assert.Equal(t, tt.want.SkipDeps, got.SkipDeps)
			assert.Equal(t, tt.want.Port, got.Port)
		})
	}
}

func TestValidateInstallConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     installConfig
		wantErr string
	}{
		{
			name:    "missing domain",
			cfg:     installConfig{Email: "a@b.com", Password: "secret123", Port: 8080},
			wantErr: "domain is required",
		},
		{
			name:    "missing email",
			cfg:     installConfig{Domain: "example.com", Password: "secret123", Port: 8080},
			wantErr: "admin email is required",
		},
		{
			name:    "missing password",
			cfg:     installConfig{Domain: "example.com", Email: "a@b.com", Port: 8080},
			wantErr: "admin password is required",
		},
		{
			name:    "invalid port low",
			cfg:     installConfig{Domain: "example.com", Email: "a@b.com", Password: "secret123", Port: 0},
			wantErr: "port must be 1-65535",
		},
		{
			name:    "invalid port high",
			cfg:     installConfig{Domain: "example.com", Email: "a@b.com", Password: "secret123", Port: 70000},
			wantErr: "port must be 1-65535",
		},
		{
			name: "valid",
			cfg:  installConfig{Domain: "example.com", Email: "a@b.com", Password: "secret123", Port: 8080},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstallConfig(tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstallCmd_RejectsNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("this test is for non-linux platforms")
	}
	err := cmdInstall([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only supports Linux")
}

func TestCreateInstallDirs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &installConfig{InstallDir: tmpDir}

	err := createInstallDirs(cfg)
	require.NoError(t, err)

	expected := []string{"bin", "config", "data", "logs"}
	for _, sub := range expected {
		path := tmpDir + "/" + sub
		info, err := os.Stat(path)
		assert.NoError(t, err, "directory %s should exist", sub)
		assert.True(t, info.IsDir(), "%s should be a directory", sub)
	}
}

func TestRandomToken(t *testing.T) {
	token1, err := randomToken(32)
	require.NoError(t, err)
	assert.NotEmpty(t, token1)

	token2, err := randomToken(32)
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2, "tokens should be unique")
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		assert.Equal(t, tt.want, got, "formatBytes(%d)", tt.input)
	}
}

func TestMin(t *testing.T) {
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 3, min(5, 3))
	assert.Equal(t, 3, min(3, 3))
}

func TestGetInstallDir(t *testing.T) {
	original := os.Getenv("WHCMS_HOME")
	defer os.Setenv("WHCMS_HOME", original)

	os.Setenv("WHCMS_HOME", "/custom/path")
	assert.Equal(t, "/custom/path", getInstallDir())

	os.Unsetenv("WHCMS_HOME")
	dir := getInstallDir()
	assert.True(t, strings.HasSuffix(dir, ".whcms"), "default should end with .whcms, got %s", dir)
}

func TestCommandExists(t *testing.T) {
	assert.True(t, commandExists("go"), "go should exist in test environment")
	assert.False(t, commandExists("nonexistent_binary_xyz_123"), "random binary should not exist")
}

func TestIsPortAvailable(t *testing.T) {
	assert.True(t, isPortAvailable(0), "port 0 should ask OS for any free port")
}

func TestGetEnvValue(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "env-*.env")
	require.NoError(t, err)

	content := `# comment
APP_ENV=production
APP_PORT=8080
DATABASE_URL=postgres://user:pass@localhost/db?sslmode=disable
EMPTY_VAR=
`
	_, err = io.Copy(tmpFile, strings.NewReader(content))
	require.NoError(t, err)
	tmpFile.Close()

	assert.Equal(t, "production", getEnvValue(tmpFile.Name(), "APP_ENV"))
	assert.Equal(t, "8080", getEnvValue(tmpFile.Name(), "APP_PORT"))
	assert.Equal(t, "postgres://user:pass@localhost/db?sslmode=disable", getEnvValue(tmpFile.Name(), "DATABASE_URL"))
	assert.Equal(t, "", getEnvValue(tmpFile.Name(), "EMPTY_VAR"))
	assert.Equal(t, "", getEnvValue(tmpFile.Name(), "NONEXISTENT"))
	assert.Equal(t, "", getEnvValue("/nonexistent/path.env", "ANY"))
}

func TestBackupRestore_RoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	require.NoError(t, os.MkdirAll(srcDir+"/subdir", 0755))
	require.NoError(t, os.WriteFile(srcDir+"/file1.txt", []byte("hello"), 0644))
	require.NoError(t, os.WriteFile(srcDir+"/subdir/file2.txt", []byte("world"), 0644))

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := addDirToTar(tw, srcDir, "backup")
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	tarPath := srcDir + "/test.tar.gz"
	require.NoError(t, os.WriteFile(tarPath, buf.Bytes(), 0644))

	err = extractTarGz(tarPath, dstDir)
	require.NoError(t, err)

	data, err := os.ReadFile(dstDir + "/backup/file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))

	data, err = os.ReadFile(dstDir + "/backup/subdir/file2.txt")
	require.NoError(t, err)
	assert.Equal(t, "world", string(data))
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir() + "/dst"

	require.NoError(t, os.MkdirAll(srcDir+"/nested", 0755))
	require.NoError(t, os.WriteFile(srcDir+"/a.txt", []byte("aaa"), 0644))
	require.NoError(t, os.WriteFile(srcDir+"/nested/b.txt", []byte("bbb"), 0644))

	err := copyDir(srcDir, dstDir)
	require.NoError(t, err)

	data, err := os.ReadFile(dstDir + "/a.txt")
	require.NoError(t, err)
	assert.Equal(t, "aaa", string(data))

	data, err = os.ReadFile(dstDir + "/nested/b.txt")
	require.NoError(t, err)
	assert.Equal(t, "bbb", string(data))
}

func TestServiceStatus_Fallback(t *testing.T) {
	status := getServiceStatus("nonexistent-service-xyz")
	assert.Equal(t, "nonexistent-service-xyz", status.Name)
}

func TestCmdStatus_NoPanic(t *testing.T) {
	err := cmdStatus([]string{})
	assert.NoError(t, err)
}

func TestCmdStatus_JSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmdStatus([]string{"--json"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "whcms-api")
}

func TestAddFileToTar(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := addFileToTar(tw, testFile, "test.txt")
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	assert.True(t, buf.Len() > 0)
}

func TestBackupBinaries(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := tmpDir + "/bin"
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.WriteFile(binDir+"/whcms-api", []byte("fake binary"), 0755))

	backupPath := tmpDir + "/backup.tar.gz"
	err := backupBinaries(tmpDir, backupPath)
	require.NoError(t, err)

	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0)
}
