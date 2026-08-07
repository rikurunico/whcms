package storage_test

// Uses the real local RustFS (S3) service; auto-skips when unreachable.

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/storage"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStorage(t *testing.T) *storage.S3 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := storage.New(ctx, storage.Options{
		Endpoint:  "http://localhost:9000",
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    "whmcs",
	})
	if err != nil {
		t.Skipf("rustfs not available, skipping: %v", err)
	}
	return s
}

func TestPutGetDeleteRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := testStorage(t)
	key := "test/" + uuid.NewString() + ".txt"
	content := "hello whcms storage"

	require.NoError(t, s.Put(ctx, key, strings.NewReader(content), int64(len(content)), "text/plain"))
	t.Cleanup(func() { _ = s.Delete(context.Background(), key) })

	rc, err := s.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, content, string(got))

	require.NoError(t, s.Delete(ctx, key))
	_, err = s.Get(ctx, key)
	assert.Error(t, err, "deleted object must not be readable")
}

func TestGetMissing(t *testing.T) {
	s := testStorage(t)
	_, err := s.Get(context.Background(), "test/definitely-missing-"+uuid.NewString())
	assert.Error(t, err)
}

func TestPresignGet(t *testing.T) {
	ctx := context.Background()
	s := testStorage(t)
	key := "test/" + uuid.NewString() + ".bin"
	payload := bytes.Repeat([]byte{0xAB}, 64)

	require.NoError(t, s.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "application/octet-stream"))
	t.Cleanup(func() { _ = s.Delete(context.Background(), key) })

	u, err := s.PresignGet(ctx, key, time.Minute)
	require.NoError(t, err)
	assert.Contains(t, u, key)

	resp, err := http.Get(u)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	got, _ := io.ReadAll(resp.Body)
	assert.Equal(t, payload, got)
}

func TestHealthy(t *testing.T) {
	s := testStorage(t)
	assert.NoError(t, s.Healthy(context.Background()))
}

// TestNewStripsHTTPSAndSurfacesBucketCheckError covers the "https://" prefix
// branch of New (endpoint stripping + UseSSL=true) together with the bucket
// check error path: an empty bucket name fails client-side validation before
// any network call is made, so this needs no reachable endpoint.
func TestNewStripsHTTPSAndSurfacesBucketCheckError(t *testing.T) {
	_, err := storage.New(context.Background(), storage.Options{
		Endpoint:  "https://localhost:9000",
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    "", // invalid: too short, fails client-side before any dial
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage: bucket check")
}

// TestNewClientCreationError covers the minio client construction error
// branch: a fully-qualified path in the endpoint is rejected before any
// network call is attempted.
func TestNewClientCreationError(t *testing.T) {
	_, err := storage.New(context.Background(), storage.Options{
		Endpoint:  "http://localhost:9000/not/a/valid/path",
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    "whmcs",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage: client")
}

// TestNewCreatesBucketWhenMissing covers the MakeBucket branch of New by
// targeting a fresh, uuid-suffixed bucket that does not yet exist. The
// bucket is a self-cleaning fixture: it is removed via a raw client in
// t.Cleanup and never touches the shared "whmcs" bucket.
func TestNewCreatesBucketWhenMissing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bucket := "whmcs-test-" + uuid.NewString()
	s, err := storage.New(ctx, storage.Options{
		Endpoint:  "http://localhost:9000",
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    bucket,
	})
	if err != nil {
		t.Skipf("rustfs not available, skipping: %v", err)
	}
	require.NotNil(t, s)

	t.Cleanup(func() {
		raw, err := minio.New("localhost:9000", &minio.Options{
			Creds:  credentials.NewStaticV4("rustfsadmin", "rustfsadmin", ""),
			Secure: false,
		})
		if err != nil {
			return
		}
		_ = raw.RemoveBucket(context.Background(), bucket)
	})

	assert.NoError(t, s.Healthy(context.Background()))
}

// TestPutEmptyKeyError covers Put's error branch: an empty object key fails
// minio-go's client-side validation before any network call.
func TestPutEmptyKeyError(t *testing.T) {
	s := testStorage(t)
	err := s.Put(context.Background(), "", strings.NewReader("x"), 1, "text/plain")
	assert.Error(t, err)
}

// TestGetEmptyKeyError covers Get's first error branch (the GetObject call
// itself, distinct from the lazy Stat() error covered by TestGetMissing).
func TestGetEmptyKeyError(t *testing.T) {
	s := testStorage(t)
	_, err := s.Get(context.Background(), "")
	assert.Error(t, err)
}

// TestDeleteEmptyKeyError covers Delete's error branch.
func TestDeleteEmptyKeyError(t *testing.T) {
	s := testStorage(t)
	err := s.Delete(context.Background(), "")
	assert.Error(t, err)
}

// TestPresignGetEmptyKeyError covers PresignGet's error branch.
func TestPresignGetEmptyKeyError(t *testing.T) {
	s := testStorage(t)
	_, err := s.PresignGet(context.Background(), "", time.Minute)
	assert.Error(t, err)
}

// TestHealthyContextCanceled covers Healthy's error branch: an
// already-canceled context makes the underlying BucketExists request fail
// immediately, with no dependence on server-side behavior.
func TestHealthyContextCanceled(t *testing.T) {
	s := testStorage(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, s.Healthy(ctx))
}
