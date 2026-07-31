package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	ErrInvalidKey      = errors.New("object key must be a relative path")
	ErrInvalidEndpoint = errors.New("minio endpoint is required")
	ErrInvalidBucket   = errors.New("minio bucket is required")
	ErrBucketNotFound  = errors.New("minio bucket does not exist")
)

type PutOptions struct {
	ContentType string
}

// Client abstracts the MinIO operations used by Store.
type Client interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts PutOptions) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	RemoveObject(ctx context.Context, bucket, key string) error
}

type store struct {
	client Client
	bucket string
}

// New adapts a MinIO client into the Store port. The optional bucket makes it
// convenient to test input validation without configuring a bucket.
func New(client Client, bucket ...string) Store {
	name := ""
	if len(bucket) > 0 {
		name = bucket[0]
	}
	return store{client: client, bucket: name}
}

// Open creates a private MinIO-backed Store and verifies the configured bucket exists.
func Open(ctx context.Context, cfg config.MinIOConfig) (Store, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, ErrInvalidEndpoint
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, ErrInvalidBucket
	}

	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	client := minioAdapter{client: minioClient}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrBucketNotFound, cfg.Bucket)
	}

	return New(client, cfg.Bucket), nil
}

func (s store) Health(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: %s", ErrBucketNotFound, s.bucket)
	}
	return nil
}

func (s store) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := s.client.PutObject(ctx, s.bucket, key, reader, size, PutOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	reader, err := s.client.GetObject(ctx, s.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	return reader, nil
}

func (s store) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func validateKey(key string) error {
	if key == "" || strings.Contains(key, `\\`) || path.IsAbs(key) {
		return ErrInvalidKey
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == ".." {
			return ErrInvalidKey
		}
	}
	return nil
}

type minioAdapter struct {
	client *minio.Client
}

func (a minioAdapter) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return a.client.BucketExists(ctx, bucket)
}

func (a minioAdapter) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts PutOptions) error {
	_, err := a.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{ContentType: opts.ContentType})
	return err
}

func (a minioAdapter) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	object, err := a.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func (a minioAdapter) RemoveObject(ctx context.Context, bucket, key string) error {
	return a.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}
