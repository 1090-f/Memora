package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/ai"
	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/api"
	"github.com/1090-f/Memora/internal/background"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	previewservice "github.com/1090-f/Memora/internal/service/preview"
	previewrenderer "github.com/1090-f/Memora/internal/service/preview/renderer"
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
	previewRepo := repository.NewDocumentPreviewRepository(a.db)
	officeRenderer, officeErr := previewrenderer.NewLibreOffice(cfg.Preview.Enabled && cfg.Preview.Office.Enabled, cfg.Preview.Office.MaxConcurrency, cfg.Preview.Office.Timeout)
	if officeErr != nil {
		// LibreOffice 缺失不阻断启动：Descriptor 会返回 parsed text/download fallback。
		logger.Warnf("Office 异步预览不可用，请安装 LibreOffice: %v", officeErr)
		officeRenderer, _ = previewrenderer.NewLibreOffice(false, 1, cfg.Preview.Office.Timeout)
	}
	xlsxRenderer := previewrenderer.NewXLSX(cfg.Preview.XLSX)
	previewScheduler := previewservice.NewScheduler(docs, previewRepo, cfg.Preview.Enabled, officeRenderer.Info(), xlsxRenderer.Info())
	previewService := previewservice.NewService(docs, previewRepo, a.store, parseConfigHash, cfg.JWT.Secret, previewScheduler, officeRenderer.Info(), xlsxRenderer.Info(), cfg.Preview.XLSX.MaxUncompressedBytes)
	previewProcessor := previewservice.NewProcessor(docs, previewRepo, a.store, []previewservice.Renderer{officeRenderer, xlsxRenderer}, cfg.Preview.XLSX.MaxUncompressedBytes)
	documentService := service.NewDocumentService(docs, importTasks, kbs, dirs, a.store, parseConfigHash, cfg.JWT.Secret, nil)
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
	documentProcessService, err := buildDocumentProcessService(cfg, a.store, importTasks, docs, chunks, vectors, documentEmbeddingResolver, previewScheduler)
	if err != nil {
		_ = a.Close()
		return err
	}
	a.background = background.NewManager(a.redis, importTasks, previewRepo, repository.NewTaskOutboxRepository(a.db), documentProcessService, previewProcessor, cfg.DocumentConsumer, cfg.Preview, cfg.Outbox)

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

	// 初始化 Plan-Execute 服务（Phase 5）
	planRepo := repository.NewPlanRepository(a.db)
	planStateStore := service.NewPlanStateStore(planRepo)

	plannerService, err := service.NewPlannerService(modelFactory, "internal/ai/prompts/planner.yaml", cfg.Agent.MaxPlanSteps)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 PlannerService 失败: %w", err)
	}

	// TODO: 从配置注入 ToolExecutor
	// 暂时使用 nil，需要在 Agent Core 初始化时注入
	planExecutorService := service.NewPlanExecutorService(nil, planStateStore, modelFactory, cfg.Agent.MaxToolCalls, cfg.Agent.MaxToolResultBytes)
	_ = planExecutorService // 用于后续集成

	reviewerService, err := service.NewReviewerService(modelFactory, "internal/ai/prompts/reviewer.yaml", planStateStore)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 ReviewerService 失败: %w", err)
	}
	_ = reviewerService // 用于后续集成

	replanService := service.NewReplanService(plannerService, planStateStore, cfg.Agent.MaxReplans)
	_ = replanService // 用于后续集成

	// 初始化 Memory 提取与管理服务（Phase 6）
	memoryManager, err := service.NewMemoryManager(memoryRepo, embeddingSvc, modelFactory, "internal/ai/prompts/memory_dedup.yaml")
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 MemoryManager 失败: %w", err)
	}

	memoryExtractor, err := service.NewMemoryExtractor(modelFactory, memoryManager, "internal/ai/prompts/memory_extractor.yaml")
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化 MemoryExtractor 失败: %w", err)
	}
	_ = memoryExtractor // 用于后续集成

	router := api.NewRouter(api.Dependencies{
		Config: cfg.CORS, Auth: authService, Users: userService, MCP: mcpService,
		KnowledgeBases: kbService, Directories: directoryService, AIModelConfigs: aiModelConfigs, AIEncryption: aiEncryption,
		Documents: documentService, Preview: previewService, DocumentReader: documentReader, DocumentProcess: documentProcessService,
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
