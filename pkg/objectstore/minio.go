package objectstore

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client 封装MinIO客户端，提供对象存储操作的统一接口
type Client struct {
	client *minio.Client
	bucket string
}

// Open 初始化MinIO客户端并验证存储桶是否存在
func Open(ctx context.Context, cfg *config.MinIOConfig) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}
	store := &Client{client: client, bucket: cfg.Bucket}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// ensureBucket 确保存储桶存在，不存在则自动创建
func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("检查 MinIO 存储桶失败: %w", err)
	}
	if exists {
		return nil
	}
	if err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("创建 MinIO 存储桶 %q 失败: %w", c.bucket, err)
	}
	return nil
}

// Health 检查MinIO连接和存储桶是否健康
func (c *Client) Health(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("检查 MinIO 存储桶失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("MinIO 存储桶 %q 不存在", c.bucket)
	}
	return nil
}
