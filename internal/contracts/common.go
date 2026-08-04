package contracts

import "time"

// Version 是当前 API 版本常量。
const Version = "v0.1"

// ID 是用于实体标识符的字符串类型。
type ID string

// PageRequest 表示分页查询请求。
type PageRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword,omitempty"`
	Sort     string `json:"sort,omitempty"`
}

// Page 表示包含数据项和分页元数据的分页响应。
type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// TokenUsage 跟踪 AI 模型调用中消耗的 Token 数量。
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// TimeRange 表示一个可选起止时间的时间区间。
type TimeRange struct {
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}
