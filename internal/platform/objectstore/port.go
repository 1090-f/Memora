package objectstore

import (
	"context"
	"io"
)

// Store is the application port for private object storage.
type Store interface {
	Health(ctx context.Context) error
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
