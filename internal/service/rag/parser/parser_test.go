package parser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeStore 是内存版 ObjectStore，用于 ArtifactStore 单元测试。
type fakeStore struct {
	objects map[string][]byte
	content map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, content: map[string]string{}}
}

func (f *fakeStore) OpenObject(_ context.Context, objectKey string) (io.ReadCloser, error) {
	data, ok := f.objects[objectKey]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeStore) PutObject(_ context.Context, objectKey string, reader io.Reader, _ int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.objects[objectKey] = data
	f.content[objectKey] = contentType
	return nil
}

func (f *fakeStore) StatObject(_ context.Context, objectKey string) (*ObjectInfo, error) {
	data, ok := f.objects[objectKey]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return &ObjectInfo{Key: objectKey, Size: int64(len(data)), ContentType: f.content[objectKey]}, nil
}

func (f *fakeStore) RemoveObject(_ context.Context, objectKey string) error {
	delete(f.objects, objectKey)
	return nil
}

func (f *fakeStore) Bucket() string { return "memora" }

// testDocument 构造最小合法 ParsedDocument（含一张表与一张图）。
func testDocument() *ParsedDocument {
	return &ParsedDocument{
		SchemaVersion: SchemaVersion,
		Parser:        ParserInfo{Name: ParserNameDocling, Version: "2.118.1", AdapterVersion: AdapterVersion},
		Source:        SourceInfo{FileName: "a.pdf", Format: "pdf", SHA256: strings.Repeat("ab", 32), Size: 10},
		Document:      DocumentInfo{Title: "t", PageCount: 1},
		Blocks: []Block{
			{ID: "block-000001", Type: BlockTypeHeading, Text: "第一章", HeadingPath: []string{"第一章"}},
			{ID: "block-000002", Type: BlockTypeTable, Text: "| a | b |", TableRef: "table-000001"},
			{ID: "block-000003", Type: BlockTypePicture, AssetRefs: []string{"asset-000001"}},
		},
		Tables: []Table{{ID: "table-000001", RowCount: 1, ColumnCount: 2, Headers: [][]string{{"a", "b"}}}},
		Assets: []Asset{{
			ID: "asset-000001", Kind: "picture", MIMEType: "image/png",
			DataBase64: "iVBORw0KGgo=", // 上述 7 字节的 base64
			Width:      1, Height: 1, Page: 1,
		}},
	}
}

func TestArtifactKeyPrefixDeterministic(t *testing.T) {
	first := ArtifactKeyPrefix("u1", "d1", 3, "hash123")
	second := ArtifactKeyPrefix("u1", "d1", 3, "hash123")
	if first != second {
		t.Fatalf("相同参数 key 不一致: %q != %q", first, second)
	}
	want := "derived/u1/d1/content-3/parse-hash123/"
	if first != want {
		t.Errorf("key = %q, 期望 %q", first, want)
	}
	if ArtifactKeyPrefix("u1", "d1", 3, "other") == first {
		t.Error("parse config hash 变化后 key 应变化")
	}
	if ArtifactKeyPrefix("u1", "d1", 4, "hash123") == first {
		t.Error("content version 变化后 key 应变化")
	}
}

func TestArtifactStoreSaveAndResolveAndLoad(t *testing.T) {
	ctx := context.Background()
	store := NewArtifactStore(newFakeStore(), DefaultValidateLimits())
	doc := testDocument()
	prefix := ArtifactKeyPrefix("u1", "d1", 1, "parse-hash")

	ref, err := store.Save(ctx, prefix, doc, "parse-hash")
	if err != nil {
		t.Fatalf("保存 Artifact 失败: %v", err)
	}
	if ref.Manifest.SourceSHA256 != doc.Source.SHA256 {
		t.Errorf("manifest 源哈希不一致")
	}
	if ref.Manifest.ParserName != ParserNameDocling {
		t.Errorf("manifest parser 名不一致")
	}
	if len(ref.Manifest.Assets) != 1 || ref.Manifest.Assets[0].ObjectKey == "" {
		t.Errorf("manifest 资产信息不完整: %+v", ref.Manifest.Assets)
	}
	// 保存后 base64 已替换为 object_key。
	if doc.Assets[0].DataBase64 != "" || doc.Assets[0].ObjectKey == "" {
		t.Errorf("保存后 Asset 应为 object_key 引用: %+v", doc.Assets[0])
	}

	resolved, err := store.Resolve(ctx, prefix, doc.Source.SHA256)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if resolved.Manifest.ParsedDocumentSHA256 != ref.Manifest.ParsedDocumentSHA256 {
		t.Errorf("Resolve 与 Save 的 manifest 不一致")
	}

	loaded, err := store.Load(ctx, resolved)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(loaded.Blocks) != 3 || loaded.Blocks[1].TableRef != "table-000001" {
		t.Errorf("Load 内容不一致: %+v", loaded.Blocks)
	}
	if loaded.Assets[0].ObjectKey == "" {
		t.Error("Load 后 Asset 应带 object_key")
	}
}

func TestArtifactStoreResolveMissing(t *testing.T) {
	store := NewArtifactStore(newFakeStore(), DefaultValidateLimits())
	ctx := context.Background()
	prefix := ArtifactKeyPrefix("u1", "d1", 1, "h")
	if _, err := store.Resolve(ctx, prefix, ""); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("期望 ErrArtifactNotFound，实际 %v", err)
	}
}

func TestArtifactStoreRejectCorruptManifest(t *testing.T) {
	fake := newFakeStore()
	store := NewArtifactStore(fake, DefaultValidateLimits())
	ctx := context.Background()
	prefix := ArtifactKeyPrefix("u1", "d1", 1, "h")
	// 只有 manifest 但没有文档：不完整 → 拒绝。
	_ = fake.PutObject(ctx, prefix+"manifest.json", strings.NewReader(`{"artifact_schema_version":"1.0"}`), -1, "application/json")
	if _, err := store.Resolve(ctx, prefix, ""); !errors.Is(err, ErrArtifactCorrupt) {
		t.Fatalf("期望 ErrArtifactCorrupt，实际 %v", err)
	}
	// manifest 缺失时按未找到处理。
	prefix2 := ArtifactKeyPrefix("u1", "d1", 2, "h")
	if _, err := store.Resolve(ctx, prefix2, ""); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("期望 ErrArtifactNotFound，实际 %v", err)
	}
}

func TestArtifactStoreRejectCorruptDocumentHash(t *testing.T) {
	fake := newFakeStore()
	store := NewArtifactStore(fake, DefaultValidateLimits())
	ctx := context.Background()
	prefix := ArtifactKeyPrefix("u1", "d1", 1, "h")
	doc := testDocument()
	if _, err := store.Save(ctx, prefix, doc, "parse-hash"); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 篡改文档内容 → 哈希校验失败 → 拒绝复用。
	key := prefix + ArtifactDocumentFile
	fake.objects[key] = zstdCompress([]byte(`{"schema_version":"1.0","blocks":[]}`))
	if _, err := store.Load(ctx, &ArtifactRef{Prefix: prefix, Manifest: (func() ArtifactManifest {
		ref, _ := store.Resolve(ctx, prefix, "")
		return ref.Manifest
	})()}); !errors.Is(err, ErrArtifactCorrupt) {
		t.Fatalf("期望 ErrArtifactCorrupt，实际 %v", err)
	}
}

func TestArtifactStoreRejectMissingAsset(t *testing.T) {
	fake := newFakeStore()
	store := NewArtifactStore(fake, DefaultValidateLimits())
	ctx := context.Background()
	prefix := ArtifactKeyPrefix("u1", "d1", 1, "h")
	if _, err := store.Save(ctx, prefix, testDocument(), "parse-hash"); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 删除资产对象 → Load 拒绝复用。
	for key := range fake.objects {
		if strings.Contains(key, "/assets/") {
			delete(fake.objects, key)
		}
	}
	ref, err := store.Resolve(ctx, prefix, "")
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if _, err := store.Load(ctx, ref); !errors.Is(err, ErrArtifactCorrupt) {
		t.Fatalf("期望 ErrArtifactCorrupt，实际 %v", err)
	}
}

func TestTextParserParseMarkdown(t *testing.T) {
	content := "# 标题\n\n正文第一段。\n\n## 小节\n\n- 列表项一\n- 列表项二\n\n```\ncode block\n```\n\n结尾段落。"
	parser := NewTextParser(0)
	doc, err := parser.Parse(context.Background(), ParseInput{
		FileName: "doc.md",
		Content:  strings.NewReader(content),
		Size:     int64(len(content)),
		Options:  DefaultParseOptions(),
	})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.SchemaVersion != SchemaVersion {
		t.Errorf("schema 版本错误: %s", doc.SchemaVersion)
	}
	if doc.Parser.Name != ParserNameGoText {
		t.Errorf("parser 名错误: %s", doc.Parser.Name)
	}
	digest := sha256.Sum256([]byte(content))
	if doc.Source.SHA256 != hex.EncodeToString(digest[:]) {
		t.Errorf("源哈希错误")
	}
	if doc.Document.Title != "标题" {
		t.Errorf("标题 = %q", doc.Document.Title)
	}
	// 校验 Block 结构与 heading_path。
	var gotHeading bool
	for _, block := range doc.Blocks {
		switch block.Type {
		case BlockTypeHeading:
			gotHeading = true
			if block.Text == "小节" && len(block.HeadingPath) != 2 {
				t.Errorf("小节 heading_path = %v，期望 2 级", block.HeadingPath)
			}
		case BlockTypeCode:
			if !strings.Contains(block.Markdown, "```") {
				t.Errorf("code block markdown 缺少围栏")
			}
		}
	}
	if !gotHeading {
		t.Error("缺少 heading Block")
	}
}

func TestTextParserRejectsEmptyAndOversize(t *testing.T) {
	parser := NewTextParser(16)
	if _, err := parser.Parse(context.Background(), ParseInput{FileName: "a.txt", Content: strings.NewReader("   \n\n "), Size: 6}); err == nil {
		t.Error("全空白文件应报错")
	}
	if _, err := parser.Parse(context.Background(), ParseInput{FileName: "a.txt", Content: strings.NewReader(strings.Repeat("x", 17)), Size: 17}); err == nil {
		t.Error("超过大小限制应报错")
	}
}

func TestRouterRoutesByExtension(t *testing.T) {
	text := NewTextParser(0)
	python := &PythonDocumentParser{cfg: PythonParserConfig{BaseURL: "http://x"}}
	router := NewParserRouter(text, python)
	for _, tc := range []struct {
		file   string
		parser Parser
	}{
		{"a.txt", text}, {"a.md", text}, {"a.Markdown", text},
		{"a.pdf", python}, {"a.docx", python}, {"a.PDF", python},
	} {
		got, err := router.Route(tc.file)
		if err != nil {
			t.Fatalf("路由 %q 失败: %v", tc.file, err)
		}
		if got != tc.parser {
			t.Errorf("路由 %q 得到 %T，期望 %T", tc.file, got, tc.parser)
		}
	}
	for _, bad := range []string{"a.bin", "a.ppt", "a"} {
		if _, err := router.Route(bad); err == nil {
			t.Errorf("格式 %q 应报错", bad)
		}
	}
}

func TestRouterParsesTextEndToEnd(t *testing.T) {
	router := NewParserRouter(NewTextParser(0), nil)
	content := "hello world"
	doc, err := router.Parse(context.Background(), ParseInput{
		FileName: "x.txt",
		Content:  strings.NewReader(content),
		Size:     int64(len(content)),
		Options:  DefaultParseOptions(),
	})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.Source.Format != "txt" {
		t.Errorf("format = %q", doc.Source.Format)
	}
}
