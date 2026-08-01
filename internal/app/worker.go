package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	workerengine "github.com/1090-f/Memora/internal/worker"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type WorkerApp struct {
	db        *gorm.DB
	redis     *redis.Client
	store     *objectstore.Client
	runner    *workerengine.Runner
	heartbeat *workerengine.Heartbeat
	registry  *workerengine.Registry
	sources   *workerengine.RoutedSource
}

func NewWorker() *WorkerApp {
	return &WorkerApp{registry: workerengine.NewRegistry(), sources: workerengine.NewRoutedSource()}
}

func (a *WorkerApp) RegisterJob(jobType string, source workerengine.Source, handler workerengine.Handler) error {
	if err := a.registry.Register(jobType, handler); err != nil {
		return err
	}
	return a.sources.Register(jobType, source)
}

func (a *WorkerApp) Initialize(ctx context.Context) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	if err := logger.Init(&cfg.Log); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	a.db, err = database.InitPostgres(initCtx, &cfg.Database)
	if err != nil {
		return err
	}
	a.redis, err = database.InitRedis(initCtx, &cfg.Redis)
	if err != nil {
		_ = database.ClosePostgres(a.db)
		return err
	}
	a.store, err = objectstore.Open(initCtx, &cfg.MinIO)
	if err != nil {
		_ = a.redis.Close()
		_ = database.ClosePostgres(a.db)
	}
	if err != nil {
		return err
	}
	a.runner, err = workerengine.NewRunner(workerengine.RunnerConfig{
		Concurrency: cfg.Worker.Concurrency, PollInterval: cfg.Worker.PollInterval,
		DefaultTimeout: cfg.Worker.DefaultTimeout, MaxRetryDelay: cfg.Worker.MaxRetryDelay,
		IdempotencyTTL: cfg.Worker.IdempotencyTTL,
	}, a.sources, a.registry, workerengine.NewRedisIdempotencyStore(a.redis))
	if err != nil {
		_ = a.Close()
		return err
	}
	a.heartbeat, err = workerengine.NewHeartbeat(a.redis, 10*time.Second, 30*time.Second)
	if err != nil {
		_ = a.Close()
	}
	return err
}

func (a *WorkerApp) Run(ctx context.Context) error {
	logger.Info("Memora worker started")
	heartbeatErr := make(chan error, 1)
	go func() { heartbeatErr <- a.heartbeat.Run(ctx) }()
	runnerErr := a.runner.Run(ctx)
	return errors.Join(runnerErr, <-heartbeatErr, a.Close())
}

func (a *WorkerApp) Close() error {
	var closeErr error
	if a.redis != nil {
		closeErr = errors.Join(closeErr, a.redis.Close())
	}
	closeErr = errors.Join(closeErr, database.ClosePostgres(a.db))
	if logger.GetLogger() != nil {
		closeErr = errors.Join(closeErr, logger.Sync())
	}
	return closeErr
}
