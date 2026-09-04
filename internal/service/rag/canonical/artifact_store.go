package canonical

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/klauspost/compress/zstd"
)

const (
	// CanonicalArtifactSchemaVersion 是独立于 CanonicalDocument 的存储协议版本。
	CanonicalArtifactSchemaVersion = "canonical-artifact-v1"
	CanonicalArtifactManifestFile  = "manifest.json"
	CanonicalArtifactDocumentFile  = "canonical-document.json.zst"
	CanonicalArtifactMarkdownFile  = "canonical.md"
	CanonicalArtifactSourceMapFile = "source-map.json.zst"
	canonicalArtifactReadLimit     = 128 * 1024 * 1024
)

var (
	ErrArtifactNotFound = errors.New("Canonical Artifact 不存在")
	ErrArtifactCorrupt  = errors.New("Canonical Artifact 损坏")
)

// ArtifactKeyPrefix 将 Canonical 缓存挂在对应 Parsed Artifact 下，二者可独立失效。
func ArtifactKeyPrefix(parseArtifactPrefix, configHash string) string {
	return path.Join(parseArtifactPrefix, "canonical-"+configHash) + "/"
}

type ArtifactManifest struct {
	ArtifactSchemaVersion  string       `json:"artifact_schema_version"`
	CanonicalSchemaVersion string       `json:"canonical_schema_version"`
	Renderer               RendererInfo `json:"renderer"`
	ConfigHash             string       `json:"config_hash"`
	ContentHash            string       `json:"content_hash"`
	DocumentSHA256         string       `json:"document_sha256"`
	MarkdownSHA256         string       `json:"markdown_sha256"`
	SourceMapSHA256        string       `json:"source_map_sha256"`
	CreatedAt              string       `json:"created_at"`
}

type ArtifactRef struct {
	Prefix   string
	Manifest ArtifactManifest
}

type ArtifactStore struct{ store parser.ObjectStore }

func NewArtifactStore(store parser.ObjectStore) *ArtifactStore { return &ArtifactStore{store: store} }

// Resolve 只接受与当前配置及 Renderer 完全兼容的缓存；兼容性变化按未命中处理。
func (s *ArtifactStore) Resolve(ctx context.Context, prefix, configHash string, renderer RendererInfo) (*ArtifactRef, error) {
	key := path.Join(prefix, CanonicalArtifactManifestFile)
	if _, err := s.store.StatObject(ctx, key); err != nil {
		if errors.Is(err, parser.ErrObjectNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("查询 Canonical manifest 失败: %w", err)
	}
	body, err := s.readObject(ctx, key, 1<<20)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, ErrArtifactCorrupt
	}
	if manifest.ArtifactSchemaVersion != CanonicalArtifactSchemaVersion ||
		manifest.CanonicalSchemaVersion != SchemaVersion ||
		manifest.ConfigHash != configHash || manifest.Renderer != renderer {
		return nil, ErrArtifactNotFound
	}
	if manifest.ContentHash == "" || manifest.DocumentSHA256 == "" ||
		manifest.MarkdownSHA256 == "" || manifest.SourceMapSHA256 == "" {
		return nil, ErrArtifactCorrupt
	}
	return &ArtifactRef{Prefix: prefix, Manifest: manifest}, nil
}

// Save 依次写入三份内容视图，并最后写 manifest，使半成品不会被识别为有效缓存。
func (s *ArtifactStore) Save(ctx context.Context, prefix string, doc *CanonicalDocument, configHash string, renderer RendererInfo) (*ArtifactRef, error) {
	if doc == nil {
		return nil, fmt.Errorf("保存 Canonical Artifact 前文档为空")
	}
	contentHash, err := HashDocument(doc)
	if err != nil || !strings.EqualFold(contentHash, doc.ContentHash) {
		return nil, fmt.Errorf("保存 Canonical Artifact 前内容哈希无效")
	}
	documentBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("序列化 CanonicalDocument 失败: %w", err)
	}
	sourceMapBytes, err := json.Marshal(doc.SourceMap)
	if err != nil {
		return nil, fmt.Errorf("序列化 Canonical SourceMap 失败: %w", err)
	}
	markdownBytes := []byte(doc.Markdown)
	files := []struct {
		name, contentType string
		data              []byte
	}{
		{CanonicalArtifactDocumentFile, "application/octet-stream", compress(documentBytes)},
		{CanonicalArtifactMarkdownFile, "text/markdown; charset=utf-8", markdownBytes},
		{CanonicalArtifactSourceMapFile, "application/octet-stream", compress(sourceMapBytes)},
	}
	for _, file := range files {
		if err := s.store.PutObject(ctx, path.Join(prefix, file.name), bytes.NewReader(file.data), int64(len(file.data)), file.contentType); err != nil {
			return nil, fmt.Errorf("保存 Canonical Artifact %s 失败: %w", file.name, err)
		}
	}
	manifest := ArtifactManifest{
		ArtifactSchemaVersion: CanonicalArtifactSchemaVersion, CanonicalSchemaVersion: SchemaVersion,
		Renderer: renderer, ConfigHash: configHash, ContentHash: doc.ContentHash,
		DocumentSHA256: sha256Hex(documentBytes), MarkdownSHA256: sha256Hex(markdownBytes),
		SourceMapSHA256: sha256Hex(sourceMapBytes), CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if err := s.store.PutObject(ctx, path.Join(prefix, CanonicalArtifactManifestFile), bytes.NewReader(manifestBytes), int64(len(manifestBytes)), "application/json"); err != nil {
		return nil, fmt.Errorf("保存 Canonical manifest 失败: %w", err)
	}
	return &ArtifactRef{Prefix: prefix, Manifest: manifest}, nil
}

// Load 同时验证结构化文档、Markdown 视图与 SourceMap 视图，避免部分对象被篡改后继续复用。
func (s *ArtifactStore) Load(ctx context.Context, ref *ArtifactRef) (*CanonicalDocument, error) {
	documentCompressed, err := s.readObject(ctx, path.Join(ref.Prefix, CanonicalArtifactDocumentFile), canonicalArtifactReadLimit)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	documentBytes, err := decompress(documentCompressed)
	if err != nil || !strings.EqualFold(sha256Hex(documentBytes), ref.Manifest.DocumentSHA256) {
		return nil, ErrArtifactCorrupt
	}
	markdownBytes, err := s.readObject(ctx, path.Join(ref.Prefix, CanonicalArtifactMarkdownFile), canonicalArtifactReadLimit)
	if err != nil || !strings.EqualFold(sha256Hex(markdownBytes), ref.Manifest.MarkdownSHA256) {
		return nil, ErrArtifactCorrupt
	}
	sourceMapCompressed, err := s.readObject(ctx, path.Join(ref.Prefix, CanonicalArtifactSourceMapFile), canonicalArtifactReadLimit)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	sourceMapBytes, err := decompress(sourceMapCompressed)
	if err != nil || !strings.EqualFold(sha256Hex(sourceMapBytes), ref.Manifest.SourceMapSHA256) {
		return nil, ErrArtifactCorrupt
	}
	var doc CanonicalDocument
	var sourceMap []SourceSpan
	if json.Unmarshal(documentBytes, &doc) != nil || json.Unmarshal(sourceMapBytes, &sourceMap) != nil {
		return nil, ErrArtifactCorrupt
	}
	if doc.Markdown != string(markdownBytes) || !equalJSON(doc.SourceMap, sourceMap) ||
		doc.SchemaVersion != ref.Manifest.CanonicalSchemaVersion || doc.RendererVersion != ref.Manifest.Renderer.Version {
		return nil, ErrArtifactCorrupt
	}
	contentHash, err := HashDocument(&doc)
	if err != nil || !strings.EqualFold(contentHash, doc.ContentHash) || !strings.EqualFold(contentHash, ref.Manifest.ContentHash) {
		return nil, ErrArtifactCorrupt
	}
	return &doc, nil
}

func (s *ArtifactStore) readObject(ctx context.Context, key string, limit int64) ([]byte, error) {
	reader, err := s.store.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrArtifactCorrupt
	}
	return data, nil
}

func compress(data []byte) []byte {
	encoder, _ := zstd.NewWriter(nil)
	defer encoder.Close()
	return encoder.EncodeAll(data, nil)
}

func decompress(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(canonicalArtifactReadLimit))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(data, nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func equalJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}
