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

const KnowledgeSearchToolName = "knowledge_search"

type searchArguments struct {
	Query string `json:"query"`
	Mode  string `json:"mode,omitempty"`
	TopK  int    `json:"top_k,omitempty"`
}

// KnowledgeSearchTool 将检索服务适配为 Eino 只读工具，身份由 ToolContext 注入。
type KnowledgeSearchTool struct{ service contracts.RetrievalService }

// NewKnowledgeSearchTool 创建知识库检索工具；检索算法和所有权过滤由下游 Service 负责。
func NewKnowledgeSearchTool(service contracts.RetrievalService) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{service: service}
}

// Spec 返回注册表和 Executor 使用的静态安全规格。
func (t *KnowledgeSearchTool) Spec() contracts.ToolSpec {
	return contracts.ToolSpec{Name: KnowledgeSearchToolName, Description: "在当前知识库中检索相关文档片段", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true, MaxCalls: 10}
}

// Info 返回给模型的工具名称、用途和固定 JSON 参数模式。
func (t *KnowledgeSearchTool) Info(context.Context) (*schema.ToolInfo, error) {
	return info(KnowledgeSearchToolName, "检索当前知识库中的相关资料；需要知识依据时使用。", schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"query": {Type: schema.String, Desc: "要检索的问题或关键词", Required: true},
		"mode":  {Type: schema.String, Desc: "keyword、vector 或 hybrid", Enum: []string{"keyword", "vector", "hybrid"}},
		"top_k": {Type: schema.Integer, Desc: "返回条数，最大 20"},
	})), nil
}

// InvokableRun 从上下文读取服务端注入的 ToolContext，校验 JSON 后调用检索服务。
func (t *KnowledgeSearchTool) InvokableRun(ctx context.Context, input string, _ ...tool.Option) (string, error) {
	tc, ok := toolContextFrom(ctx)
	if !ok {
		return "", fmt.Errorf("tool context is missing")
	}
	return t.execute(ctx, tc, input)
}

// Execute 执行工具调用并将参数错误与下游服务错误分开编码。
func (t *KnowledgeSearchTool) Execute(ctx context.Context, tc contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	text, err := t.execute(ctx, tc, string(call.Arguments))
	if err != nil {
		code := contracts.ErrInternal
		if _, ok := err.(*argumentError); ok {
			code = contracts.ErrInvalidArgument
		}
		return contracts.ToolResult{CallID: call.CallID, ToolName: KnowledgeSearchToolName, Success: false, ErrorCode: code, ErrorMessage: err.Error()}, err
	}
	result, err := decodeResult(text)
	if err != nil {
		return contracts.ToolResult{CallID: call.CallID, ToolName: KnowledgeSearchToolName, Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: err.Error()}, err
	}
	result.CallID = call.CallID
	result.ToolName = KnowledgeSearchToolName
	return result, nil
}

func (t *KnowledgeSearchTool) execute(ctx context.Context, tc contracts.ToolContext, input string) (string, error) {
	var args searchArguments
	if err := parseArguments(input, &args); err != nil {
		return "", err
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return "", invalidArgument("query is required")
	}
	if len(args.Query) > 4096 {
		return "", invalidArgument("query is too long")
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
		return "", invalidArgument("invalid retrieval mode")
	}
	result, err := t.service.Retrieve(ctx, contracts.RetrievalRequest{UserID: tc.UserID, KnowledgeBaseID: tc.KnowledgeBaseID, Query: args.Query, Mode: mode, TopK: args.TopK, Config: contracts.DefaultSearchConfig()})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(contracts.ToolResult{ToolName: KnowledgeSearchToolName, Text: marshalRetrievalText(result), Citations: retrievalCitations(result), Success: true})
	if err != nil {
		return "", fmt.Errorf("marshal retrieval result: %w", err)
	}
	return string(data), nil
}

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
