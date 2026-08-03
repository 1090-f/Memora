package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

const DocumentReadToolName = "document_read"

type DocumentReadTool struct {
	service contracts.DocumentService
	spec    contracts.ToolSpec
}

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

func (t *DocumentReadTool) Spec() contracts.ToolSpec {
	return t.spec
}

type documentReadArgs struct {
	DocumentID contracts.ID `json:"document_id"`
	Section    string       `json:"section,omitempty"`
	Cursor     string       `json:"cursor,omitempty"`
	MaxTokens  int          `json:"max_tokens"`
}

type documentReadOutput struct {
	DocumentID contracts.ID       `json:"document_id"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Truncated  bool               `json:"truncated"`
	Citation   contracts.Citation `json:"citation"`
}

func (t *DocumentReadTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	if t.service == nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInvalidState, ErrorMessage: "document service is not configured"}, nil
	}

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

	req := contracts.DocumentReadRequest{
		UserID:          toolContext.UserID,
		KnowledgeBaseID: toolContext.KnowledgeBaseID,
		DocumentID:      args.DocumentID,
		Section:         args.Section,
		Cursor:          args.Cursor,
		MaxTokens:       args.MaxTokens,
	}

	res, err := t.service.Read(ctx, req)
	if err != nil {
		return contracts.ToolResult{Success: false, ErrorCode: contracts.ErrInternal, ErrorMessage: "document read failed"}, nil
	}

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

	return contracts.ToolResult{
		Text:           truncateUTF8ByBytes(res.Content, 4000),
		StructuredData: structured,
		Citations:      []contracts.Citation{res.Citation},
		Truncated:      res.Truncated,
		Success:        true,
	}, nil
}
