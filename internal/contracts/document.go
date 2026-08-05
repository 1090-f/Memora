package contracts

import "context"

// DocumentReadRequest 表示读取文档内容的请求。
type DocumentReadRequest struct {
	UserID          ID     `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id"` // 知识库标识
	DocumentID      ID     `json:"document_id"`       // 文档 ID
	Section         string `json:"section,omitempty"` // 可选：目标章节
	Cursor          string `json:"cursor,omitempty"`  // 可选：游标，用于继续读取后续内容
	MaxTokens       int    `json:"max_tokens"`        // 本次允许读取的最大 token
}

// DocumentReadResult 包含从文档读取的内容及引用信息。
type DocumentReadResult struct {
	DocumentID ID       `json:"document_id"`           // 文档 ID
	Title      string   `json:"title"`                 // 文档标题
	Content    string   `json:"content"`               // 读取到的内容
	NextCursor string   `json:"next_cursor,omitempty"` // 下次继续读取的游标
	Truncated  bool     `json:"truncated"`             // 是否因 token 限制被截断
	Citation   Citation `json:"citation"`              // 对应文档的引用信息
}

// DocumentService 提供文档读取能力。
type DocumentService interface {
	// Read 根据请求参数从文档中检索内容。
	Read(ctx context.Context, request DocumentReadRequest) (DocumentReadResult, error)
}
