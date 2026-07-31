package worker

import (
	"context"
	"errors"
	"io"
	"sync"
)

// Dependencies are the runners and resources owned by the worker process.
type Dependencies struct {
	Runners []Runner
	Closers []io.Closer
}

// App coordinates background work and dependency shutdown.
type App struct {
	runners []Runner
	closers []io.Closer
}

// New builds a worker application from explicit dependencies.
func New(deps Dependencies) *App {
	return &App{
		runners: append([]Runner(nil), deps.Runners...),
		closers: append([]io.Closer(nil), deps.Closers...),
	}
}

// Run starts every runner, waits for cancellation or completion, and then closes dependencies.
func (a *App) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(a.runners))
	var wg sync.WaitGroup
	for _, runner := range a.runners {
		wg.Add(1)
		go func(runner Runner) {
			defer wg.Done()
			errCh <- runner.Run(runCtx)
		}(runner)
	}

	runnerErr := waitForRunners(ctx, cancel, errCh, len(a.runners))
	wg.Wait()
	return errors.Join(runnerErr, a.closeDependencies())
}

func waitForRunners(ctx context.Context, cancel context.CancelFunc, errCh <-chan error, count int) error {
	if count == 0 {
		<-ctx.Done()
		return nil
	}

	var runnerErr error
	for completed := 0; completed < count; completed++ {
		select {
		case <-ctx.Done():
			cancel()
			for ; completed < count; completed++ {
				err := <-errCh
				if err != nil && !errors.Is(err, context.Canceled) {
					runnerErr = errors.Join(runnerErr, err)
				}
			}
			return runnerErr
		case err := <-errCh:
			if err != nil {
				runnerErr = errors.Join(runnerErr, err)
				cancel()
			}
		}
	}
	return runnerErr
}

func (a *App) closeDependencies() error {
	var closeErr error
	for index := len(a.closers) - 1; index >= 0; index-- {
		if err := a.closers[index].Close(); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}
