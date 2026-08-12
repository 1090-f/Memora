// Package pipeline 定义文档加工与检索的 Eino 编排。
//
// 文档加工 Graph 在应用初始化时 Compile 一次并注入 Worker Handler。
// 节点流（见 docs/2026-08-08-docling-document-parsing-execution-plan.md）：
//
//	resolve_artifact → parse_if_missing → validate_parsed_document → ocr_assets → persist_artifact
//	→ document_normalize → asset_enrich → structure_chunk → chunk_clean
//	→ token_count → persist_chunks → embed_and_index
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/asset"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/indexing"
	"github.com/1090-f/Memora/internal/service/rag/loader"
	"github.com/1090-f/Memora/internal/service/rag/normalizer"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/tokenizer"
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
}

// pipelineState 是 Graph 内部流转状态。
type pipelineState struct {
	input          ProcessInput
	doc            *parser.ParsedDocument
	chunks         []chunking.ParsedChunk
	artifactPrefix string
	computedHash   string
	// indexDocs 供向量索引节点使用（persist_chunks 后填充）。
	indexDocs     []*schema.Document
	loadedContent string
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

	router := parser.NewParserRouter(parser.NewTextParser(64*1024*1024), parser.NewMarkdownParser(64*1024*1024), newPythonParser(cfg))
	artifactStore := parser.NewArtifactStore(cfg.Store, cfg.ValidateLimits)
	docNormalizer := normalizer.NewDocumentNormalizer()
	enricher := cfg.AssetEnricher
	if enricher == nil {
		enricher = asset.NewNoopEnricher()
	}
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
	chunkCleaner := transformer.NewChunkCleaner()
	ftsTokenizer := tokenizer.NewNgramTokenizer(tokenizer.DefaultNgramConfig())

	chunkConfigHash := computeChunkConfigHash(cfg)

	g := compose.NewGraph[ProcessInput, ProcessOutput]()

	// load_source：URL 来源在 Worker 内通过安全 Eino Loader 抓取；文件来源保持 MinIO 流；
	// 手工文档（manual）直接使用正文 Content，不访问 MinIO。
	loadSourceLambda := compose.InvokableLambda(func(ctx context.Context, input ProcessInput) (*pipelineState, error) {
		state := &pipelineState{input: input}
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
		ref, err := artifactStore.Resolve(ctx, state.artifactPrefix, state.input.DocMeta.SourceHash)
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
		doc, err := router.Parse(ctx, parser.ParseInput{
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
		return state, nil
	})
	if err := g.AddLambdaNode("persist_artifact", persistArtifactLambda); err != nil {
		return nil, fmt.Errorf("注册 persist_artifact 节点失败: %w", err)
	}

	// document_normalize：分块前规范化。
	normalizeLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
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

	// structure_chunk：结构感知分块。
	chunkLambda := compose.InvokableLambda(func(ctx context.Context, state *pipelineState) (*pipelineState, error) {
		chunks, err := chunker.Chunk(ctx, state.doc, cfg.ChunkOptions)
		if err != nil {
			return nil, fmt.Errorf("结构分块失败: %w", err)
		}
		// 纯图片文档（图片无 OCR/caption 文字）没有可索引文本，允许 0 Chunk 成功
		// 导入（资产与原文件保留）；有正文却分不出 Chunk 才是分块器 bug。
		if len(chunks) == 0 && !assetOnlyDocument(state.doc) {
			return nil, fmt.Errorf("文档未产生任何 Chunk")
		}
		state.chunks = chunks
		return state, nil
	})
	if err := g.AddLambdaNode("structure_chunk", chunkLambda); err != nil {
		return nil, fmt.Errorf("注册 structure_chunk 节点失败: %w", err)
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
			return state, nil
		}
		entities := make([]*entity.DocumentChunk, 0, len(state.chunks))
		for i, chunk := range state.chunks {
			entityChunk, err := chunkToEntity(chunk, i, state.input.DocMeta, chunkConfigHash, ftsTokenizer)
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
		"document_normalize", "asset_enrich", "structure_chunk", "chunk_clean",
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
				return processOutput(state, len(state.indexDocs)), nil
			}
			for _, doc := range state.indexDocs {
				einoadapter.SetMetaString(doc, einoadapter.MetaEmbeddingModelID, modelID)
			}
			ids, err := vectorIndexer.Store(ctx, state.indexDocs, indexer.WithEmbedding(embedder))
			if err != nil {
				return ProcessOutput{}, fmt.Errorf("向量索引失败: %w", err)
			}
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
	}
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
func chunkToEntity(chunk chunking.ParsedChunk, chunkNo int, meta transformer.DocMeta, chunkConfigHash string, fts *tokenizer.NgramTokenizer) (*entity.DocumentChunk, error) {
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
	ftsTokens := strings.Join(fts.Tokenize(chunk.Content), " ")
	if ftsTokens == "" {
		return nil, fmt.Errorf("Chunk %d 未产生 fts_tokens", chunkNo)
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
		FTSTokens:       ftsTokens,
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
