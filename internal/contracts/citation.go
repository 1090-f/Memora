package contracts

import "time"

type CitationSource string

const (
	CitationKnowledge CitationSource = "knowledge_base"
	CitationNetwork   CitationSource = "network"
)

type Citation struct {
	SourceType        CitationSource `json:"source_type"`
	DocumentID        ID             `json:"document_id,omitempty"`
	DocumentTitle     string         `json:"document_title,omitempty"`
	ChunkID           ID             `json:"chunk_id,omitempty"`
	QuotedText        string         `json:"quoted_text,omitempty"`
	SourceLocation    map[string]any `json:"source_location,omitempty"`
	DocumentUpdatedAt *time.Time     `json:"document_updated_at,omitempty"`
	Title             string         `json:"title,omitempty"`
	URL               string         `json:"url,omitempty"`
	SiteName          string         `json:"site_name,omitempty"`
	PublishedAt       *time.Time     `json:"published_at,omitempty"`
	FetchedAt         *time.Time     `json:"fetched_at,omitempty"`
}
