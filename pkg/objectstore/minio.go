package objectstore

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client *minio.Client
	bucket string
}

func Open(ctx context.Context, cfg *config.MinIOConfig) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	store := &Client{client: client, bucket: cfg.Bucket}
	if err := store.Health(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (c *Client) Health(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("minio bucket %q does not exist", c.bucket)
	}
	return nil
}
