package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/ai"
	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/api"
	"github.com/1090-f/Memora/internal/background"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	ragpipeline "github.com/1090-f/Memora/internal/service/rag/pipeline"
	ragretrieval "github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
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
	cfg            *config.Config
	db             *gorm.DB
	redis          *redis.Client
	store          *objectstore.Client
	server         *http.Server
	background     *background.Manager
	documentParser *documentParserProcess
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
	a.documentParser = newDocumentParserProcess(cfg.DocumentParser)
	if err := a.documentParser.Ensure(ctx); err != nil {
		_ = a.Close()
		return fmt.Errorf("启动文档解析服务失败: %w", err)
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

	aiModelConfigs := repository.NewAIModelConfigRepository(a.db)
	keyMaterial := cfg.AI.EncryptionKey
	if keyMaterial == "" {
		keyMaterial = cfg.MCP.EncryptionKey
	}
	if keyMaterial == "" {
		keyMaterial = cfg.JWT.Secret
	}
	key := sha256.Sum256([]byte(keyMaterial))
	aiEncryption, err := encryption.NewService(key[:])
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 AI 凭证加密失败: %w", err)
	}
	modelFactory := ai.NewModelFactory(aiModelConfigs, aiEncryption, ai.NewProviderFactory())

	docs := repository.NewDocumentRepository(a.db)
	importTasks := repository.NewImportTaskRepository(a.db)
	chunks := repository.NewDocumentChunkRepository(a.db)
	vectors := repository.NewVectorRepository(a.db)
	parseConfigHash, err := parser.ParseConfigHash(documentParseOptions(cfg))
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("计算文档预览解析配置哈希失败: %w", err)
	}
	documentService := service.NewDocumentService(docs, importTasks, kbs, dirs, a.store, parseConfigHash, cfg.JWT.Secret)
	citationService := service.NewCitationService()
	documentReader, err := service.NewDocumentReader(chunks, citationService, cfg.JWT.Secret)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化文档读取服务失败: %w", err)
	}
	keywordRetriever, err := ragretrieval.NewPostgresKeywordRetriever(repository.NewKeywordSearchRepository(a.db), tokenizer.NewNgramTokenizer(tokenizer.DefaultNgramConfig()))
	if err != nil {
		_ = a.Close()
		return err
	}
	vectorRetriever, err := ragretrieval.NewPgVectorRetriever(vectors)
	if err != nil {
		_ = a.Close()
		return err
	}
	retrievalPipeline, err := ragpipeline.NewRetrievalPipeline(keywordRetriever, vectorRetriever, citationService)
	if err != nil {
		_ = a.Close()
		return err
	}
	retrievalService, err := service.NewRetrievalService(kbs, searchConfigs, aiModelConfigs, modelFactory, retrievalPipeline)
	if err != nil {
		_ = a.Close()
		return err
	}
	documentEmbeddingResolver, err := service.NewDocumentEmbeddingResolver(kbs, aiModelConfigs, modelFactory)
	if err != nil {
		_ = a.Close()
		return err
	}
	documentProcessService, err := buildDocumentProcessService(cfg, a.store, importTasks, docs, chunks, vectors, documentEmbeddingResolver)
	if err != nil {
		_ = a.Close()
		return err
	}
	a.background = background.NewManager(a.redis, importTasks, repository.NewTaskOutboxRepository(a.db), documentProcessService, cfg.DocumentConsumer, cfg.Outbox)

	// 初始化 ContextBuilder（Phase 3）
	messages := repository.NewMessageRepository(a.db)
	tokenCounter := service.NewTokenCounter()
	convCtxService := service.NewConversationContextService(messages, nil, tokenCounter)

	memoryRepo := repository.NewMemoryRepository(a.db)
	embeddingSvc := service.NewEmbeddingService(nil, "") // TODO: 从配置加载 embedding 模型
	memoryRetriever := service.NewMemoryRetriever(memoryRepo, embeddingSvc)

	contextBuilder := service.NewContextBuilder(agentConfigs, convCtxService, memoryRetriever)

	// 初始化 RouterService（Phase 4）
	// 使用用户配置的 ChatModelID 做路由判断
	llmRouter := service.NewLLMRouter(modelFactory)
	routerService := service.NewRouterService(llmRouter)

	// 初始化工具注册表和执行器
	// 注册内置只读工具：知识检索 + 文档阅读
	toolRegistry, err := tools.NewBuiltinRegistry(retrievalService, documentReader)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化工具注册表失败: %w", err)
	}
	toolExecutor := tools.NewExecutor(toolRegistry)

	// 初始化 Plan-Execute 服务（Phase 5）
	planRepo := repository.NewPlanRepository(a.db)
	planStateStore := service.NewPlanStateStore(planRepo)

	plannerService, err := service.NewPlannerService(modelFactory, "internal/ai/prompts/planner.yaml", cfg.Agent.MaxPlanSteps)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 PlannerService 失败: %w", err)
	}

	planExecutorService := service.NewPlanExecutorService(toolExecutor, planStateStore, modelFactory, cfg.Agent.MaxToolCalls, cfg.Agent.MaxToolResultBytes)

	reviewerService, err := service.NewReviewerService(modelFactory, "internal/ai/prompts/reviewer.yaml", planStateStore)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 ReviewerService 失败: %w", err)
	}

	replanService := service.NewReplanService(plannerService, planStateStore, cfg.Agent.MaxReplans)

	// 初始化 ReAct Agent 服务（Phase 6）
	reactService, err := service.NewReactService(modelFactory, toolExecutor, toolRegistry, "internal/ai/prompts/react.yaml")
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 ReactService 失败: %w", err)
	}

	// 初始化事件发布器（Phase 7）
	// TODO: 替换为真正的 contracts.EventPublisher 实现（如 Redis Event Publisher）
	var eventPub contracts.EventPublisher = noopEventPublisher{}
	sequencedEvents := core.NewSequencedEventPublisher(eventPub)

	// 初始化 ReactRunner
	reactRunner := core.NewReactRunner(reactService, sequencedEvents)
	_ = reactRunner // 用于后续集成

	// 初始化 PlanRunner
	planRunner := core.NewPlanRunner(plannerService, planExecutorService, reviewerService, replanService, planStateStore)
	_ = planRunner // 用于后续集成

	// 初始化 Agent Core Service（Phase 8）
	// TODO: 实现 RunRepository 接口后将 agent 核心服务注册到路由
	// agentCoreService := core.NewService(reactRunner, routerService, runRepository, sequencedEvents)
	// agentCoreService.SetPlanRunner(planRunner)

	router := api.NewRouter(api.Dependencies{
		Config: cfg.CORS, Auth: authService, Users: userService, MCP: mcpService,
		KnowledgeBases: kbService, Directories: directoryService, AIModelConfigs: aiModelConfigs, AIEncryption: aiEncryption,
		Documents: documentService, DocumentReader: documentReader, DocumentProcess: documentProcessService,
		Retrieval:      retrievalService,
		ContextBuilder: contextBuilder,
		AssetSignKey:   cfg.JWT.Secret,
		Router:         routerService,
		PostgresHealth: func(ctx context.Context) error { return database.CheckPostgres(ctx, a.db) },
		RedisHealth:    func(ctx context.Context) error { return database.CheckRedis(ctx, a.redis) },
		MinIOHealth:    a.store.Health,
		ParserHealth:   a.documentParser.Health,
		WorkerCount: func(context.Context) (int64, error) {
			if cfg.DocumentConsumer.Enabled {
				return int64(cfg.DocumentConsumer.Concurrency), nil
			}
			return 0, nil
		},
	})
	a.server = &http.Server{
		Addr: cfg.App.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: cfg.App.ReadTimeout, WriteTimeout: cfg.App.WriteTimeout,
	}
	return nil
}

// Run 启动 HTTP 服务器并阻塞等待，直到上下文取消后执行优雅关闭。
func (a *ServerApp) Run(ctx context.Context) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	httpErrCh := make(chan error, 1)
	backgroundErrCh := make(chan error, 1)
	go func() {
		logger.Info("Memora API 服务已启动")
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		httpErrCh <- err
	}()
	go func() { backgroundErrCh <- a.background.Run(runCtx) }()
	select {
	case err := <-httpErrCh:
		cancelRun()
		return errors.Join(err, <-backgroundErrCh, a.Close())
	case err := <-backgroundErrCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
		defer cancel()
		return errors.Join(err, a.server.Shutdown(shutdownCtx), <-httpErrCh, a.Close())
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
		defer cancel()
		shutdownErr := a.server.Shutdown(shutdownCtx)
		cancelRun()
		backgroundErr := <-backgroundErrCh
		<-httpErrCh
		return errors.Join(shutdownErr, backgroundErr, a.Close())
	}
}

// noopEventPublisher 是 contracts.EventPublisher 的空实现，用于在真正的发布器就绪前保持链路可运行。
type noopEventPublisher struct{}

func (noopEventPublisher) Publish(_ context.Context, _ contracts.AgentEvent) error { return nil }

// Close 释放服务器应用持有的所有资源。
func (a *ServerApp) Close() error {
	var closeErr error
	if a.documentParser != nil {
		closeErr = errors.Join(closeErr, a.documentParser.Close())
	}
	if a.redis != nil {
		closeErr = errors.Join(closeErr, a.redis.Close())
	}
	closeErr = errors.Join(closeErr, database.ClosePostgres(a.db))
	if logger.GetLogger() != nil {
		closeErr = errors.Join(closeErr, logger.Sync())
	}
	return closeErr
}
