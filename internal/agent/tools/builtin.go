package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	KnowledgeSearchToolName = "knowledge_search"
	DocumentReadToolName    = "document_read"
)

type searchArguments struct {
	Query string `json:"query"`
	Mode  string `json:"mode,omitempty"`
	TopK  int    `json:"top_k,omitempty"`
}

type documentArguments struct {
	DocumentID string `json:"document_id"`
	Section    string `json:"section,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	MaxTokens  int    `json:"max_tokens,omitempty"`
}

type KnowledgeSearchTool struct{ service contracts.RetrievalService }

func NewKnowledgeSearchTool(service contracts.RetrievalService) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{service: service}
}

func (t *KnowledgeSearchTool) Spec() contracts.ToolSpec {
	return contracts.ToolSpec{Name: KnowledgeSearchToolName, Description: "在当前知识库中检索相关文档片段", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true, MaxCalls: 10}
}

func (t *KnowledgeSearchTool) Info(context.Context) (*schema.ToolInfo, error) {
	return info(KnowledgeSearchToolName, "检索当前知识库中的相关资料；需要知识依据时使用。", schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"query": {Type: schema.String, Desc: "要检索的问题或关键词", Required: true},
		"mode":  {Type: schema.String, Desc: "keyword、vector 或 hybrid", Enum: []string{"keyword", "vector", "hybrid"}},
		"top_k": {Type: schema.Integer, Desc: "返回条数，最大 20"},
	})), nil
}

func (t *KnowledgeSearchTool) InvokableRun(ctx context.Context, input string, _ ...tool.Option) (string, error) {
	toolContext, ok := toolContextFrom(ctx)
	if !ok {
		return "", fmt.Errorf("tool context is missing")
	}
	return t.execute(ctx, toolContext, input)
}

func (t *KnowledgeSearchTool) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	text, err := t.execute(ctx, toolContext, string(call.Arguments))
	if err != nil {
		return contracts.ToolResult{CallID: call.CallID, ToolName: KnowledgeSearchToolName, Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: err.Error()}, err
	}
	result, err := decodeResult(text)
	result.CallID = call.CallID
	return result, err
}

func (t *KnowledgeSearchTool) execute(ctx context.Context, toolContext contracts.ToolContext, input string) (string, error) {
	var args searchArguments
	if err := parseArguments(input, &args); err != nil {
		return "", err
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.TopK <= 0 {
		args.TopK = 8
	}
	if args.TopK > 20 {
		args.TopK = 20
	}
	mode := contracts.RetrievalHybrid
	if args.Mode != "" {
		mode = contracts.RetrievalMode(args.Mode)
	}
	if mode != contracts.RetrievalKeyword && mode != contracts.RetrievalVector && mode != contracts.RetrievalHybrid {
		return "", fmt.Errorf("invalid retrieval mode")
	}
	result, err := t.service.Retrieve(ctx, contracts.RetrievalRequest{UserID: toolContext.UserID, KnowledgeBaseID: toolContext.KnowledgeBaseID, Query: args.Query, Mode: mode, TopK: args.TopK, Config: contracts.DefaultSearchConfig()})
	if err != nil {
		return "", err
	}
	resultJSON, err := json.Marshal(contracts.ToolResult{ToolName: KnowledgeSearchToolName, Text: marshalRetrievalText(result), Citations: retrievalCitations(result), Success: true})
	if err != nil {
		return "", err
	}
	return string(resultJSON), nil
}

type DocumentReadTool struct{ service contracts.DocumentService }

func NewDocumentReadTool(service contracts.DocumentService) *DocumentReadTool {
	return &DocumentReadTool{service: service}
}

func (t *DocumentReadTool) Spec() contracts.ToolSpec {
	return contracts.ToolSpec{Name: DocumentReadToolName, Description: "读取当前知识库内文档的受限内容", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true, MaxCalls: 10}
}

func (t *DocumentReadTool) Info(context.Context) (*schema.ToolInfo, error) {
	return info(DocumentReadToolName, "读取已知文档的指定章节或后续内容；需要完整上下文时使用。", schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"document_id": {Type: schema.String, Desc: "目标文档 ID", Required: true},
		"section":     {Type: schema.String, Desc: "可选章节"},
		"cursor":      {Type: schema.String, Desc: "继续读取游标"},
		"max_tokens":  {Type: schema.Integer, Desc: "最大读取 token 数"},
	})), nil
}

func (t *DocumentReadTool) InvokableRun(ctx context.Context, input string, _ ...tool.Option) (string, error) {
	toolContext, ok := toolContextFrom(ctx)
	if !ok {
		return "", fmt.Errorf("tool context is missing")
	}
	return t.execute(ctx, toolContext, input)
}

func (t *DocumentReadTool) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	text, err := t.execute(ctx, toolContext, string(call.Arguments))
	if err != nil {
		return contracts.ToolResult{CallID: call.CallID, ToolName: DocumentReadToolName, Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: err.Error()}, err
	}
	result, err := decodeResult(text)
	result.CallID = call.CallID
	return result, err
}

func (t *DocumentReadTool) execute(ctx context.Context, toolContext contracts.ToolContext, input string) (string, error) {
	var args documentArguments
	if err := parseArguments(input, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.DocumentID) == "" {
		return "", fmt.Errorf("document_id is required")
	}
	if args.MaxTokens <= 0 {
		args.MaxTokens = 6000
	}
	if args.MaxTokens > 6000 {
		args.MaxTokens = 6000
	}
	result, err := t.service.Read(ctx, contracts.DocumentReadRequest{UserID: toolContext.UserID, KnowledgeBaseID: toolContext.KnowledgeBaseID, DocumentID: contracts.ID(args.DocumentID), Section: args.Section, Cursor: args.Cursor, MaxTokens: args.MaxTokens})
	if err != nil {
		return "", err
	}
	resultJSON, err := json.Marshal(contracts.ToolResult{ToolName: DocumentReadToolName, Text: result.Content, Citations: []contracts.Citation{result.Citation}, Truncated: result.Truncated, Success: true})
	if err != nil {
		return "", err
	}
	return string(resultJSON), nil
}

func fmtInvalidArgument(err error) error { return fmt.Errorf("invalid tool arguments: %w", err) }
func marshalRetrievalText(result contracts.RetrievalResult) string {
	value, _ := json.Marshal(result)
	return string(value)
}
func retrievalCitations(result contracts.RetrievalResult) []contracts.Citation {
	citations := make([]contracts.Citation, 0, len(result.Items))
	for _, item := range result.Items {
		citations = append(citations, item.Citation)
	}
	return citations
}
