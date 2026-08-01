package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/api"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServerApp struct {
	cfg    *config.Config
	db     *gorm.DB
	redis  *redis.Client
	store  *objectstore.Client
	server *http.Server
}

func NewServer() *ServerApp { return &ServerApp{} }

func (a *ServerApp) Initialize(ctx context.Context) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	a.cfg = cfg
	if err := logger.Init(&cfg.Log); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	gin.SetMode(cfg.App.Mode)

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
		return err
	}

	users := repository.NewUserRepository(a.db)
	userService := service.NewUserService(users)
	tokens, err := jwtmanager.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	if err != nil {
		_ = a.Close()
		return err
	}
	authService, err := service.NewAuthService(users, a.redis, tokens)
	if err != nil {
		_ = a.Close()
		return err
	}

	router := api.NewRouter(api.Dependencies{
		Config: cfg.CORS, Auth: authService, Users: userService,
		PostgresHealth: func(ctx context.Context) error { return database.CheckPostgres(ctx, a.db) },
		RedisHealth:    func(ctx context.Context) error { return database.CheckRedis(ctx, a.redis) },
		MinIOHealth:    a.store.Health,
		WorkerCount:    func(ctx context.Context) (int64, error) { return database.CountWorkerHeartbeats(ctx, a.redis) },
	})
	a.server = &http.Server{
		Addr: cfg.App.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: cfg.App.ReadTimeout, WriteTimeout: cfg.App.WriteTimeout,
	}
	return nil
}

func (a *ServerApp) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("Memora API started")
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return errors.Join(err, a.Close())
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
		defer cancel()
		return errors.Join(a.server.Shutdown(shutdownCtx), a.Close())
	}
}

func (a *ServerApp) Close() error {
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
