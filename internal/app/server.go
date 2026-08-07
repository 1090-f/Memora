package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/api"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/audit"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ServerApp 管理 HTTP 服务器应用的生命周期，包括初始化、运行和关闭。
type ServerApp struct {
	cfg    *config.Config
	db     *gorm.DB
	redis  *redis.Client
	store  *objectstore.Client
	server *http.Server
}

// NewServer 创建一个新的 ServerApp 实例。
func NewServer() *ServerApp { return &ServerApp{} }

// Initialize 初始化所有依赖，包括配置、数据库、缓存、对象存储、服务和路由。
func (a *ServerApp) Initialize(ctx context.Context) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	a.cfg = cfg
	if err := logger.Init(&cfg.Log); err != nil {
		return fmt.Errorf("初始化日志器失败: %w", err)
	}
	gin.SetMode(cfg.App.Mode)
	gin.DefaultWriter = io.Discard

	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := database.Migrate(cfg.Database.URL, "up"); err != nil {
		return err
	}
	a.db, err = database.InitPostgres(initCtx, &cfg.Database)
	if err != nil {
		return err
	}
	audit.SetStore(repository.NewAuditRepository(a.db))
	if err := bootstrapAdmin(initCtx, a.db, cfg.App.Mode); err != nil {
		_ = database.ClosePostgres(a.db)
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

	kbs := repository.NewKnowledgeBaseRepository(a.db)
	dirs := repository.NewDocumentDirectoryRepository(a.db)
	searchConfigs := repository.NewSearchConfigRepository(a.db)
	agentConfigs := repository.NewAgentConfigRepository(a.db)
	modelConfigs := repository.NewModelConfigRepository(a.db)
	transactor := repository.NewTransactor(a.db)
	kbService := service.NewKnowledgeBaseService(kbs, dirs, searchConfigs, agentConfigs, modelConfigs, transactor)
	directoryService := service.NewDirectoryService(kbs, dirs)

	mcpServers := repository.NewMCPServerRepository(a.db)
	mcpTools := repository.NewMCPToolRepository(a.db)
	mcpService := service.NewImportService(mcpServers, mcpTools, cfg)

	docs := repository.NewDocumentRepository(a.db)
	importTasks := repository.NewImportTaskRepository(a.db)
	chunks := repository.NewDocumentChunkRepository(a.db)
	vectors := repository.NewVectorRepository(a.db)
	documentService := service.NewDocumentService(docs, importTasks, kbs, dirs, a.store)
	documentProcessService := service.NewDocumentProcessService(importTasks, docs, chunks, vectors, nil)

	router := api.NewRouter(api.Dependencies{
		Config: cfg.CORS, Auth: authService, Users: userService, MCP: mcpService,
		KnowledgeBases: kbService, Directories: directoryService,
		Documents: documentService, DocumentProcess: documentProcessService,
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

// Run 启动 HTTP 服务器并阻塞等待，直到上下文取消后执行优雅关闭。
func (a *ServerApp) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("Memora API 服务已启动")
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

// Close 释放服务器应用持有的所有资源。
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
