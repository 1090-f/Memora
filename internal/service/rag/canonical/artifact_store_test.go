package canonical

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

type memoryObjectStore struct{ objects map[string][]byte }

func newMemoryObjectStore() *memoryObjectStore {
	return &memoryObjectStore{objects: make(map[string][]byte)}
}

func (s *memoryObjectStore) OpenObject(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, parser.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memoryObjectStore) PutObject(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err == nil {
		s.objects[key] = data
	}
	return err
}

func (s *memoryObjectStore) StatObject(_ context.Context, key string) (*parser.ObjectInfo, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, parser.ErrObjectNotFound
	}
	return &parser.ObjectInfo{Key: key, Size: int64(len(data))}, nil
}

func (s *memoryObjectStore) RemoveObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func (s *memoryObjectStore) Bucket() string { return "test" }

func canonicalTestDocument(t *testing.T) (*parser.ParsedDocument, *CanonicalDocument, RendererInfo) {
	t.Helper()
	parsed := &parser.ParsedDocument{
		SchemaVersion: parser.SchemaVersion,
		Source:        parser.SourceInfo{FileName: "a.md", Format: "markdown", SHA256: "source"},
		Parser:        parser.ParserInfo{Name: "go-markdown", Version: parser.GoParserVersion},
		Blocks:        []parser.Block{{ID: "b1", Type: parser.BlockTypeParagraph, Text: "正文"}},
	}
	renderer := NewParsedDocumentRenderer(RenderOptions{})
	doc, err := renderer.Render(context.Background(), parsed)
	if err != nil {
		t.Fatalf("渲染测试文档失败: %v", err)
	}
	return parsed, doc, renderer.Info()
}

func TestArtifactStoreRoundTripAndCompatibility(t *testing.T) {
	ctx := context.Background()
	objects := newMemoryObjectStore()
	store := NewArtifactStore(objects)
	parsed, doc, renderer := canonicalTestDocument(t)
	configHash, err := ArtifactConfigHash(parsed, renderer, `{"mode":"stable"}`)
	if err != nil {
		t.Fatal(err)
	}
	prefix := ArtifactKeyPrefix("derived/u/d/content-1/parse-x/", configHash)
	ref, err := store.Save(ctx, prefix, doc, configHash, renderer)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	for _, name := range []string{CanonicalArtifactManifestFile, CanonicalArtifactDocumentFile, CanonicalArtifactMarkdownFile, CanonicalArtifactSourceMapFile} {
		if _, ok := objects.objects[prefix+name]; !ok {
			t.Errorf("缺少 Artifact 文件 %s", name)
		}
	}
	resolved, err := store.Resolve(ctx, prefix, configHash, renderer)
	if err != nil {
		t.Fatalf("查找失败: %v", err)
	}
	loaded, err := store.Load(ctx, resolved)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if loaded.ContentHash != doc.ContentHash || loaded.Markdown != doc.Markdown {
		t.Fatalf("往返结果不一致: %+v", loaded)
	}
	if _, err := store.Resolve(ctx, prefix, "changed", renderer); err != ErrArtifactNotFound {
		t.Fatalf("配置变化应返回未命中，实际 %v", err)
	}
	_ = ref
}

func TestArtifactStoreRejectsCorruptView(t *testing.T) {
	ctx := context.Background()
	objects := newMemoryObjectStore()
	store := NewArtifactStore(objects)
	parsed, doc, renderer := canonicalTestDocument(t)
	configHash, _ := ArtifactConfigHash(parsed, renderer, "")
	prefix := ArtifactKeyPrefix("derived/u/d/content-1/parse-x/", configHash)
	ref, err := store.Save(ctx, prefix, doc, configHash, renderer)
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[prefix+CanonicalArtifactMarkdownFile] = []byte("被篡改")
	if _, err := store.Load(ctx, ref); err != ErrArtifactCorrupt {
		t.Fatalf("篡改 Markdown 应拒绝复用，实际 %v", err)
	}
}
