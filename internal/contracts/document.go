package contracts

import "context"

// DocumentReadRequest 表示读取文档内容的请求。
type DocumentReadRequest struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id"`
	DocumentID      ID     `json:"document_id"`
	Section         string `json:"section,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	MaxTokens       int    `json:"max_tokens"`
}

// DocumentReadResult 包含从文档读取的内容及引用信息。
type DocumentReadResult struct {
	DocumentID ID       `json:"document_id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	NextCursor string   `json:"next_cursor,omitempty"`
	Truncated  bool     `json:"truncated"`
	Citation   Citation `json:"citation"`
}

// DocumentService 提供文档读取能力。
type DocumentService interface {
	// Read 根据请求参数从文档中检索内容。
	Read(ctx context.Context, request DocumentReadRequest) (DocumentReadResult, error)
}
