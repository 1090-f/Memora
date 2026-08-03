// Package contracts 定义系统各层之间的共享数据类型与接口契约。
package contracts

import "time"

// Version 表示系统当前契约版本的标识。
const Version = "v0.1"

// ID 是系统中所有业务实体的通用唯一标识。
type ID string

// PageRequest 是通用分页查询请求参数。
type PageRequest struct {
	Page     int    `json:"page"`              // 页码，从 1 开始
	PageSize int    `json:"page_size"`         // 每页条数
	Keyword  string `json:"keyword,omitempty"` // 可选：搜索关键字
	Sort     string `json:"sort,omitempty"`    // 可选：排序字段/规则
}

// Page 是通用分页返回结构，泛型 T 为列表元素类型。
type Page[T any] struct {
	Items    []T   `json:"items"`     // 当前页数据
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页条数
	Total    int64 `json:"total"`     // 总条数
}

// TokenUsage 记录一次模型调用的 token 消耗情况。
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`  // 输入 token 数
	OutputTokens int `json:"output_tokens"` // 输出 token 数
	TotalTokens  int `json:"total_tokens"`  // 总 token 数
}

// TimeRange 表示一个时间区间。
type TimeRange struct {
	StartedAt *time.Time `json:"started_at,omitempty"` // 区间起始时间
	EndedAt   *time.Time `json:"ended_at,omitempty"`   // 区间结束时间
}
