package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

const KnowledgeSearchToolName = "knowledge_search"

type KnowledgeSearchTool struct {
	service contracts.RetrievalService
	spec    contracts.ToolSpec
}

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

func (t *KnowledgeSearchTool) Spec() contracts.ToolSpec {
	return t.spec
}

type knowledgeSearchArgs struct {
	Query       string                  `json:"query"`
	Mode        contracts.RetrievalMode `json:"mode"`
	TopK        int                     `json:"top_k"`
	DocumentIDs []contracts.ID          `json:"document_ids,omitempty"`
}

type knowledgeSearchItem struct {
	DocumentID    contracts.ID       `json:"document_id"`
	DocumentTitle string             `json:"document_title,omitempty"`
	ChunkID       contracts.ID       `json:"chunk_id"`
	Content       string             `json:"content"`
	KeywordRank   *int               `json:"keyword_rank,omitempty"`
	VectorRank    *int               `json:"vector_rank,omitempty"`
	Score         float64            `json:"score"`
	IndexVersion  int                `json:"index_version"`
	Citation      contracts.Citation `json:"citation"`
}

type knowledgeSearchOutput struct {
	Items           []knowledgeSearchItem `json:"items"`
	RewrittenQuery  string                `json:"rewritten_query,omitempty"`
	KnowledgeStatus string                `json:"knowledge_status,omitempty"`
}

func (t *KnowledgeSearchTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	if t.service == nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidState, ErrorMessage: "retrieval service is not configured"}, nil
	}

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

	structured, err := json.Marshal(out)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "failed to build result"}, nil
	}

	return contracts.ToolResult{
		Text:           truncateUTF8ByBytes(string(structured), 4000),
		StructuredData: structured,
		Citations:      citations,
		Success:        true,
	}, nil
}
