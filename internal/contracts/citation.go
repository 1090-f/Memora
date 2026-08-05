package contracts

import "time"

// CitationSource 表示引用来源的类型。
type CitationSource string

// 引用来源常量。
const (
	// CitationKnowledge 表示引用来自知识库文档。
	CitationKnowledge CitationSource = "knowledge_base"
	// CitationNetwork 表示引用来自网络资源。
	CitationNetwork CitationSource = "network"
)

// Citation 表示 Agent 响应中引用的参考来源。
type Citation struct {
	SourceType        CitationSource `json:"source_type"`                   // 来源类型（知识库 / 网络）
	DocumentID        ID             `json:"document_id,omitempty"`         // 文档 ID（知识库来源）
	DocumentTitle     string         `json:"document_title,omitempty"`      // 文档标题
	ChunkID           ID             `json:"chunk_id,omitempty"`            // 片段 ID
	QuotedText        string         `json:"quoted_text,omitempty"`         // 被引用的原文片段
	SourceLocation    map[string]any `json:"source_location,omitempty"`     // 来源定位信息（如页码、行列）
	DocumentUpdatedAt *time.Time     `json:"document_updated_at,omitempty"` // 文档更新时间（知识库来源）
	Title             string         `json:"title,omitempty"`               // 网络信息标题
	URL               string         `json:"url,omitempty"`                 // 网络信息 URL
	SiteName          string         `json:"site_name,omitempty"`           // 网络信息来源站点
	PublishedAt       *time.Time     `json:"published_at,omitempty"`        // 网络信息发布时间
	FetchedAt         *time.Time     `json:"fetched_at,omitempty"`          // 网络信息抓取时间
}
