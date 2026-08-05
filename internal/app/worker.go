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

// WorkerApp 管理 Worker 应用的生命周期，包括初始化、任务注册和执行。
type WorkerApp struct {
	db        *gorm.DB
	redis     *redis.Client
	store     *objectstore.Client
	runner    *workerengine.Runner
	heartbeat *workerengine.Heartbeat
	registry  *workerengine.Registry
	sources   *workerengine.RoutedSource
}

// NewWorker 创建一个新的 WorkerApp 实例，初始化空的注册表和路由任务源。
func NewWorker() *WorkerApp {
	return &WorkerApp{registry: workerengine.NewRegistry(), sources: workerengine.NewRoutedSource()}
}

// RegisterJob 注册一个任务类型及其对应的任务源和处理器。
func (a *WorkerApp) RegisterJob(jobType string, source workerengine.Source, handler workerengine.Handler) error {
	if err := a.registry.Register(jobType, handler); err != nil {
		return err
	}
	return a.sources.Register(jobType, source)
}

// Initialize 初始化所有依赖，包括配置、数据库、缓存、对象存储和 Worker 引擎。
func (a *WorkerApp) Initialize(ctx context.Context) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	if err := logger.Init(&cfg.Log); err != nil {
		return fmt.Errorf("初始化日志器失败: %w", err)
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

// Run 启动 Worker 运行器和心跳机制，阻塞等待直到上下文取消。
func (a *WorkerApp) Run(ctx context.Context) error {
	logger.Info("Memora Worker 已启动")
	heartbeatErr := make(chan error, 1)
	go func() { heartbeatErr <- a.heartbeat.Run(ctx) }()
	runnerErr := a.runner.Run(ctx)
	return errors.Join(runnerErr, <-heartbeatErr, a.Close())
}

// Close 释放 Worker 应用持有的所有资源。
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
