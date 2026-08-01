package contracts

import "context"

type DocumentReadRequest struct {
	UserID          ID     `json:"user_id"`
	KnowledgeBaseID ID     `json:"knowledge_base_id"`
	DocumentID      ID     `json:"document_id"`
	Section         string `json:"section,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	MaxTokens       int    `json:"max_tokens"`
}

type DocumentReadResult struct {
	DocumentID ID       `json:"document_id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	NextCursor string   `json:"next_cursor,omitempty"`
	Truncated  bool     `json:"truncated"`
	Citation   Citation `json:"citation"`
}

type DocumentService interface {
	Read(ctx context.Context, request DocumentReadRequest) (DocumentReadResult, error)
}
