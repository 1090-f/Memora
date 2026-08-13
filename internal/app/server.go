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
	"github.com/1090-f/Memora/internal/api/v1/agent"
	"github.com/1090-f/Memora/internal/background"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/events"
	"github.com/1090-f/Memora/internal/mcp"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	previewservice "github.com/1090-f/Memora/internal/service/preview"
	previewrenderer "github.com/1090-f/Memora/internal/service/preview/renderer"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	ragpipeline "github.com/1090-f/Memora/internal/service/rag/pipeline"
	ragretrieval "github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
	"github.com/1090-f/Memora/internal/worker"
	"github.com/1090-f/Memora/pkg/audit"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
	jwtmanager "github.com/1090-f/Memora/pkg/jwt"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
	workerCancel   context.CancelFunc // Agent Worker 生命周期取消函数
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

	// MCP Server 和 Tool 仓库（稍后用于初始化 MCP 服务和工具刷新器）
	mcpServers := repository.NewMCPServerRepository(a.db)
	mcpTools := repository.NewMCPToolRepository(a.db)

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

	conversationRepo := repository.NewConversationRepository(a.db)
	conversationService := service.NewConversationService(conversationRepo)

	// 初始化工具注册表和执行器（Phase 3.5）
	// 注册内置只读工具：知识检索 + 文档阅读
	// 必须在 ContextBuilder 之前创建，因为 ContextBuilder 需要访问工具注册表
	toolRegistry, err := tools.NewBuiltinRegistry(retrievalService, documentReader)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("初始化工具注册表失败: %w", err)
	}
	toolExecutor := tools.NewExecutor(toolRegistry)

	// 初始化 ContextBuilder（Phase 3）
	contextBuilder := service.NewContextBuilder(agentConfigs, convCtxService, memoryRetriever, retrievalService, toolRegistry)

	// 初始化 RouterService（Phase 4）
	// 使用用户配置的 ChatModelID 做路由判断
	llmRouter := service.NewLLMRouter(modelFactory)
	routerService := service.NewRouterService(llmRouter)

	// 初始化 MCP ImportService（用于 MCP Server 和工具管理）
	// 使用前面已创建的 mcpServers 和 mcpTools 仓库
	mcpImportService := service.NewImportService(mcpServers, mcpTools, cfg)
	mcpService := mcpImportService // 供 HTTP 路由使用

	// 注入第二层校验器：工具调用前实时校验 MCP 工具启用状态
	toolExecutor.SetAvailabilityChecker(mcpImportService)

	// 创建 MCP 工具刷新器：负责在 Agent 启动前动态加载用户已启用的 MCP 工具（第一层校验）
	mcpClient := mcp.NewMCPClient()
	mcpToolRefresher := tools.NewMCPToolRefresher(toolRegistry, mcpImportService, mcpClient)

	// 将 MCP 工具刷新器注入到 ContextBuilder，使其在构建 AgentContext 时自动刷新工具列表
	// contextBuilder 变量以 contracts.ContextBuilder 接口暴露，这里通过类型断言注入刷新器
	if cb, ok := contextBuilder.(interface{ SetMCPToolRefresher(*tools.MCPToolRefresher) }); ok {
		cb.SetMCPToolRefresher(mcpToolRefresher)
	}

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
	planExecutorService := service.NewPlanExecutorService(toolExecutor, planStateStore, modelFactory, cfg.Agent.MaxToolCalls, cfg.Agent.MaxToolResultBytes)
	_ = planExecutorService // 用于后续集成

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

	// 初始化事件发布器与订阅器（Phase 7）：使用 Redis Pub/Sub 实现实时事件推送。
	// eventPub 将 AgentEvent 序列化为 JSON 后发布到 Redis 频道；
	// eventSub 从同一频道过滤出指定 runID 的事件供 SSE 端点使用。
	eventPub := events.NewRedisEventPublisher(a.redis)
	eventSub := events.NewRedisEventSubscriber(a.redis)
	// 用带序列号的发布器包装底层 Redis 发布器，确保事件序号单调递增。
	sequencedEvents := core.NewSequencedEventPublisher(eventPub)

	// 初始化 ReactRunner（ReAct 模式执行器）
	reactRunner := core.NewReactRunner(reactService, sequencedEvents)

	// 初始化 PlanRunner（Plan-Execute 模式执行器），注入 sequencedEvents 用于实时推送计划与步骤事件。
	planRunner := core.NewPlanRunner(plannerService, planExecutorService, reviewerService, replanService, planStateStore, sequencedEvents)

	// 初始化 Agent 运行和工具调用 Repository
	agentRunRepo := repository.NewAgentRunRepository(a.db)
	toolCallRepo := repository.NewToolCallRepository(a.db)

	// 创建 Repository 适配器，使 repository.AgentRunRepository 适配 core.RunRepository 接口
	runRepoAdapter := &agentRunRepoAdapter{repo: agentRunRepo}

	// 初始化 Agent Core Service，作为 contracts.AgentRunService 的实现
	agentCoreService := core.NewService(reactRunner, routerService, runRepoAdapter, sequencedEvents)
	agentCoreService.SetPlanRunner(planRunner)

	// 初始化 Agent 运行管理的 HTTP 控制器，注入事件订阅器以支持 SSE 流式事件推送
	agentController := agent.NewController(
		agentCoreService,
		agentRunRepo,
		toolCallRepo,
		messages,
		agentConfigs,
		contextBuilder,
		eventSub,
	)

	// 初始化 Agent Worker（异步执行 Agent 运行的后台工作者）
	// Worker 周期性轮询数据库中状态为 queued 的运行记录，原子性领取后在后台 goroutine 中执行
	agentWorker := worker.NewAgentWorker(
		agentCoreService,
		agentRunRepo,
		messages,
		contextBuilder,
		worker.DefaultAgentWorkerConfig(),
	)
	// 使用独立生命周期上下文启动 Worker，服务器关闭时主动取消该上下文。
	workerCtx, workerCancel := context.WithCancel(context.Background())
	a.workerCancel = workerCancel
	go func() {
		if err := agentWorker.Run(workerCtx); err != nil {
			logger.Error("Agent Worker 运行异常", zap.Error(err))
		}
	}()

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
		Retrieval:       retrievalService,
		ContextBuilder:  contextBuilder,
		AssetSignKey:    cfg.JWT.Secret,
		Router:          routerService,
		MemoryRepo:      memoryRepo,
		AgentController: agentController,
		Conversations:   conversationService,
		Messages:        messages,
		PostgresHealth:  func(ctx context.Context) error { return database.CheckPostgres(ctx, a.db) },
		RedisHealth:     func(ctx context.Context) error { return database.CheckRedis(ctx, a.redis) },
		MinIOHealth:     a.store.Health,
		ParserHealth:    a.documentParser.Health,
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

// agentRunRepoAdapter 适配 repository.AgentRunRepository 到 core.RunRepository 接口。
// core.Service.Cancel 和 Retry 只依赖 Cancel 和 Retry 两个方法，无需暴露完整 Repository 给核心层。
type agentRunRepoAdapter struct {
	repo repository.AgentRunRepository
}

// Cancel 通过用户 ID 和运行 ID 取消运行，委托给 repository.MarkCancelled。
func (a *agentRunRepoAdapter) Cancel(ctx context.Context, runID, userID contracts.ID) error {
	uid, err := uuid.Parse(string(userID))
	if err != nil {
		return fmt.Errorf("解析用户 ID 失败: %w", err)
	}
	rid, err := uuid.Parse(string(runID))
	if err != nil {
		return fmt.Errorf("解析运行 ID 失败: %w", err)
	}
	return a.repo.MarkCancelled(ctx, uid, rid)
}

// Retry 基于失败运行创建新的排队运行，返回新运行 ID。
func (a *agentRunRepoAdapter) Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error) {
	uid, err := uuid.Parse(string(userID))
	if err != nil {
		return "", fmt.Errorf("解析用户 ID 失败: %w", err)
	}
	rid, err := uuid.Parse(string(runID))
	if err != nil {
		return "", fmt.Errorf("解析运行 ID 失败: %w", err)
	}
	newID, err := a.repo.CreateRetry(ctx, rid, uid)
	if err != nil {
		return "", err
	}
	return contracts.ID(newID.String()), nil
}

// Close 释放服务器应用持有的所有资源。
func (a *ServerApp) Close() error {
	var closeErr error
	// 先停止 Agent Worker，避免数据库和 Redis 关闭后仍有后台任务访问它们。
	if a.workerCancel != nil {
		a.workerCancel()
		a.workerCancel = nil
	}
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
