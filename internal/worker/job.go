package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrNoWork = errors.New("no work available")

type Job struct {
	ID             string
	Type           string
	Payload        json.RawMessage
	Attempt        int
	MaxAttempts    int
	Timeout        time.Duration
	IdempotencyKey string
}

type Handler interface {
	Handle(ctx context.Context, job Job) error
}

type HandlerFunc func(context.Context, Job) error

func (fn HandlerFunc) Handle(ctx context.Context, job Job) error { return fn(ctx, job) }

type Source interface {
	Reserve(ctx context.Context) (*Job, error)
	Complete(ctx context.Context, job Job) error
	Retry(ctx context.Context, job Job, availableAt time.Time, cause error) error
	Fail(ctx context.Context, job Job, cause error) error
}

type IdempotencyStore interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Complete(ctx context.Context, key string, ttl time.Duration) error
	Release(ctx context.Context, key string) error
}
