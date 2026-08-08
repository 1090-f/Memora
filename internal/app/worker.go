package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/internal/service/rag/asset"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	workerengine "github.com/1090-f/Memora/internal/worker"
	documentworker "github.com/1090-f/Memora/internal/worker/document"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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

	if err := a.registerDocumentJobs(ctx, cfg); err != nil {
		_ = a.Close()
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

// registerDocumentJobs 显式注册文档导入任务类型（不使用隐式 init()）。
func (a *WorkerApp) registerDocumentJobs(ctx context.Context, cfg *config.Config) error {
	importTasks := repository.NewImportTaskRepository(a.db)
	docs := repository.NewDocumentRepository(a.db)
	chunks := repository.NewDocumentChunkRepository(a.db)
	vectors := repository.NewVectorRepository(a.db)

	// 解析与分块配置来自统一 Config。
	parseOptions := parser.ParseOptions{
		SchemaVersion:   parser.SchemaVersion,
		OCRLanguages:    cfg.DocumentParser.OCRLanguages,
		DoOCR:           cfg.DocumentParser.DoOCR,
		TableStructure:  cfg.DocumentParser.TableStructure,
		ExtractPictures: cfg.DocumentParser.ExtractPictures,
		IncludeBBoxes:   cfg.DocumentParser.IncludeBBoxes,
	}
	chunkOptions := chunking.ChunkOptions{
		MaxTokens:       cfg.Chunking.MaxTokens,
		MinTokens:       cfg.Chunking.MinTokens,
		OverlapTokens:   cfg.Chunking.OverlapTokens,
		RepeatTableHead: cfg.Chunking.RepeatTableHead,
		StrategyVersion: cfg.Chunking.StrategyVersion,
	}

	// 构造并编译文档加工 Graph（初始化时 Compile 一次）。
	embedder := a.embeddingProvider()
	embeddingModelID := ""
	pipelineConfig := pipeline.DocumentPipelineConfig{
		Store:        &parserObjectStore{inner: a.store},
		Chunks:       chunks,
		ChunkConfig:  defaultChunkConfig,
		ChunkOptions: chunkOptions,
		Tokenizer:    chunking.NewHeuristicTokenizer(),
		ParseOptions: parseOptions,
		ParserConfig: parser.PythonParserConfig{
			BaseURL:          cfg.DocumentParser.BaseURL,
			Timeout:          cfg.DocumentParser.Timeout,
			MaxResponseBytes: cfg.DocumentParser.MaxResponseBytes,
		},
		ValidateLimits: parser.DefaultValidateLimits(),
		Vectors:        vectors,
	}
	if embedder != nil {
		pipelineConfig.Embedder = embedder
		pipelineConfig.EmbeddingModelID = embeddingModelID
		logger.Info("文档加工流水线已启用向量索引")
	} else {
		logger.Warn("Embedding 模型未就绪，文档加工流水线仅启用关键词索引")
	}
	switch cfg.AssetEnrichment.Mode {
	case "none", "":
		pipelineConfig.AssetEnricher = asset.NewNoopEnricher()
	default:
		return fmt.Errorf("不支持的 asset_enrichment.mode %q（当前仅支持 none）", cfg.AssetEnrichment.Mode)
	}
	documentPipeline, err := pipeline.NewDocumentPipeline(pipelineConfig)
	if err != nil {
		return fmt.Errorf("构造文档加工流水线失败: %w", err)
	}

	processService := service.NewDocumentProcessService(importTasks, docs, chunks, vectors, documentPipeline)

	source := documentworker.NewSource(importTasks)
	handler := documentworker.NewHandler(processService)

	// 恢复卡在 running 且超过租约的任务。
	recovered, err := handler.RecoverStale(ctx)
	if err != nil {
		return fmt.Errorf("恢复过期导入任务失败: %w", err)
	}
	if recovered > 0 {
		logger.Info("已恢复过期导入任务", zap.Int64("count", recovered))
	}

	return a.RegisterJob(documentworker.JobType, source, handler)
}

// embeddingProvider 返回 Eino Embedder；成员二的 ModelFactory 未接入时返回 nil。
// 维度冻结并接入成员二实现后替换此方法，并同步替换 pipeline 的 Tokenizer。
func (a *WorkerApp) embeddingProvider() embedding.Embedder {
	return nil
}

// defaultChunkConfig 是分段配置的稳定描述，用于计算 chunk_config_hash。
// 修改分段参数时必须同步更新此描述以触发重新索引。
const defaultChunkConfig = `{"splitter":"structure-aware","chunk_size_tokens":1000,"overlap_tokens":100,"min_tokens":100,"repeat_table_header":true}`

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

// parserObjectStore 将 pkg/objectstore.Client 适配为 parser.ObjectStore。
type parserObjectStore struct {
	inner *objectstore.Client
}

func (p *parserObjectStore) OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return p.inner.OpenObject(ctx, objectKey)
}

func (p *parserObjectStore) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	return p.inner.PutObject(ctx, objectKey, reader, size, contentType)
}

func (p *parserObjectStore) StatObject(ctx context.Context, objectKey string) (*parser.ObjectInfo, error) {
	info, err := p.inner.StatObject(ctx, objectKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, parser.ErrObjectNotFound
		}
		return nil, err
	}
	return &parser.ObjectInfo{
		Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag,
	}, nil
}

func (p *parserObjectStore) RemoveObject(ctx context.Context, objectKey string) error {
	return p.inner.RemoveObject(ctx, objectKey)
}

func (p *parserObjectStore) Bucket() string { return p.inner.Bucket() }
