package worker_test

import (
	"context"
	"sync"
	"testing"

	"github.com/1090-f/Memora/internal/app/worker"
	"github.com/stretchr/testify/require"
)

func TestWorkerStopsOnContextCancellation(t *testing.T) {
	runner := newBlockingRunner()
	app := worker.New(worker.Dependencies{Runners: []worker.Runner{runner}})
	ctx, cancel := context.WithCancel(context.Background())
	done := runAsync(func() error { return app.Run(ctx) })

	<-runner.started
	cancel()

	require.NoError(t, <-done)
	require.True(t, runner.Stopped())
}

type blockingRunner struct {
	started chan struct{}
	mu      sync.RWMutex
	stopped bool
}

func newBlockingRunner() *blockingRunner {
	return &blockingRunner{started: make(chan struct{})}
}

func (r *blockingRunner) Run(ctx context.Context) error {
	close(r.started)
	<-ctx.Done()
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	return nil
}

func (r *blockingRunner) Stopped() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stopped
}

func runAsync(fn func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	return done
}
