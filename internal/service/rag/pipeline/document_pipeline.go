// Package pipeline 定义文档加工与检索的 Eino 编排。
// 文档加工 Graph 在应用初始化时 Compile 一次并注入 Worker Handler。
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/indexing"
	"github.com/1090-f/Memora/internal/service/rag/loader"
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
	// DocMeta 是稳定的业务元数据。
	DocMeta transformer.DocMeta
}

// ProcessOutput 是文档加工 Graph 的输出。
type ProcessOutput struct {
	// ChunkCount 是本次加工成功入库的 Chunk 数量。
	ChunkCount int
}

// DocumentPipeline 是已编译的文档加工编排。
type DocumentPipeline struct {
	runnable         compose.Runnable[ProcessInput, ProcessOutput]
	chunkConfigHash  string
	embeddingModelID string
	// indexOption 在 Run 时注入 Indexer 节点的 Embedding（Eino 通过 WithIndexerOption 透传）。
	indexOption compose.Option
	// hasVector 标记是否启用向量索引节点。
	hasVector bool
}

// DocumentPipelineConfig 定义文档加工流水线配置。
type DocumentPipelineConfig struct {
	// Store 是 MinIO 对象读取器。
	Store loader.MinIOObjectReader
	// Chunks 是 Chunk 持久化接口。
	Chunks ChunkWriter
	// ChunkConfig 是分段配置的稳定描述（计算 chunk_config_hash）。
	ChunkConfig string
	// Vectors 是向量持久化接口（可选：维度冻结并接入 Embedder 后启用向量索引）。
	Vectors repository.VectorRepository
	// EmbeddingModelID 是向量索引使用的模型配置 ID（Vectors 非空时必填）。
	EmbeddingModelID string
	// Embedder 是 Eino Embedding 组件（可选：由 ModelFactory 适配器提供）。
	Embedder embedding.Embedder
}

// NewDocumentPipeline 构造并编译文档加工 Graph。
// 编译失败返回错误，由调用方（internal/app）中止初始化。
//
// 图结构（类型流）：
//
//	Input(ProcessInput)
//	→ load（Lambda：MinIO Loader 读取 + 注入业务元数据 → []*schema.Document）
//	→ clean（Cleaner Transformer）
//	→ split（Markdown Header Splitter → Recursive Splitter）
//	→ enrich（ChunkEnricher Transformer：编号/计数/heading_path/chunk_config_hash）
//	→ persist（Lambda：ChunkWriter 批量落库 → ProcessOutput）
//	→ Output(ProcessOutput)
func NewDocumentPipeline(cfg DocumentPipelineConfig) (*DocumentPipeline, error) {
	if cfg.Store == nil || cfg.Chunks == nil {
		return nil, fmt.Errorf("文档加工流水线缺少 Store 或 Chunks 依赖")
	}

	minioLoader, err := loader.NewMinIOLoader(context.Background(), cfg.Store, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 MinIO Loader 失败: %w", err)
	}
	cleaner := transformer.NewCleaner()
	enricher := transformer.NewChunkEnricher(transformer.EnrichConfig{ChunkConfig: cfg.ChunkConfig})
	headingEnricher, err := transformer.NewHeadingEnricher()
	if err != nil {
		return nil, fmt.Errorf("构造分段器失败: %w", err)
	}
	tokenizerNode := transformer.NewChineseTokenizerTransformer(nil)

	hash := enricher.ChunkConfigHash()

	g := compose.NewGraph[ProcessInput, ProcessOutput]()

	// load：ProcessInput → []*schema.Document
	loadLambda := compose.InvokableLambda(func(ctx context.Context, input ProcessInput) ([]*schema.Document, error) {
		docs, loadErr := minioLoader.Load(ctx, document.Source{URI: input.ObjectKey})
		if loadErr != nil {
			return nil, loadErr
		}
		return injectMeta(docs, input.DocMeta), nil
	})
	if err := g.AddLambdaNode("load", loadLambda); err != nil {
		return nil, fmt.Errorf("注册 load 节点失败: %w", err)
	}

	// clean：Transformer 节点
	if err := g.AddDocumentTransformerNode("clean", cleaner); err != nil {
		return nil, fmt.Errorf("注册 clean 节点失败: %w", err)
	}

	// split：Markdown Header + Recursive 组合
	if err := g.AddDocumentTransformerNode("split", headingEnricher); err != nil {
		return nil, fmt.Errorf("注册 split 节点失败: %w", err)
	}

	// enrich：Transformer 节点
	if err := g.AddDocumentTransformerNode("enrich", enricher); err != nil {
		return nil, fmt.Errorf("注册 enrich 节点失败: %w", err)
	}

	// tokenize：中文分词 → fts_tokens
	if err := g.AddDocumentTransformerNode("tokenize", tokenizerNode); err != nil {
		return nil, fmt.Errorf("注册 tokenize 节点失败: %w", err)
	}

	// persist：[]*schema.Document → []*schema.Document（写回 chunk_id）
	persistLambda := compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
		chunks := make([]*entity.DocumentChunk, 0, len(docs))
		for _, doc := range docs {
			chunk, convErr := chunkFromDocument(doc)
			if convErr != nil {
				return nil, convErr
			}
			chunks = append(chunks, chunk)
		}
		ids, persistErr := cfg.Chunks.BatchInsert(ctx, chunks)
		if persistErr != nil {
			return nil, persistErr
		}
		inserted := 0
		for i := range chunks {
			if ids[i] == "" {
				continue
			}
			inserted++
			// 按位置关联：chunks[i] 对应 docs[i]，写回 DB 生成的 chunk_id。
			if i < len(docs) {
				if docs[i].MetaData == nil {
					docs[i].MetaData = make(map[string]any)
				}
				docs[i].MetaData[einoadapter.MetaChunkID] = ids[i]
			}
		}
		if inserted == 0 {
			return nil, fmt.Errorf("未插入任何 Chunk")
		}
		return docs, nil
	})
	if err := g.AddLambdaNode("persist", persistLambda); err != nil {
		return nil, fmt.Errorf("注册 persist 节点失败: %w", err)
	}

	// index：PostgresIndexer（可选，配置了 Vectors+Embedder 时启用）
	hasVector := cfg.Vectors != nil && cfg.Embedder != nil
	var indexOption compose.Option
	if hasVector {
		vectorIndexer, idxErr := indexing.NewPostgresIndexer(indexing.PostgresIndexerConfig{
			EmbeddingModelID: cfg.EmbeddingModelID,
			Repository:       cfg.Vectors,
		})
		if idxErr != nil {
			return nil, fmt.Errorf("构造向量索引器失败: %w", idxErr)
		}
		if err := g.AddIndexerNode("index", vectorIndexer); err != nil {
			return nil, fmt.Errorf("注册 index 节点失败: %w", err)
		}
		// 通过 Graph 调用选项在 Run 时注入 Embedder。
		indexOption = compose.WithIndexerOption(indexer.WithEmbedding(cfg.Embedder))
	}

	if err := g.AddEdge(compose.START, "load"); err != nil {
		return nil, fmt.Errorf("连接 start→load 失败: %w", err)
	}
	for _, edge := range [][2]string{{"load", "clean"}, {"clean", "split"}, {"split", "enrich"}, {"enrich", "tokenize"}, {"tokenize", "persist"}} {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("连接 %s→%s 失败: %w", edge[0], edge[1], err)
		}
	}
	// finalize（无向量模式）：[]*schema.Document → ProcessOutput
	finalizeDocs := compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) (ProcessOutput, error) {
		return ProcessOutput{ChunkCount: len(docs)}, nil
	})
	if err := g.AddLambdaNode("finalize", finalizeDocs); err != nil {
		return nil, fmt.Errorf("注册 finalize 节点失败: %w", err)
	}

	if hasVector {
		// finalize（向量模式）：[]string → ProcessOutput
		finalizeIDs := compose.InvokableLambda(func(ctx context.Context, ids []string) (ProcessOutput, error) {
			return ProcessOutput{ChunkCount: len(ids)}, nil
		})
		if err := g.AddLambdaNode("finalize_ids", finalizeIDs); err != nil {
			return nil, fmt.Errorf("注册 finalize_ids 节点失败: %w", err)
		}
		if err := g.AddEdge("persist", "index"); err != nil {
			return nil, fmt.Errorf("连接 persist→index 失败: %w", err)
		}
		if err := g.AddEdge("index", "finalize_ids"); err != nil {
			return nil, fmt.Errorf("连接 index→finalize_ids 失败: %w", err)
		}
		if err := g.AddEdge("finalize_ids", compose.END); err != nil {
			return nil, fmt.Errorf("连接 finalize_ids→end 失败: %w", err)
		}
	} else {
		if err := g.AddEdge("persist", "finalize"); err != nil {
			return nil, fmt.Errorf("连接 persist→finalize 失败: %w", err)
		}
		if err := g.AddEdge("finalize", compose.END); err != nil {
			return nil, fmt.Errorf("连接 finalize→end 失败: %w", err)
		}
	}

	runnable, err := g.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译文档加工 Graph 失败: %w", err)
	}
	return &DocumentPipeline{runnable: runnable, chunkConfigHash: hash, embeddingModelID: cfg.EmbeddingModelID, indexOption: indexOption, hasVector: hasVector}, nil
}

// ChunkConfigHash 返回当前分段配置的稳定哈希。
func (p *DocumentPipeline) ChunkConfigHash() string { return p.chunkConfigHash }

// EmbeddingModelID 返回向量索引使用的模型配置 ID；未启用时返回空。
func (p *DocumentPipeline) EmbeddingModelID() string { return p.embeddingModelID }

// Run 执行一次文档加工。
func (p *DocumentPipeline) Run(ctx context.Context, input ProcessInput) (ProcessOutput, error) {
	// 向量模式下通过调用期选项注入 Embedder，使编译后的 Graph 可跨请求复用。
	if p.hasVector {
		return p.runnable.Invoke(ctx, input, p.indexOption)
	}
	return p.runnable.Invoke(ctx, input)
}

// injectMeta 为 Loader 输出的每个文档注入稳定业务元数据。
func injectMeta(docs []*schema.Document, meta transformer.DocMeta) []*schema.Document {
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		doc.MetaData[einoadapter.MetaUserID] = meta.UserID
		doc.MetaData[einoadapter.MetaKnowledgeBase] = meta.KnowledgeBaseID
		doc.MetaData[einoadapter.MetaDocumentID] = meta.DocumentID
		doc.MetaData[einoadapter.MetaIndexVersion] = meta.IndexVersion
		doc.MetaData[einoadapter.MetaContentVersion] = meta.ContentVersion
		doc.MetaData[einoadapter.MetaChunkVersion] = meta.ChunkVersion
		// 仅在非空时写入标题与位置，避免空值覆盖 Loader 已注入的元数据。
		if meta.DocumentTitle != "" {
			doc.MetaData[einoadapter.MetaDocumentTitle] = meta.DocumentTitle
		}
		if meta.SourceLocation != nil {
			doc.MetaData[einoadapter.MetaSourceLocation] = meta.SourceLocation
		}
	}
	return docs
}

// chunkFromDocument 将 Eino schema.Document 转换为 document_chunks 实体。
// 元数据缺失/类型错误时返回错误，避免损坏数据。
func chunkFromDocument(doc *schema.Document) (*entity.DocumentChunk, error) {
	userID := getMetaString(doc, einoadapter.MetaUserID)
	kbID := getMetaString(doc, einoadapter.MetaKnowledgeBase)
	documentID := getMetaString(doc, einoadapter.MetaDocumentID)
	if userID == "" || kbID == "" || documentID == "" {
		return nil, fmt.Errorf("chunk 缺少 user_id/knowledge_base_id/document_id 元数据")
	}
	indexVersion := getMetaInt(doc, einoadapter.MetaIndexVersion)
	contentVersion := getMetaInt(doc, einoadapter.MetaContentVersion)
	chunkVersion := getMetaInt(doc, einoadapter.MetaChunkVersion)
	chunkNo := getMetaInt(doc, einoadapter.MetaChunkNo)
	chunkConfigHash := getMetaString(doc, einoadapter.MetaChunkConfigHash)
	if indexVersion <= 0 || chunkNo < 0 {
		return nil, fmt.Errorf("chunk 缺少有效 index_version/chunk_no 元数据")
	}

	headingPath, _ := json.Marshal(getMetaSlice(doc, einoadapter.MetaHeadingPath))
	sourceLocation, _ := json.Marshal(doc.MetaData[einoadapter.MetaSourceLocation])

	contextTitle := getMetaString(doc, einoadapter.MetaContextTitle)
	ftsTokens := getMetaString(doc, einoadapter.MetaFTSTokens)
	if ftsTokens == "" {
		return nil, fmt.Errorf("chunk 缺少 fts_tokens 元数据")
	}
	return &entity.DocumentChunk{
		UserID:          userID,
		KnowledgeBaseID: kbID,
		DocumentID:      documentID,
		ChunkNo:         chunkNo,
		Content:         doc.Content,
		CharCount:       getMetaInt(doc, einoadapter.MetaCharCount),
		TokenCount:      getMetaInt(doc, einoadapter.MetaTokenCount),
		ContextTitle:    stringPtrOrNil(contextTitle),
		HeadingPath:     headingPath,
		SourceLocation:  sourceLocation,
		ContentVersion:  contentVersion,
		ChunkVersion:    chunkVersion,
		IndexVersion:    indexVersion,
		ChunkConfigHash: chunkConfigHash,
		FTSTokens:       ftsTokens,
	}, nil
}

// getMetaString 安全读取字符串元数据，缺失或类型不符时返回空串。
func getMetaString(doc *schema.Document, key string) string {
	if doc.MetaData == nil {
		return ""
	}
	value, _ := doc.MetaData[key].(string)
	return value
}

// getMetaInt 安全读取 int 元数据；兼容 float64（JSON 解码产物），缺失返回 0。
func getMetaInt(doc *schema.Document, key string) int {
	if doc.MetaData == nil {
		return 0
	}
	switch typed := doc.MetaData[key].(type) {
	case int:
		return typed
	case float64:
		// 经 JSON 反序列化后的数字会变成 float64，需显式转换。
		return int(typed)
	}
	return 0
}

// getMetaSlice 读取字符串数组元数据，兼容 []string 与 JSON 解码产生的 []any。
func getMetaSlice(doc *schema.Document, key string) []string {
	if doc.MetaData == nil {
		return nil
	}
	switch typed := doc.MetaData[key].(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// stringPtrOrNil 将空串转换为 nil 指针，便于数据库空值语义表达。
func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
