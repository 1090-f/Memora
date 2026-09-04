package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// HashDocument computes a stable hash over canonical text, nodes and source map.
// Profile is deliberately excluded because it is derived from the canonical body.
func HashDocument(doc *CanonicalDocument) (string, error) {
	payload := struct {
		SchemaVersion   string          `json:"schema_version"`
		RendererVersion string          `json:"renderer_version"`
		Markdown        string          `json:"markdown"`
		Nodes           []CanonicalNode `json:"nodes"`
		SourceMap       []SourceSpan    `json:"source_map"`
	}{
		SchemaVersion: doc.SchemaVersion, RendererVersion: doc.RendererVersion,
		Markdown: doc.Markdown, Nodes: doc.Nodes, SourceMap: doc.SourceMap,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ConfigHash computes a stable renderer/config identity used by downstream hashes.
func ConfigHash(info RendererInfo, opts RenderOptions) (string, error) {
	data, err := json.Marshal(struct {
		SchemaVersion string        `json:"schema_version"`
		Renderer      RendererInfo  `json:"renderer"`
		Options       RenderOptions `json:"options"`
	}{SchemaVersion: SchemaVersion, Renderer: info, Options: opts})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ArtifactConfigHash 计算 Canonical Artifact 的确定性身份。
// 将规范化、资产增强后的 ParsedDocument 一并纳入哈希，避免增强结果变化时误用旧缓存。
func ArtifactConfigHash(doc *parser.ParsedDocument, info RendererInfo, config string) (string, error) {
	data, err := json.Marshal(struct {
		ArtifactSchemaVersion string                 `json:"artifact_schema_version"`
		CanonicalSchema       string                 `json:"canonical_schema"`
		Renderer              RendererInfo           `json:"renderer"`
		Config                string                 `json:"config"`
		Document              *parser.ParsedDocument `json:"document"`
	}{
		ArtifactSchemaVersion: CanonicalArtifactSchemaVersion,
		CanonicalSchema:       SchemaVersion,
		Renderer:              info,
		Config:                config,
		Document:              doc,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
