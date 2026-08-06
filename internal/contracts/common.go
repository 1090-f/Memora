// Package contracts 定义系统各层之间的共享数据类型与接口契约。
package contracts

import "time"

// Version 是当前 API 版本常量。
const Version = "v0.1"

// ID 是用于实体标识符的字符串类型。
type ID string

// PageRequest 表示分页查询请求。
type PageRequest struct {
	Page     int    `json:"page"`              // 页码，从 1 开始
	PageSize int    `json:"page_size"`         // 每页条数
	Keyword  string `json:"keyword,omitempty"` // 可选：搜索关键字
	Sort     string `json:"sort,omitempty"`    // 可选：排序字段/规则
}

// Page 表示包含数据项和分页元数据的分页响应。
type Page[T any] struct {
	Items    []T   `json:"items"`     // 当前页数据
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页条数
	Total    int64 `json:"total"`     // 总条数
}

// TokenUsage 跟踪 AI 模型调用中消耗的 Token 数量。
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`  // 输入 token 数
	OutputTokens int `json:"output_tokens"` // 输出 token 数
	TotalTokens  int `json:"total_tokens"`  // 总 token 数
}

// TimeRange 表示一个可选起止时间的时间区间。
type TimeRange struct {
	StartedAt *time.Time `json:"started_at,omitempty"` // 区间起始时间
	EndedAt   *time.Time `json:"ended_at,omitempty"`   // 区间结束时间
}
