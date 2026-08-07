// Package storage implements ports.Storage over any S3-compatible endpoint
// (RustFS in this deployment) using minio-go.
package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
)

// S3 is the minio-backed ports.Storage.
type S3 struct {
	client *minio.Client
	bucket string
}

// Options configure the S3 storage.
type Options struct {
	Endpoint  string // may include http:// or https:// scheme
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// New connects to the S3 endpoint and ensures the bucket exists.
func New(ctx context.Context, opts Options) (*S3, error) {
	endpoint := opts.Endpoint
	useSSL := opts.UseSSL
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useSSL = true
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		useSSL = false
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: client: %w", err)
	}

	s := &S3{client: client, bucket: opts.Bucket}
	exists, err := client.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, opts.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("storage: make bucket: %w", err)
		}
	}
	return s, nil
}

// Put uploads an object.
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	return nil
}

// Get downloads an object. The caller must Close the reader.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	// GetObject is lazy; surface missing-object errors now.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("storage: stat %s: %w", key, err)
	}
	return obj, nil
}

// Delete removes an object (no error if absent).
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}

// PresignGet returns a presigned GET URL valid for ttl. opts may override the
// response Content-Type/Content-Disposition headers the client sees (S3's
// response-content-type / response-content-disposition query overrides) so
// downloads stay safe even if the stored object's own content-type is wrong.
func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration, opts ...ports.PresignOption) (string, error) {
	var o ports.PresignOptions
	for _, opt := range opts {
		opt(&o)
	}
	reqParams := url.Values{}
	if o.ResponseContentType != "" {
		reqParams.Set("response-content-type", o.ResponseContentType)
	}
	if o.ResponseContentDisposition != "" {
		reqParams.Set("response-content-disposition", o.ResponseContentDisposition)
	}
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, reqParams)
	if err != nil {
		return "", fmt.Errorf("storage: presign %s: %w", key, err)
	}
	return u.String(), nil
}

// Healthy reports whether the bucket is reachable (used by /readyz).
func (s *S3) Healthy(ctx context.Context) error {
	if _, err := s.client.BucketExists(ctx, s.bucket); err != nil {
		return fmt.Errorf("storage: health: %w", err)
	}
	return nil
}
