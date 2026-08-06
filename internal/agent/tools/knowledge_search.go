package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/utils"
)

// KnowledgeSearchToolName 是知识库搜索工具在注册表中的名称。
const KnowledgeSearchToolName = "knowledge_search"

// KnowledgeSearchTool 内置工具：搜索知识库文档，
// 返回按相关度排序的证据片段，供 Agent 回答问题时引用。
type KnowledgeSearchTool struct {
	service contracts.RetrievalService
	spec    contracts.ToolSpec
}

// NewKnowledgeSearchTool 创建知识库搜索工具。
func NewKnowledgeSearchTool(service contracts.RetrievalService) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{
		service: service,
		spec: contracts.ToolSpec{
			Name:        KnowledgeSearchToolName,
			Description: "Search knowledge base documents and return ranked evidence chunks",
			Type:        contracts.ToolTypeBuiltin,
			ReadOnly:    true,
			Enabled:     true,
			Timeout:     15 * time.Second,
		},
	}
}

// Spec 返回该工具的规格描述。
func (t *KnowledgeSearchTool) Spec() contracts.ToolSpec {
	return t.spec
}

// knowledgeSearchArgs 是知识库搜索工具的入参。
type knowledgeSearchArgs struct {
	Query       string                  `json:"query"`                  // 搜索关键词/问题
	Mode        contracts.RetrievalMode `json:"mode"`                   // 检索模式：关键词、向量或混合
	TopK        int                     `json:"top_k"`                  // 返回的最相关片段数量
	DocumentIDs []contracts.ID          `json:"document_ids,omitempty"` // 可选：限定搜索的文档范围
}

// knowledgeSearchItem 表示单条搜索结果（证据片段）。
type knowledgeSearchItem struct {
	DocumentID    contracts.ID       `json:"document_id"`              // 来源文档 ID
	DocumentTitle string             `json:"document_title,omitempty"` // 来源文档标题
	ChunkID       contracts.ID       `json:"chunk_id"`                 // 片段 ID
	Content       string             `json:"content"`                  // 片段内容
	KeywordRank   *int               `json:"keyword_rank,omitempty"`   // 关键词检索下的排名
	VectorRank    *int               `json:"vector_rank,omitempty"`    // 向量检索下的排名
	Score         float64            `json:"score"`                    // 相关度得分
	IndexVersion  int                `json:"index_version"`            // 使用的索引版本
	Citation      contracts.Citation `json:"citation"`                 // 引用信息
}

// knowledgeSearchOutput 是知识库搜索工具的出参。
type knowledgeSearchOutput struct {
	Items           []knowledgeSearchItem `json:"items"`                      // 命中的证据片段列表
	RewrittenQuery  string                `json:"rewritten_query,omitempty"`  // 检索前被改写后的查询（若有）
	KnowledgeStatus string                `json:"knowledge_status,omitempty"` // 知识库状态信息（若有）
}

// Run 执行一次知识库检索：校验入参后调用底层检索服务，
// 收集各片段的引用信息，封装为统一的工具结果。
func (t *KnowledgeSearchTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	// 底层服务未配置时直接报错。
	if t.service == nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidState, ErrorMessage: "retrieval service is not configured"}, nil
	}

	// 解析入参并做基础校验（查询非空、TopK 为正、模式合法）。
	var args knowledgeSearchArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "invalid arguments"}, nil
	}
	if args.Query == "" {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "query is required"}, nil
	}
	if args.TopK <= 0 {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "top_k must be positive"}, nil
	}
	if args.Mode != contracts.RetrievalKeyword && args.Mode != contracts.RetrievalVector && args.Mode != contracts.RetrievalHybrid {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "mode is invalid"}, nil
	}

	// 组装请求，身份信息取自调用上下文，检索配置使用默认值。
	req := contracts.RetrievalRequest{
		UserID:          toolContext.UserID,
		KnowledgeBaseID: toolContext.KnowledgeBaseID,
		Query:           args.Query,
		Mode:            args.Mode,
		DocumentIDs:     args.DocumentIDs,
		TopK:            args.TopK,
		Config:          contracts.DefaultSearchConfig(),
	}
	res, err := t.service.Retrieve(ctx, req)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "retrieval failed"}, nil
	}

	// 组装结构化输出，同时收集所有引用。
	out := knowledgeSearchOutput{RewrittenQuery: res.RewrittenQuery, KnowledgeStatus: res.KnowledgeStatus}
	out.Items = make([]knowledgeSearchItem, 0, len(res.Items))
	citations := make([]contracts.Citation, 0, len(res.Items))
	for _, it := range res.Items {
		item := knowledgeSearchItem{
			DocumentID:    it.DocumentID,
			DocumentTitle: it.Citation.DocumentTitle,
			ChunkID:       it.ChunkID,
			Content:       it.Content,
			KeywordRank:   it.KeywordRank,
			VectorRank:    it.VectorRank,
			Score:         it.Score,
			IndexVersion:  it.IndexVersion,
			Citation:      it.Citation,
		}
		out.Items = append(out.Items, item)
		citations = append(citations, it.Citation)
	}

	// 序列化为结构化数据。
	structured, err := json.Marshal(out)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "failed to build result"}, nil
	}

	// 文本部分使用结构化的 JSON 表示并做字节级截断。
	return contracts.ToolResult{
		Text:           utils.TruncateUTF8ByBytes(string(structured), 4000),
		StructuredData: structured,
		Citations:      citations,
		Success:        true,
	}, nil
}
