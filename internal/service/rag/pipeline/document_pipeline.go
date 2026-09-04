// Package pipeline 定义文档加工与检索的 Eino 编排。
//
// 文档加工 Graph 在应用初始化时 Compile 一次并注入 Worker Handler。
// 节点流（见 docs/2026-08-08-docling-document-parsing-execution-plan.md）：
//
//	resolve_artifact → parse_if_missing → validate_parsed_document → ocr_assets → persist_artifact
//	→ document_normalize → asset_enrich → canonical_render → validate_canonical_document
//	→ persist_canonical_artifact → document_profile → canonical_chunk → chunk_clean
//	→ token_count → persist_chunks → embed_and_index
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/asset"
	"github.com/1090-f/Memora/internal/service/rag/canonical"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/indexing"
	"github.com/1090-f/Memora/internal/service/rag/loader"
	"github.com/1090-f/Memora/internal/service/rag/normalizer"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/transformer"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ChunkWriter 持久化 Chunk 的接口（由 Repository 实现）。
type ChunkWriter interface {
	// BatchInsert 在短事务中批量插入 document_chunks，返回各 Chunk 的 ID。
	// 顺序与输入 chunks 一致；冲突跳过时返回空 ID。
	BatchInsert(ctx context.Context, chunks []*entity.DocumentChunk) ([]string, error)
}

// ProcessInput 是文档加工 Graph 的输入。
type ProcessInput struct {
	// ObjectKey 是 MinIO 对象 key。
	ObjectKey string
	// SourceURL 是 URL 导入来源；与 ObjectKey 二选一。
	SourceURL string
	// Content 是手工文档（manual）的正文；非空时优先于 ObjectKey/SourceURL。
	Content string
	// FileName 是原始文件名。
	FileName string
	// MIMEType 是原始文件 MIME 类型（仅辅助判断）。
	MIMEType string
	// DocMeta 是稳定的业务元数据。
	DocMeta transformer.DocMeta
	// Attachments 是 zip 导入的附件映射（相对路径 → MinIO object key），可选。
	Attachments map[string]string
	// Embedder/EmbeddingModelID 是本任务按用户与知识库解析出的可选向量模型。
	Embedder         embedding.Embedder
	EmbeddingModelID string
	// ChunkStrategyOverride 是文档/知识库级显式策略覆盖；为空时使用流水线配置。
	ChunkStrategyOverride string
}

// ProcessOutput 是文档加工 Graph 的输出。
type ProcessOutput struct {
	// ChunkCount 是本次加工成功入库的 Chunk 数量。
	ChunkCount int
	FinalURL   string
	SourceHash string
	Title      string
	// Warnings 是解析/OCR 等阶段的非致命提示（如 unresolved 图片）。
	Warnings []string
	// CanonicalHash/CanonicalNodeCount 暴露影子 Canonical 阶段的稳定摘要，
	// 避免把整份 CanonicalDocument 复制到 Worker 输出。
	CanonicalHash        string
	CanonicalNodeCount   int
	ChunkStrategy        string
	ChunkStrategyVersion string
	// ChunkConfigHash 是纳入 Canonical 内容与最终路由决策后的文档级哈希。
	ChunkConfigHash string
	// ChunkDiffReport 仅在影子双跑开启时返回，不改变生产分块与索引内容。
	ChunkDiffReport *chunking.ChunkDiffReport
}

// DocumentPipelineConfig 定义文档加工流水线配置。
type DocumentPipelineConfig struct {
	// Store 是 MinIO 对象存储（原始文件读取 + Artifact 持久化）。
	Store parser.ObjectStore
	// Chunks 是 Chunk 持久化接口。
	Chunks ChunkWriter
	// ChunkConfig 是分段配置的稳定描述（参与 chunk_config_hash）。
	ChunkConfig string
	// ChunkOptions 是分块参数（参与 chunk_config_hash）。
	ChunkOptions chunking.ChunkOptions
	// Tokenizer 是与 Embedding 模型对齐的 token 计量器（参与 chunk_config_hash）。
	Tokenizer chunking.Tokenizer
	// ParseOptions 是解析选项（参与 parse_config_hash）。
	ParseOptions parser.ParseOptions
	// ParserConfig 是 Python document-parser 客户端配置。
	ParserConfig parser.PythonParserConfig
	// ValidateLimits 是 ParsedDocument 资源限制。
	ValidateLimits parser.ValidateLimits
	// Vectors 是向量持久化接口（可选：接入 Embedder 后启用向量索引）。
	Vectors repository.VectorRepository
	// EmbeddingModelID 是向量索引使用的模型配置 ID（参与 chunk_config_hash）。
	EmbeddingModelID string
	// Embedder 是 Eino Embedding 组件（可选）。
	Embedder embedding.Embedder
	// WebLoader 是 URL 来源使用的安全 Eino Loader。
	WebLoader document.Loader
	// AssetLoader 解析 Markdown 图片引用（nil 时使用默认实现：网络图片 + 无附件）。
	AssetLoader parser.AssetLoader
	// AssetEnricher 是图片资产增强器（nil 时使用 NoopEnricher）。
	AssetEnricher asset.Enricher
	// CanonicalRenderer 在 normalize/enrich 后生成稳定的分块中间表示。
	CanonicalRenderer canonical.Renderer
	// CanonicalValidator 校验 UTF-8 byte offsets、来源与内容哈希。
	CanonicalValidator canonical.Validator
	// CanonicalConfig 是 Renderer 配置的稳定描述（参与 chunk_config_hash）。
	CanonicalConfig string
	// EnableCanonicalChunkDiff 影子运行 legacy 分块器并与 Canonical 主链路生成差异报告。
	EnableCanonicalChunkDiff bool
	// ChunkStrategy controls deterministic routing: structured/paragraph/recursive_fallback/auto.
	ChunkStrategy string
}

// pipelineState 是 Graph 内部流转状态。
type pipelineState struct {
	input                   ProcessInput
	doc                     *parser.ParsedDocument
	canonical               *canonical.CanonicalDocument
	chunkDecision           chunking.ChunkDecision
	chunkDiff               *chunking.ChunkDiffReport
	chunks                  []chunking.ParsedChunk
	artifactPrefix          string
	canonicalArtifactPrefix string
	canonicalConfigHash     string
	canonicalCacheHit       bool
	chunkConfigHash         string
	computedHash            string
	// indexDocs 供向量索引节点使用（persist_chunks 后填充）。
	indexDocs     []*schema.Document
	loadedContent string
	stageStarted  map[contracts.DocumentStage]time.Time
}

func startDocumentStage(ctx context.Context, state *pipelineState, stage contracts.DocumentStage, summary string) {
	if state.stageStarted == nil {
		state.stageStarted = make(map[contracts.DocumentStage]time.Time)
	}
	started := time.Now().UTC()
	state.stageStarted[stage] = started
	contracts.ReportDocumentStage(ctx, stage, contracts.StageObservation{Stage: string(stage), Status: contracts.StageRunning, StartedAt: &started, Summary: summary})
}

func finishDocumentStage(ctx context.Context, state *pipelineState, stage contracts.DocumentStage, status contracts.StageStatus, summary string, metadata map[string]any) {
	ended := time.Now().UTC()
	started, ok := state.stageStarted[stage]
	if !ok {
		started = ended
	}
	duration := ended.Sub(started).Milliseconds()
	contracts.ReportDocumentStage(ctx, stage, contracts.StageObservation{Stage: string(stage), Status: status, StartedAt: &started, EndedAt: &ended, DurationMS: &duration, Summary: summary, Metadata: metadata})
}

// DocumentPipeline 是已编译的文档加工编排。
type DocumentPipeline struct {
	runnable         compose.Runnable[ProcessInput, ProcessOutput]
	chunkConfigHash  string
	embeddingModelID string
}

// NewDocumentPipeline 构造并编译文档加工 Graph。
func NewDocumentPipeline(cfg DocumentPipelineConfig) (*DocumentPipeline, error) {
	if cfg.Store == nil || cfg.Chunks == nil {
		return nil, fmt.Errorf("文档加工流水线缺少 Store 或 Chunks 依赖")
	}
	cfg = applyDefaults(cfg)

	parseConfigHash, err := parser.ParseConfigHash(cfg.ParseOptions)
	if err != nil {
		return nil, fmt.Errorf("计算解析配置哈希失败: %w", err)
	}
	parseRuntimeIdentity := parser.DefaultParseRuntimeIdentity()

	parserRouter := parser.NewParserRouter(parser.NewTextParser(64*1024*1024), parser.NewMarkdownParser(64*1024*1024), newPythonParser(cfg))
	artifactStore := parser.NewArtifactStore(cfg.Store, cfg.ValidateLimits)
	canonicalArtifactStore := canonical.NewArtifactStore(cfg.Store)
	docNormalizer := normalizer.NewDocumentNormalizer()
	enricher := cfg.AssetEnricher
	if enricher == nil {
		enricher = asset.NewNoopEnricher()
	}
	canonicalRenderer := cfg.CanonicalRenderer
	canonicalValidator := cfg.CanonicalValidator
	ocrClient := parser.NewPythonOcrClient(cfg.ParserConfig.BaseURL, cfg.ParserConfig.Timeout)
	assetLoader := cfg.AssetLoader
	if assetLoader == nil {
		assetLoader = loader.NewMarkdownAssetLoader(cfg.Store, nil)
	}
	// 任务级附件 loader：zip 导入时以附件映射替换默认 loader。
	assetLoaderForTask := func(attachments map[string]string) parser.AssetLoader {
		if len(attachments) > 0 {
			return loader.NewMarkdownAssetLoader(cfg.Store, attachments)
		}
		return assetLoader
	}
	chunker := chunking.NewStructureAwareChunker(cfg.Tokenizer, cfg.ChunkOptions.StrategyVersion)
	chunkRouter := chunking.NewStrategyRouter(chunking.DefaultRouterConfig())
	chunkCleaner := transformer.NewChunkCleaner()

	chunkConfigHash := computeChunkConfigHash(cfg)

	g := compose.NewGraph[ProcessInput, ProcessOutput]()

	// load_source：URL 来源在 Worker 内通过安全 Eino Loader 抓取；文件来源保持 MinIO 流；
	// 手工文档（manual）直接使用正文 Content，不访问 MinIO。
	loadSourceLambda := compose.InvokableLambda(func(ctx context.Context, input ProcessInput) (*pipelineState, error) {
		state := &pipelineState{input: input, stageStarted: make(map[contracts.DocumentStage]time.Time)}
		startDocumentStage(ctx, state, contracts.DocumentStageParse, "正在加载并解析文档")
		if input.Content != "" {
			state.loadedContent = input.Content
			if state.input.FileName == "" {
				state.input.FileName = "manual.txt"
			}
			return state, nil
		}
		if input.ObjectKey == "" && input.SourceURL != "" {
			if cfg.WebLoader == nil {
				return nil, fmt.Errorf("URL 导入未配置安全 WebLoader")
			}
			docs, err := cfg.WebLoader.Load(ctx, document.Source{URI: input.SourceURL})
			if err != nil {
				return nil, fmt.Errorf("安全抓取 URL 失败: %w", err)
			}
			if len(docs) != 1 || strings.TrimSpace(docs[0].Content) == "" {
				return nil, fmt.Errorf("URL Loader 返回内容数量异常")
			}
			state.loadedContent = docs[0].Content
			state.input.FileName = "web.md"
			if title := einoadapter.GetMetaString(docs[0].MetaData, "title"); title != "" {
				state.input.FileName = title + ".md"
			}
			if hash := einoadapter.GetMetaString(docs[0].MetaData, "source_hash"); hash != "" {
				state.input.DocMeta.SourceHash = hash
			}
			if state.input.DocMeta.SourceLocation == nil {
				state.input.DocMeta.SourceLocation = make(map[string]any)
			}
			for _, key := range []string{"source_url", "final_url", "title", "content_type", "fetched_at"} {
				if value := einoadapter.GetMetaAny(docs[0].MetaData, key); value != nil {
					state.input.DocMeta.SourceLocation[key] = value
				}
			}
		}
		return state, nil
	})
	if err := g.AddLambdaNode("load_source", loadSourceLambda); err != nil {
		return nil, fmt.Errorf("注册 load_source 节点失败: %w", err)
	}

	// resolve_artifact：确定性 key 查找兼容 Artifact；命中则加载，未命中标记待解析。
	// 附件场景（zip/补传图片）强制重新解析：附件变化后旧 Artifact 不含新图片，
	// 若命中缓存会导致补传图片不生效。
	resolveLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		state.artifactPrefix = parser.ArtifactKeyPrefix(
			state.input.DocMeta.UserID, state.input.DocMeta.DocumentID,
			state.input.DocMeta.ContentVersion, parseConfigHash)
		if len(state.input.Attachments) > 0 {
			return state, nil
		}
		ref, err := artifactStore.ResolveWithIdentity(
			ctx, state.artifactPrefix, state.input.DocMeta.SourceHash, parseRuntimeIdentity,
		)
		if err != nil {
			if isArtifactNotFound(err) {
				return state, nil
			}
			return nil, fmt.Errorf("查找 Parsed Artifact 失败: %w", err)
		}
		doc, err := artifactStore.Load(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("加载 Parsed Artifact 失败: %w", err)
		}
		state.doc = doc
		return state, nil
	})
	if err := g.AddLambdaNode("resolve_artifact", resolveLambda); err != nil {
		return nil, fmt.Errorf("注册 resolve_artifact 节点失败: %w", err)
	}

	// parse_if_missing：Artifact 不存在时调用对应 Parser（TXT/MD→Go，PDF/DOCX→Python）。
	parseLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if state.doc != nil {
			return state, nil
		}
		var reader io.ReadCloser
		if state.loadedContent != "" {
			reader = io.NopCloser(strings.NewReader(state.loadedContent))
		} else {
			var err error
			reader, err = cfg.Store.OpenObject(ctx, state.input.ObjectKey)
			if err != nil {
				return nil, fmt.Errorf("打开原始文件失败: %w", err)
			}
		}
		hashed := parser.NewHashReader(reader)
		doc, err := parserRouter.Parse(ctx, parser.ParseInput{
			FileName:    state.input.FileName,
			Content:     hashed,
			Size:        -1,
			Options:     cfg.ParseOptions,
			AssetLoader: assetLoaderForTask(state.input.Attachments),
		})
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("解析文档失败: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("关闭原始文件失败: %w", closeErr)
		}
		state.computedHash = hashed.Sum()
		state.doc = doc
		return state, nil
	})
	if err := g.AddLambdaNode("parse_if_missing", parseLambda); err != nil {
		return nil, fmt.Errorf("注册 parse_if_missing 节点失败: %w", err)
	}

	// validate_parsed_document：schema、源哈希、引用与资源限制校验。
	validateLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if err := parser.ValidateParsedDocument(state.doc, state.computedHash, cfg.ValidateLimits); err != nil {
			return nil, err
		}
		return state, nil
	})
	if err := g.AddLambdaNode("validate_parsed_document", validateLambda); err != nil {
		return nil, fmt.Errorf("注册 validate_parsed_document 节点失败: %w", err)
	}

	// ocr_assets：对图片资产做 OCR（可选），结果写 asset.metadata["ocr_text"]；
	// 位于 persist_artifact 之前，OCR 文本随 Artifact 持久化，重试可复用。
	// 单图失败仅追加 warning，不阻断文档加工。
	ocrLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if !cfg.ParseOptions.DoImageOCR || state.doc == nil || ocrClient == nil {
			return state, nil
		}
		for i := range state.doc.Assets {
			asset := &state.doc.Assets[i]
			if asset.Omitted || asset.Kind != "picture" || strings.TrimSpace(asset.DataBase64) == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(asset.DataBase64)
			if err != nil {
				state.doc.Warnings = append(state.doc.Warnings, fmt.Sprintf("图片 %s OCR 跳过: base64 解码失败", asset.ID))
				continue
			}
			result, err := ocrClient.OcrImage(ctx, data, cfg.ParseOptions.OCRLanguages)
			if err != nil {
				state.doc.Warnings = append(state.doc.Warnings, fmt.Sprintf("图片 %s OCR 失败: %v", asset.ID, err))
				continue
			}
			text := strings.TrimSpace(strings.Join(result.Lines, "\n"))
			if text == "" {
				continue
			}
			if asset.Metadata == nil {
				asset.Metadata = make(map[string]any)
			}
			asset.Metadata["ocr_text"] = text
			// caption 为空或仅通用编号（如 "图 1"）时用 OCR 文字回填，
			// 提升图片 alt 展示与检索文本质量。
			caption := strings.TrimSpace(asset.Caption)
			if caption == "" || genericFigureCaption(caption) {
				asset.Caption = text
			}
		}
		return state, nil
	})
	if err := g.AddLambdaNode("ocr_assets", ocrLambda); err != nil {
		return nil, fmt.Errorf("注册 ocr_assets 节点失败: %w", err)
	}

	// persist_artifact：assets → parsed-document → manifest 原子完成。
	persistArtifactLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if _, err := artifactStore.Save(ctx, state.artifactPrefix, state.doc, parseConfigHash); err != nil {
			return nil, fmt.Errorf("保存 Parsed Artifact 失败: %w", err)
		}
		finishDocumentStage(ctx, state, contracts.DocumentStageParse, contracts.StageSucceeded, "文档解析完成", map[string]any{"warning_count": len(state.doc.Warnings)})
		return state, nil
	})
	if err := g.AddLambdaNode("persist_artifact", persistArtifactLambda); err != nil {
		return nil, fmt.Errorf("注册 persist_artifact 节点失败: %w", err)
	}

	// document_normalize：分块前规范化。
	normalizeLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		startDocumentStage(ctx, state, contracts.DocumentStageNormalize, "正在规范化文档结构")
		if err := docNormalizer.Normalize(ctx, state.doc); err != nil {
			return nil, fmt.Errorf("文档规范化失败: %w", err)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("document_normalize", normalizeLambda); err != nil {
		return nil, fmt.Errorf("注册 document_normalize 节点失败: %w", err)
	}

	// asset_enrich：图片资产二次增强（默认 Noop）。
	enrichLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if err := enricher.Enrich(ctx, state.doc); err != nil {
			return nil, fmt.Errorf("资产增强失败: %w", err)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("asset_enrich", enrichLambda); err != nil {
		return nil, fmt.Errorf("注册 asset_enrich 节点失败: %w", err)
	}

	// canonical_render：优先复用独立 Canonical Artifact；未命中时再执行确定性渲染。
	canonicalRenderLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		configHash, err := canonical.ArtifactConfigHash(state.doc, canonicalRenderer.Info(), cfg.CanonicalConfig)
		if err != nil {
			return nil, fmt.Errorf("计算 Canonical Artifact 配置哈希失败: %w", err)
		}
		state.canonicalConfigHash = configHash
		state.canonicalArtifactPrefix = canonical.ArtifactKeyPrefix(state.artifactPrefix, configHash)
		ref, err := canonicalArtifactStore.Resolve(ctx, state.canonicalArtifactPrefix, configHash, canonicalRenderer.Info())
		if err == nil {
			doc, loadErr := canonicalArtifactStore.Load(ctx, ref)
			if loadErr != nil {
				return nil, fmt.Errorf("加载 Canonical Artifact 失败: %w", loadErr)
			}
			state.canonical = doc
			state.canonicalCacheHit = true
			return state, nil
		}
		if !errors.Is(err, canonical.ErrArtifactNotFound) {
			return nil, fmt.Errorf("查找 Canonical Artifact 失败: %w", err)
		}
		doc, err := canonicalRenderer.Render(ctx, state.doc)
		if err != nil {
			return nil, fmt.Errorf("渲染 CanonicalDocument 失败: %w", err)
		}
		state.canonical = doc
		return state, nil
	})
	if err := g.AddLambdaNode("canonical_render", canonicalRenderLambda); err != nil {
		return nil, fmt.Errorf("注册 canonical_render 节点失败: %w", err)
	}

	// validate_canonical_document：验证 byte offsets、来源映射和稳定 hash。
	canonicalValidateLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if err := canonicalValidator.Validate(state.canonical); err != nil {
			return nil, fmt.Errorf("校验 CanonicalDocument 失败: %w", err)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("validate_canonical_document", canonicalValidateLambda); err != nil {
		return nil, fmt.Errorf("注册 validate_canonical_document 节点失败: %w", err)
	}

	// persist_canonical_artifact：结构化文档、Markdown 与 SourceMap 分开保存，manifest 最后落盘。
	persistCanonicalArtifactLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		if state.canonicalCacheHit {
			return state, nil
		}
		if _, err := canonicalArtifactStore.Save(
			ctx, state.canonicalArtifactPrefix, state.canonical, state.canonicalConfigHash, canonicalRenderer.Info(),
		); err != nil {
			return nil, fmt.Errorf("保存 Canonical Artifact 失败: %w", err)
		}
		return state, nil
	})
	if err := g.AddLambdaNode("persist_canonical_artifact", persistCanonicalArtifactLambda); err != nil {
		return nil, fmt.Errorf("注册 persist_canonical_artifact 节点失败: %w", err)
	}

	// document_profile：提取确定性策略路由特征；当前仍使用 Structured 主策略。
	profileLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		profile, err := canonical.Profile(state.canonical, state.doc, cfg.Tokenizer)
		if err != nil {
			return nil, fmt.Errorf("生成 DocumentProfile 失败: %w", err)
		}
		state.canonical.Profile = profile
		finishDocumentStage(ctx, state, contracts.DocumentStageNormalize, contracts.StageSucceeded, "文档规范化完成", map[string]any{"canonical_cache_hit": state.canonicalCacheHit})
		return state, nil
	})
	if err := g.AddLambdaNode("document_profile", profileLambda); err != nil {
		return nil, fmt.Errorf("注册 document_profile 节点失败: %w", err)
	}

	// chunk_strategy_route：固定策略或基于 Profile 的确定性 auto 路由。
	routeLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		startDocumentStage(ctx, state, contracts.DocumentStageChunk, "正在生成检索分块")
		decision, err := chunkRouter.RouteWithOverride(
			state.canonical.Profile, cfg.ChunkStrategy, state.input.ChunkStrategyOverride,
		)
		if err != nil {
			return nil, fmt.Errorf("选择分块策略失败: %w", err)
		}
		state.chunkDecision = decision
		state.chunkConfigHash = computeDocumentChunkConfigHash(chunkConfigHash, state.canonical.ContentHash, decision)
		return state, nil
	})
	if err := g.AddLambdaNode("chunk_strategy_route", routeLambda); err != nil {
		return nil, fmt.Errorf("注册 chunk_strategy_route 节点失败: %w", err)
	}

	// canonical_chunk：按 Router 决策消费 CanonicalDocument 分块。
	chunkLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		var chunks []chunking.ParsedChunk
		var err error
		if cfg.EnableCanonicalChunkDiff {
			legacyChunks, legacyErr := chunker.Chunk(ctx, state.doc, cfg.ChunkOptions)
			if legacyErr != nil {
				return nil, fmt.Errorf("旧分块器影子运行失败: %w", legacyErr)
			}
			candidateChunks, candidateErr := chunker.ChunkCanonicalStrategy(ctx, state.canonical, cfg.ChunkOptions, state.chunkDecision.Strategy)
			if candidateErr != nil {
				return nil, fmt.Errorf("Canonical 分块器影子运行失败: %w", candidateErr)
			}
			report := chunking.CompareChunks(
				legacyChunks, candidateChunks, state.chunkDecision.Strategy, state.chunkDecision.Version,
			)
			state.chunkDiff = &report
			chunks = candidateChunks
		} else {
			chunks, err = chunker.ChunkCanonicalStrategy(ctx, state.canonical, cfg.ChunkOptions, state.chunkDecision.Strategy)
		}
		if err != nil {
			return nil, fmt.Errorf("Canonical 分块失败: %w", err)
		}
		// 纯图片文档（图片无 OCR/caption 文字）没有可索引文本，允许 0 Chunk 成功
		// 导入（资产与原文件保留）；有正文却分不出 Chunk 才是分块器 bug。
		if len(chunks) == 0 && !assetOnlyDocument(state.doc) {
			return nil, fmt.Errorf("文档未产生任何 Chunk")
		}
		for i := range chunks {
			chunks[i].SourceSpans = canonical.SelectChunkSourceSpans(
				state.canonical, chunks[i].Content, chunks[i].BlockIDs, chunks[i].TableRefs, chunks[i].AssetRefs,
			)
		}
		chunking.MarkOverlapSpans(chunks)
		for i := range chunks {
			decision := state.chunkDecision
			chunks[i].Decision = &decision
		}
		state.chunks = chunks
		return state, nil
	})
	if err := g.AddLambdaNode("canonical_chunk", chunkLambda); err != nil {
		return nil, fmt.Errorf("注册 canonical_chunk 节点失败: %w", err)
	}

	// chunk_clean：分块后清理（不改变 Chunk 边界）。
	cleanLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		cleaned, err := chunkCleaner.Clean(state.chunks, cfg.ChunkOptions.MaxTokens, cfg.Tokenizer.Count)
		if err != nil {
			return nil, fmt.Errorf("Chunk 清理失败: %w", err)
		}
		state.chunks = cleaned
		return state, nil
	})
	if err := g.AddLambdaNode("chunk_clean", cleanLambda); err != nil {
		return nil, fmt.Errorf("注册 chunk_clean 节点失败: %w", err)
	}

	// token_count：后置 TokenCounter 只记录 token_count，不切分不合并。
	countLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		for i := range state.chunks {
			tokens, err := cfg.Tokenizer.Count(state.chunks[i].Content)
			if err != nil {
				return nil, fmt.Errorf("统计 Chunk %d token 数失败: %w", i, err)
			}
			state.chunks[i].TokenCount = tokens
			if tokens > cfg.ChunkOptions.MaxTokens {
				return nil, fmt.Errorf("Chunk %d 超出 MaxTokens（%d > %d），分块器存在 bug", i, tokens, cfg.ChunkOptions.MaxTokens)
			}
		}
		return state, nil
	})
	if err := g.AddLambdaNode("token_count", countLambda); err != nil {
		return nil, fmt.Errorf("注册 token_count 节点失败: %w", err)
	}

	// persist_chunks：ParsedChunk → entity.DocumentChunk → BatchInsert。
	persistChunksLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		// 纯图片文档无 Chunk：跳过落库，indexDocs 保持为空。
		if len(state.chunks) == 0 {
			finishDocumentStage(ctx, state, contracts.DocumentStageChunk, contracts.StageSucceeded, "文档无需生成文本分块", map[string]any{"chunk_count": 0})
			return state, nil
		}
		entities := make([]*entity.DocumentChunk, 0, len(state.chunks))
		for i, chunk := range state.chunks {
			entityChunk, err := chunkToEntity(chunk, i, state.input.DocMeta, state.chunkConfigHash)
			if err != nil {
				return nil, err
			}
			entities = append(entities, entityChunk)
		}
		ids, err := cfg.Chunks.BatchInsert(ctx, entities)
		if err != nil {
			return nil, fmt.Errorf("持久化 Chunk 失败: %w", err)
		}
		inserted := 0
		for i, id := range ids {
			if id == "" {
				continue
			}
			inserted++
			state.indexDocs = append(state.indexDocs, indexDocumentFromChunk(&state.chunks[i], id, state.input.DocMeta, state.input.EmbeddingModelID))
		}
		if inserted == 0 {
			return nil, fmt.Errorf("未插入任何 Chunk")
		}
		finishDocumentStage(ctx, state, contracts.DocumentStageChunk, contracts.StageSucceeded, "文档分块完成", map[string]any{"chunk_count": inserted})
		return state, nil
	})
	if err := g.AddLambdaNode("persist_chunks", persistChunksLambda); err != nil {
		return nil, fmt.Errorf("注册 persist_chunks 节点失败: %w", err)
	}

	// 节点连线。
	if err := g.AddEdge(compose.START, "load_source"); err != nil {
		return nil, err
	}
	chain := []string{
		"load_source", "resolve_artifact", "parse_if_missing", "validate_parsed_document", "ocr_assets", "persist_artifact",
		"document_normalize", "asset_enrich", "canonical_render", "validate_canonical_document", "persist_canonical_artifact", "document_profile", "chunk_strategy_route",
		"canonical_chunk", "chunk_clean",
		"token_count", "persist_chunks",
	}
	for i := 0; i+1 < len(chain); i++ {
		if err := g.AddEdge(chain[i], chain[i+1]); err != nil {
			return nil, fmt.Errorf("连接 %s→%s 失败: %w", chain[i], chain[i+1], err)
		}
	}

	// 向量索引（可选）：模型按任务动态注入；无模型时保留关键词索引能力。
	if cfg.Vectors != nil {
		vectorIndexer, err := indexing.NewPostgresIndexer(indexing.PostgresIndexerConfig{
			EmbeddingModelID: cfg.EmbeddingModelID,
			Repository:       cfg.Vectors,
		})
		if err != nil {
			return nil, fmt.Errorf("构造向量索引器失败: %w", err)
		}
		embedAndIndex := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (ProcessOutput, error) {
			embedder, modelID := state.input.Embedder, state.input.EmbeddingModelID
			if embedder == nil {
				embedder, modelID = cfg.Embedder, cfg.EmbeddingModelID
			}
			if embedder == nil || len(state.indexDocs) == 0 {
				finishDocumentStage(ctx, state, contracts.DocumentStageEmbed, contracts.StageSkipped, "未配置向量模型或没有可向量化分块", map[string]any{"chunk_count": len(state.indexDocs)})
				return processOutput(state, len(state.indexDocs)), nil
			}
			startDocumentStage(ctx, state, contracts.DocumentStageEmbed, "正在生成并保存文档向量")
			for _, doc := range state.indexDocs {
				einoadapter.SetMetaString(doc, einoadapter.MetaEmbeddingModelID, modelID)
			}
			ids, err := vectorIndexer.Store(ctx, state.indexDocs, indexer.WithEmbedding(embedder))
			if err != nil {
				return ProcessOutput{}, fmt.Errorf("向量索引失败: %w", err)
			}
			finishDocumentStage(ctx, state, contracts.DocumentStageEmbed, contracts.StageSucceeded, "文档向量化完成", map[string]any{"vector_count": len(ids), "embedding_model_id": modelID})
			return processOutput(state, len(ids)), nil
		})
		if err := g.AddLambdaNode("embed_and_index", embedAndIndex); err != nil {
			return nil, err
		}
		if err := g.AddEdge("persist_chunks", "embed_and_index"); err != nil {
			return nil, err
		}
		if err := g.AddEdge("embed_and_index", compose.END); err != nil {
			return nil, err
		}
	} else {
		finalize := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (ProcessOutput, error) {
			finishDocumentStage(ctx, state, contracts.DocumentStageEmbed, contracts.StageSkipped, "向量索引未启用", map[string]any{"chunk_count": len(state.chunks)})
			return processOutput(state, len(state.chunks)), nil
		})
		if err := g.AddLambdaNode("finalize", finalize); err != nil {
			return nil, err
		}
		if err := g.AddEdge("persist_chunks", "finalize"); err != nil {
			return nil, err
		}
		if err := g.AddEdge("finalize", compose.END); err != nil {
			return nil, err
		}
	}

	runnable, err := g.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译文档加工 Graph 失败: %w", err)
	}
	return &DocumentPipeline{
		runnable:         runnable,
		chunkConfigHash:  chunkConfigHash,
		embeddingModelID: cfg.EmbeddingModelID,
	}, nil
}

func processOutput(state *pipelineState, chunkCount int) ProcessOutput {
	output := ProcessOutput{ChunkCount: chunkCount, SourceHash: state.input.DocMeta.SourceHash}
	if state.canonical != nil {
		output.CanonicalHash = state.canonical.ContentHash
		output.CanonicalNodeCount = len(state.canonical.Nodes)
	}
	output.ChunkStrategy = state.chunkDecision.Strategy
	output.ChunkStrategyVersion = state.chunkDecision.Version
	output.ChunkConfigHash = state.chunkConfigHash
	output.ChunkDiffReport = state.chunkDiff
	if state.doc != nil && len(state.doc.Warnings) > 0 {
		output.Warnings = append([]string(nil), state.doc.Warnings...)
	}
	if state.input.DocMeta.SourceLocation != nil {
		if value, ok := state.input.DocMeta.SourceLocation["final_url"].(string); ok {
			output.FinalURL = value
		}
		if value, ok := state.input.DocMeta.SourceLocation["title"].(string); ok {
			output.Title = value
		}
	}
	return output
}

// applyDefaults 填充缺省配置。
func applyDefaults(cfg DocumentPipelineConfig) DocumentPipelineConfig {
	if cfg.ChunkOptions.StrategyVersion == "" {
		cfg.ChunkOptions.StrategyVersion = "structure-v1"
	}
	if cfg.ChunkOptions.MaxTokens <= 0 {
		cfg.ChunkOptions.MaxTokens = 1000
	}
	if cfg.Tokenizer == nil {
		cfg.Tokenizer = chunking.NewHeuristicTokenizer()
	}
	if cfg.CanonicalRenderer == nil {
		cfg.CanonicalRenderer = canonical.NewParsedDocumentRenderer(canonical.RenderOptions{})
	}
	if cfg.CanonicalValidator == nil {
		cfg.CanonicalValidator = canonical.NewValidator()
	}
	if cfg.ChunkStrategy == "" {
		cfg.ChunkStrategy = chunking.StrategyAuto
	}
	if cfg.ValidateLimits.MaxBlocks == 0 {
		cfg.ValidateLimits = parser.DefaultValidateLimits()
	}
	if cfg.ParserConfig.Timeout <= 0 {
		cfg.ParserConfig.Timeout = 8 * time.Minute
	}
	return cfg
}

// assetOnlyDocument 判断文档是否没有任何可索引文本：只有标题/图片（且图片无
// OCR/caption 文字，分块器因此不产出单元）或完全为空。这类文档允许 0 Chunk
// 成功导入（资产与原文件保留），避免把"无文字图片"当成分块器故障。
func assetOnlyDocument(doc *parser.ParsedDocument) bool {
	if doc == nil {
		return false
	}
	for _, block := range doc.Blocks {
		switch block.Type {
		case parser.BlockTypeHeading, parser.BlockTypeTitle, parser.BlockTypePicture:
			continue
		default:
			if strings.TrimSpace(block.Text) != "" || block.TableRef != "" || len(block.AssetRefs) > 0 {
				return false
			}
		}
	}
	return true
}

// computeChunkConfigHash 计算 chunk_config_hash：分块参数 + tokenizer + Embedding 模型。
// 任一变化都会导致重新分块（不触发重新解析）。
func computeChunkConfigHash(cfg DocumentPipelineConfig) string {
	payload := map[string]any{
		"chunk_config":       cfg.ChunkConfig,
		"strategy_version":   cfg.ChunkOptions.StrategyVersion,
		"max_tokens":         cfg.ChunkOptions.MaxTokens,
		"min_tokens":         cfg.ChunkOptions.MinTokens,
		"overlap_tokens":     cfg.ChunkOptions.OverlapTokens,
		"repeat_table_head":  cfg.ChunkOptions.RepeatTableHead,
		"tokenizer":          cfg.Tokenizer.Name(),
		"embedding_model_id": cfg.EmbeddingModelID,
		"canonical_renderer": cfg.CanonicalRenderer.Info().Identity(),
		"canonical_config":   cfg.CanonicalConfig,
		"chunk_strategy":     cfg.ChunkStrategy,
		"chunk_router":       chunking.RouterVersion,
		"paragraph_chunker":  chunking.ParagraphVersion,
		"recursive_chunker":  chunking.RecursiveVersion,
	}
	data, _ := json.Marshal(payload)
	return sha256Hex(data)
}

// computeDocumentChunkConfigHash 将实际 Canonical 内容和最终路由决策加入基础配置哈希。
// 同一 Pipeline 下，人工覆盖或 auto 路由结果不同的文档不会共享错误的配置身份。
func computeDocumentChunkConfigHash(baseHash, canonicalHash string, decision chunking.ChunkDecision) string {
	payload := struct {
		BaseHash      string                 `json:"base_hash"`
		CanonicalHash string                 `json:"canonical_hash"`
		Decision      chunking.ChunkDecision `json:"decision"`
	}{BaseHash: baseHash, CanonicalHash: canonicalHash, Decision: decision}
	data, _ := json.Marshal(payload)
	return sha256Hex(data)
}

// newPythonParser 构造 Python 解析客户端；BaseURL 为空时解析 PDF/DOCX 报错。
func newPythonParser(cfg DocumentPipelineConfig) parser.Parser {
	client, err := parser.NewPythonDocumentParser(cfg.ParserConfig)
	if err != nil {
		return &unavailableParser{err: err}
	}
	return client
}

// unavailableParser 是 Python 服务不可用时的占位 Parser（失败不静默回退）。
type unavailableParser struct {
	err error
}

func (p *unavailableParser) Parse(context.Context, parser.ParseInput) (*parser.ParsedDocument, error) {
	return nil, fmt.Errorf("Python 解析服务不可用: %w", p.err)
}

// chunkToEntity 将 ParsedChunk 转换为 document_chunks 实体。
func chunkToEntity(chunk chunking.ParsedChunk, chunkNo int, meta transformer.DocMeta, chunkConfigHash string) (*entity.DocumentChunk, error) {
	headingPath, err := json.Marshal(chunk.HeadingPath)
	if err != nil {
		return nil, fmt.Errorf("序列化 heading_path 失败: %w", err)
	}
	sourceLocation, err := json.Marshal(sourceLocationMap(chunk, meta.SourceLocation))
	if err != nil {
		return nil, fmt.Errorf("序列化 source_location 失败: %w", err)
	}
	contextTitle := ""
	if len(chunk.HeadingPath) > 0 {
		contextTitle = chunk.HeadingPath[len(chunk.HeadingPath)-1]
	} else if meta.DocumentTitle != "" {
		contextTitle = meta.DocumentTitle
	}
	return &entity.DocumentChunk{
		UserID:          meta.UserID,
		KnowledgeBaseID: meta.KnowledgeBaseID,
		DocumentID:      meta.DocumentID,
		ChunkNo:         chunkNo,
		Content:         chunk.Content,
		CharCount:       len([]rune(chunk.Content)),
		TokenCount:      chunk.TokenCount,
		ContextTitle:    stringPtrOrNil(contextTitle),
		HeadingPath:     headingPath,
		SourceLocation:  sourceLocation,
		ContentVersion:  meta.ContentVersion,
		ChunkVersion:    meta.ChunkVersion,
		IndexVersion:    meta.IndexVersion,
		ChunkConfigHash: chunkConfigHash,
	}, nil
}

// sourceLocationMap 将 ParsedChunk 的来源信息编码为 jsonb 结构。
func sourceLocationMap(chunk chunking.ParsedChunk, base map[string]any) map[string]any {
	location := make(map[string]any, len(base)+6)
	for key, value := range base {
		location[key] = value
	}
	if chunk.SourceLocation.Page > 0 {
		location["page"] = chunk.SourceLocation.Page
	}
	if len(chunk.SourceLocation.BBox) == 4 {
		location["bbox"] = chunk.SourceLocation.BBox
	}
	if len(chunk.BlockIDs) > 0 {
		location["block_ids"] = chunk.BlockIDs
	}
	if len(chunk.TableRefs) > 0 {
		location["table_refs"] = chunk.TableRefs
	}
	if len(chunk.AssetRefs) > 0 {
		location["asset_refs"] = chunk.AssetRefs
	}
	if len(chunk.ContentTypes) > 0 {
		location["content_types"] = chunk.ContentTypes
	}
	if len(chunk.SourceSpans) > 0 {
		location["source_spans"] = chunk.SourceSpans
	}
	if chunk.Strategy != "" {
		location["strategy"] = chunk.Strategy
	}
	if chunk.StrategyVersion != "" {
		location["strategy_version"] = chunk.StrategyVersion
	}
	if chunk.Decision != nil {
		location["decision"] = chunk.Decision
	}
	return location
}

// indexDocumentFromChunk 为向量索引构造 Eino schema.Document。
func indexDocumentFromChunk(chunk *chunking.ParsedChunk, chunkID string, meta transformer.DocMeta, embeddingModelID string) *schema.Document {
	_ = chunk
	return &schema.Document{
		ID:      chunkID,
		Content: chunk.Content,
		MetaData: map[string]any{
			einoadapter.MetaUserID:           meta.UserID,
			einoadapter.MetaKnowledgeBase:    meta.KnowledgeBaseID,
			einoadapter.MetaDocumentID:       meta.DocumentID,
			einoadapter.MetaChunkID:          chunkID,
			einoadapter.MetaIndexVersion:     meta.IndexVersion,
			einoadapter.MetaEmbeddingModelID: embeddingModelID,
		},
	}
}

// isArtifactNotFound 判断 Artifact 查找结果。
func isArtifactNotFound(err error) bool {
	return err == parser.ErrArtifactNotFound
}

// genericFigureCaptionRe 匹配仅含通用编号的图片 caption（如 "图 1"、"图片"）。
var genericFigureCaptionRe = regexp.MustCompile(`^(图|图片)\s*\d*$`)

// genericFigureCaption 判断 caption 是否仅通用编号、不含描述性文字。
func genericFigureCaption(caption string) bool {
	return genericFigureCaptionRe.MatchString(strings.TrimSpace(caption))
}

// stringPtrOrNil 将空串转换为 nil 指针。
func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// sha256Hex 计算 sha256 十六进制。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ChunkConfigHash 返回当前分块配置的稳定哈希。
func (p *DocumentPipeline) ChunkConfigHash() string { return p.chunkConfigHash }

// EmbeddingModelID 返回向量索引使用的模型配置 ID；未启用时返回空。
func (p *DocumentPipeline) EmbeddingModelID() string { return p.embeddingModelID }

// Run 执行一次文档加工。
func (p *DocumentPipeline) Run(ctx context.Context, input ProcessInput) (ProcessOutput, error) {
	return p.runnable.Invoke(ctx, input)
}
