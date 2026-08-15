// Package einoadapter 是 Eino schema.Document 与 Memora 业务类型之间的单一转换边界。
//
// 约定：
//   - HTTP DTO、数据库 Entity 与跨成员 contracts 不得直接暴露 Eino 类型；
//   - schema.Document 只作为 RAG 内部交换对象；
//   - MetaData 键一律使用本包集中定义的常量，读取必须做类型与缺失校验，禁止不安全类型断言。
package einoadapter

import (
	"time"

	"github.com/cloudwego/eino/schema"
)

// MetaData 键常量。Eino schema.Document.MetaData 使用 open map[string]any，
// 集中定义可保证 Loader/Transformer/Indexer/Retriever 各阶段一致读写。
const (
	MetaUserID               = "user_id"
	MetaKnowledgeBase        = "knowledge_base_id"
	MetaDocumentID           = "document_id"
	MetaDirectoryID          = "directory_id"
	MetaChunkID              = "chunk_id"
	MetaChunkNo              = "chunk_no"
	MetaIndexVersion         = "index_version"
	MetaHeadingPath          = "heading_path"
	MetaSourceLocation       = "source_location"
	MetaKeywordRank          = "keyword_rank"
	MetaKeywordScore         = "keyword_score"
	MetaKeywordMatchLevel    = "keyword_match_level"
	MetaKeywordMatchedTerms  = "keyword_matched_terms"
	MetaKeywordCoverage      = "keyword_coverage"
	MetaKeywordRecallStage   = "keyword_recall_stage"
	MetaKeywordLowConfidence = "keyword_low_confidence"
	MetaVectorRank           = "vector_rank"
	MetaVectorScore          = "vector_score"
	MetaRRFScore             = "rrf_score"
	MetaRRFRank              = "rrf_rank"
	MetaRerankerScore        = "reranker_score"
	MetaDocumentTitle        = "document_title"
	MetaDocumentUpdAt        = "document_updated_at"
	MetaChunkConfigHash      = "chunk_config_hash"
	MetaContentVersion       = "content_version"
	MetaChunkVersion         = "chunk_version"
	MetaQuery                = "query"
	MetaCharCount            = "char_count"
	MetaTokenCount           = "token_count"
	MetaContextTitle         = "context_title"
	MetaEmbeddingModelID     = "embedding_model_id"
)

// GetMetaString 读取 MetaData 中的字符串值并做类型校验；缺失或类型不符时返回零值。
func GetMetaString(meta map[string]any, key string) string {
	value, ok := meta[key].(string)
	if !ok {
		return ""
	}
	return value
}

// GetMetaInt 读取 MetaData 中的 int 值并做类型校验；缺失或类型不符时返回 0。
func GetMetaInt(meta map[string]any, key string) int {
	value, ok := meta[key].(int)
	if !ok {
		return 0
	}
	return value
}

// GetMetaFloat 读取 MetaData 中的 float64 值并做类型校验；缺失或类型不符时返回 0。
func GetMetaFloat(meta map[string]any, key string) float64 {
	switch value := meta[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	default:
		return 0
	}
}

// GetMetaTime 读取 time.Time 或 RFC3339 字符串时间。
func GetMetaTime(meta map[string]any, key string) time.Time {
	switch value := meta[key].(type) {
	case time.Time:
		return value
	case string:
		parsed, _ := time.Parse(time.RFC3339Nano, value)
		return parsed
	default:
		return time.Time{}
	}
}

// GetMetaAny 读取 MetaData 中的任意值，缺失时返回 nil。
func GetMetaAny(meta map[string]any, key string) any {
	return meta[key]
}

// GetMetaStrings reads a string slice without exposing unsafe metadata assertions.
func GetMetaStrings(meta map[string]any, key string) []string {
	value, ok := meta[key].([]string)
	if !ok {
		return nil
	}
	return value
}

// GetMetaBool reads a bool metadata value and returns false when absent.
func GetMetaBool(meta map[string]any, key string) bool {
	value, ok := meta[key].(bool)
	return ok && value
}

// SetMetaString 以安全方式写入 MetaData 字符串值，保证 map 非空。
func SetMetaString(doc *schema.Document, key, value string) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}
	doc.MetaData[key] = value
}

// SetMetaInt 以安全方式写入 MetaData int 值，保证 map 非空。
func SetMetaInt(doc *schema.Document, key string, value int) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}
	doc.MetaData[key] = value
}

// SetMetaFloat 以安全方式写入 MetaData float64 值，保证 map 非空。
func SetMetaFloat(doc *schema.Document, key string, value float64) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}
	doc.MetaData[key] = value
}
