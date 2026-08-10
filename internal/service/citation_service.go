package service

import (
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// CitationService 统一构建知识库引用，避免检索与正文读取产生不同格式。
type CitationService interface {
	BuildKnowledge(knowledgeBaseID, documentID, documentTitle, chunkID, quotedText string, sourceLocation map[string]any, documentUpdatedAt time.Time) contracts.Citation
}

type citationService struct{}

// NewCitationService 创建无状态引用服务。
func NewCitationService() CitationService { return &citationService{} }

func (s *citationService) BuildKnowledge(knowledgeBaseID, documentID, documentTitle, chunkID, quotedText string, sourceLocation map[string]any, documentUpdatedAt time.Time) contracts.Citation {
	quoted := strings.TrimSpace(quotedText)
	if len([]rune(quoted)) > 500 {
		quoted = string([]rune(quoted)[:500])
	}
	updatedAt := documentUpdatedAt.UTC()
	citation := contracts.Citation{
		SourceType: contracts.CitationKnowledge, KnowledgeBaseID: contracts.ID(knowledgeBaseID),
		DocumentID: contracts.ID(documentID), DocumentTitle: documentTitle,
		ChunkID: contracts.ID(chunkID), QuotedText: quoted,
		SourceLocation: cloneMetadata(sourceLocation),
	}
	if !updatedAt.IsZero() {
		citation.DocumentUpdatedAt = &updatedAt
	}
	return citation
}

func cloneMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
