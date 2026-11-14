// Package artifacts stores large run artifacts in an S3-compatible bucket.
package artifacts

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Store is a thin S3 client. A nil Store means "not configured".
type Store struct {
	client *minio.Client
	bucket string
}

// New connects and creates the bucket when it does not exist. An empty endpoint
// returns (nil, nil): callers treat that as archiving disabled.
func New(ctx context.Context, endpoint, accessKey, secretKey, region, bucket string, useSSL bool) (*Store, error) {
	if endpoint == "" {
		return nil, nil
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("artifacts: client: %w", err)
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("artifacts: bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return nil, fmt.Errorf("artifacts: create bucket: %w", err)
		}
	}
	return &Store{client: client, bucket: bucket}, nil
}

// Put uploads one object (overwriting the same key is idempotent).
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if s == nil {
		return nil
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("artifacts: put %s: %w", key, err)
	}
	return nil
}

// Get downloads one object.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("artifacts: not configured")
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("artifacts: get %s: %w", key, err)
	}
	defer func() { _ = obj.Close() }()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("artifacts: read %s: %w", key, err)
	}
	return data, nil
}
