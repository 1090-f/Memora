package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
