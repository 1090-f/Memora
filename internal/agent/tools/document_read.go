package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/utils"
)

// DocumentReadToolName 是文档读取工具在注册表中的名称。
const DocumentReadToolName = "document_read"

// DocumentReadTool 内置工具：按章节或游标读取知识库文档内容，
// 返回有长度上限的内容块，供 Agent 逐段翻阅长文档。
type DocumentReadTool struct {
	service contracts.DocumentService
	spec    contracts.ToolSpec
}

// NewDocumentReadTool 创建文档读取工具。
func NewDocumentReadTool(service contracts.DocumentService) *DocumentReadTool {
	return &DocumentReadTool{
		service: service,
		spec: contracts.ToolSpec{
			Name:        DocumentReadToolName,
			Description: "Read a knowledge base document by section or cursor and return a bounded chunk of content",
			Type:        contracts.ToolTypeBuiltin,
			ReadOnly:    true,
			Enabled:     true,
			Timeout:     15 * time.Second,
		},
	}
}

// Spec 返回该工具的规格描述。
func (t *DocumentReadTool) Spec() contracts.ToolSpec {
	return t.spec
}

// documentReadArgs 是文档读取工具的入参。
type documentReadArgs struct {
	DocumentID contracts.ID `json:"document_id"`       // 目标文档 ID
	Section    string       `json:"section,omitempty"` // 可选：要读取的章节
	Cursor     string       `json:"cursor,omitempty"`  // 可选：上次返回的续读游标
	MaxTokens  int          `json:"max_tokens"`        // 返回内容的最大 token 数
}

// documentReadOutput 是文档读取工具的出参。
type documentReadOutput struct {
	DocumentID contracts.ID       `json:"document_id"`           // 文档 ID
	Title      string             `json:"title"`                 // 文档标题
	Content    string             `json:"content"`               // 读取到的内容
	NextCursor string             `json:"next_cursor,omitempty"` // 续读游标，内容未读完时使用
	Truncated  bool               `json:"truncated"`             // 内容是否被截断
	Citation   contracts.Citation `json:"citation"`              // 本次读取对应的引用信息
}

// Run 执行一次文档读取：校验入参后调用底层文档服务，
// 并把结果封装为文本 + 结构化数据 + 引用的统一工具结果。
func (t *DocumentReadTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	// 底层服务未配置时直接报错。
	if t.service == nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidState, ErrorMessage: "document service is not configured"}, nil
	}

	// 解析入参并做基础校验。
	var args documentReadArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "invalid arguments"}, nil
	}
	if args.DocumentID == "" {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "document_id is required"}, nil
	}
	if args.MaxTokens <= 0 {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidArgument, ErrorMessage: "max_tokens must be positive"}, nil
	}

	// 组装请求，身份信息取自调用上下文。
	req := contracts.DocumentReadRequest{
		UserID:          toolContext.UserID,
		KnowledgeBaseID: toolContext.KnowledgeBaseID,
		DocumentID:      args.DocumentID,
		Section:         args.Section,
		Cursor:          args.Cursor,
		MaxTokens:       args.MaxTokens,
	}

	// 调用底层文档服务。
	res, err := t.service.Read(ctx, req)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "document read failed"}, nil
	}

	// 组装结构化输出。
	out := documentReadOutput{
		DocumentID: res.DocumentID,
		Title:      res.Title,
		Content:    res.Content,
		NextCursor: res.NextCursor,
		Truncated:  res.Truncated,
		Citation:   res.Citation,
	}
	structured, err := json.Marshal(out)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "failed to build result"}, nil
	}

	// 文本部分做字节级截断，防止一次性返回过长内容。
	return contracts.ToolResult{
		Text:           utils.TruncateUTF8ByBytes(res.Content, 4000),
		StructuredData: structured,
		Citations:      []contracts.Citation{res.Citation},
		Truncated:      res.Truncated,
		Success:        true,
	}, nil
}
