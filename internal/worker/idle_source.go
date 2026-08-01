package worker

import (
	"context"
	"time"
)

// IdleSource keeps the worker process healthy until business modules register
// a durable source backed by their own status tables.
type IdleSource struct{}

func (IdleSource) Reserve(context.Context) (*Job, error) { return nil, ErrNoWork }
func (IdleSource) Complete(context.Context, Job) error   { return nil }
func (IdleSource) Retry(context.Context, Job, time.Time, error) error {
	return nil
}
func (IdleSource) Fail(context.Context, Job, error) error { return nil }
