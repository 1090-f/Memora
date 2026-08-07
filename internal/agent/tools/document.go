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

const DocumentReadToolName = "document_read"

type documentArguments struct {
	DocumentID string `json:"document_id"`
	Section    string `json:"section,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	MaxTokens  int    `json:"max_tokens,omitempty"`
}

// DocumentReadTool 将文档服务适配为受当前用户和知识库约束的 Eino 只读工具。
type DocumentReadTool struct{ service contracts.DocumentService }

// NewDocumentReadTool 创建文档读取工具，所有权校验交给 DocumentService。
func NewDocumentReadTool(service contracts.DocumentService) *DocumentReadTool {
	return &DocumentReadTool{service: service}
}

// Spec 返回注册表和 Executor 使用的静态安全规格。
func (t *DocumentReadTool) Spec() contracts.ToolSpec {
	return contracts.ToolSpec{Name: DocumentReadToolName, Description: "读取当前知识库内文档的受限内容", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true, MaxCalls: 10}
}

// Info 返回文档读取的固定 JSON Schema，避免模型传入用户或知识库身份。
func (t *DocumentReadTool) Info(context.Context) (*schema.ToolInfo, error) {
	return info(DocumentReadToolName, "读取已知文档的指定章节或后续内容；需要完整上下文时使用。", schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"document_id": {Type: schema.String, Desc: "目标文档 ID", Required: true},
		"section":     {Type: schema.String, Desc: "可选章节"},
		"cursor":      {Type: schema.String, Desc: "继续读取游标"},
		"max_tokens":  {Type: schema.Integer, Desc: "最大读取 token 数"},
	})), nil
}

// InvokableRun 从 ToolContext 获取服务端身份，完成参数校验后读取文档。
func (t *DocumentReadTool) InvokableRun(ctx context.Context, input string, _ ...tool.Option) (string, error) {
	tc, ok := toolContextFrom(ctx)
	if !ok {
		return "", fmt.Errorf("tool context is missing")
	}
	return t.execute(ctx, tc, input)
}

// Execute 执行工具调用并保留下游服务错误，不把它们误报为参数错误。
func (t *DocumentReadTool) Execute(ctx context.Context, tc contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	text, err := t.execute(ctx, tc, string(call.Arguments))
	if err != nil {
		code := contracts.ErrInternal
		if _, ok := err.(*argumentError); ok {
			code = contracts.ErrInvalidArgument
		}
		return contracts.ToolResult{CallID: call.CallID, ToolName: DocumentReadToolName, Success: false, ErrorCode: code, ErrorMessage: err.Error()}, err
	}
	result, err := decodeResult(text)
	if err != nil {
		return contracts.ToolResult{CallID: call.CallID, ToolName: DocumentReadToolName, Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: err.Error()}, err
	}
	result.CallID = call.CallID
	result.ToolName = DocumentReadToolName
	return result, nil
}

func (t *DocumentReadTool) execute(ctx context.Context, tc contracts.ToolContext, input string) (string, error) {
	var args documentArguments
	if err := parseArguments(input, &args); err != nil {
		return "", err
	}
	args.DocumentID = strings.TrimSpace(args.DocumentID)
	if args.DocumentID == "" {
		return "", invalidArgument("document_id is required")
	}
	if len(args.DocumentID) > 128 {
		return "", invalidArgument("document_id is too long")
	}
	if args.MaxTokens <= 0 {
		args.MaxTokens = 6000
	}
	if args.MaxTokens > 6000 {
		args.MaxTokens = 6000
	}
	result, err := t.service.Read(ctx, contracts.DocumentReadRequest{UserID: tc.UserID, KnowledgeBaseID: tc.KnowledgeBaseID, DocumentID: contracts.ID(args.DocumentID), Section: args.Section, Cursor: args.Cursor, MaxTokens: args.MaxTokens})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(contracts.ToolResult{ToolName: DocumentReadToolName, Text: result.Content, Citations: []contracts.Citation{result.Citation}, Truncated: result.Truncated, Success: true})
	if err != nil {
		return "", fmt.Errorf("marshal document result: %w", err)
	}
	return string(data), nil
}
