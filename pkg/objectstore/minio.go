package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client 封装MinIO客户端，提供对象存储操作的统一接口
type Client struct {
	client *minio.Client
	bucket string
}

// ObjectInfo 描述 MinIO 对象的元信息。
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
}

// ErrObjectNotFound 表示对象不存在。
var ErrObjectNotFound = errors.New("对象不存在")

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

// Bucket 返回当前使用的存储桶名称。
func (c *Client) Bucket() string { return c.bucket }

// PutObject 流式上传对象，不一次性读入内存。
// size 为 -1 时使用流式上传（UnknownSize，chunked），否则使用已知大小上传。
func (c *Client) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, c.bucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("上传对象 %q 失败: %w", objectKey, err)
	}
	return nil
}

// OpenObject 返回可关闭的对象读取流，调用方负责关闭，不一次性读入内存。
func (c *Client) OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("打开对象 %q 失败: %w", objectKey, err)
	}
	return obj, nil
}

// StatObject 查询对象元信息，对象不存在时返回 ErrObjectNotFound。
func (c *Client) StatObject(ctx context.Context, objectKey string) (*ObjectInfo, error) {
	info, err := c.client.StatObject(ctx, c.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		var responseErr minio.ErrorResponse
		if errors.As(err, &responseErr) && responseErr.Code == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("查询对象 %q 失败: %w", objectKey, err)
	}
	return &ObjectInfo{
		Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag,
	}, nil
}

// RemoveObject 删除对象，对象不存在时不视为错误。
func (c *Client) RemoveObject(ctx context.Context, objectKey string) error {
	err := c.client.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("删除对象 %q 失败: %w", objectKey, err)
	}
	return nil
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
