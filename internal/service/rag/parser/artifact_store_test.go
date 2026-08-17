package parser

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// memoryObjectStore 是 ObjectStore 的内存实现，用于 ArtifactStore 单测。
type memoryObjectStore struct {
	objects map[string][]byte
}

func newMemoryObjectStore() *memoryObjectStore {
	return &memoryObjectStore{objects: make(map[string][]byte)}
}

func (m *memoryObjectStore) PutObject(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.objects[key] = data
	return nil
}

func (m *memoryObjectStore) OpenObject(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memoryObjectStore) StatObject(_ context.Context, key string) (*ObjectInfo, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return &ObjectInfo{Key: key, Size: int64(len(data))}, nil
}

func (m *memoryObjectStore) RemoveObject(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func (m *memoryObjectStore) Bucket() string { return "test" }

// testParsedDocument 构造带单个 picture Block + Asset 的最小合法 ParsedDocument。
func testParsedDocument(asset Asset) *ParsedDocument {
	doc := &ParsedDocument{
		SchemaVersion: SchemaVersion,
		Parser: ParserInfo{
			Name:           ParserNameGoMarkdown,
			Version:        "1.0",
			AdapterVersion: AdapterVersion,
		},
		Source: SourceInfo{
			FileName: "test.md",
			Format:   "markdown",
			SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:     1,
		},
		Document: DocumentInfo{
			Title:     "test",
			Markdown:  "正文",
			PageCount: 0,
			Metadata:  map[string]any{},
		},
		Blocks: []Block{
			{
				ID:          "block-000001",
				Type:        BlockTypeParagraph,
				Text:        "正文",
				Markdown:    "正文",
				HeadingPath: []string{},
				Source:      SourceLocation{Page: 0},
				AssetRefs:   []string{asset.ID},
			},
		},
		Assets: []Asset{asset},
	}
	return doc
}

// TestArtifactStoreSaveDegradesOversizedAsset 验证超单图限制的资产降级为 Omitted，
// 不阻断整篇文档的 Artifact 保存。
func TestArtifactStoreSaveDegradesOversizedAsset(t *testing.T) {
	store := newMemoryObjectStore()
	limits := DefaultValidateLimits()
	limits.MaxAssetBytes = 16
	artifacts := NewArtifactStore(store, limits)

	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/png",
		Caption:    "大图",
		DataBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)),
	})

	if _, err := artifacts.Save(context.Background(), "prefix/", doc, "cfg-hash"); err != nil {
		t.Fatalf("超限资产应降级跳过而非失败: %v", err)
	}
	if !doc.Assets[0].Omitted {
		t.Error("超限资产应被标记 Omitted")
	}
	if doc.Assets[0].DataBase64 != "" {
		t.Error("超限资产的 base64 应被清空")
	}
	if len(doc.Warnings) == 0 || !strings.Contains(doc.Warnings[0], "已跳过") {
		t.Errorf("应追加降级 warning，实际: %v", doc.Warnings)
	}
}

// TestArtifactStoreSaveDegradesUnsupportedMIME 验证 SVG 等不受支持类型降级为 Omitted。
func TestArtifactStoreSaveDegradesUnsupportedMIME(t *testing.T) {
	store := newMemoryObjectStore()
	artifacts := NewArtifactStore(store, DefaultValidateLimits())

	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/svg+xml",
		Caption:    "svg",
		DataBase64: base64.StdEncoding.EncodeToString([]byte("<svg></svg>")),
	})

	if _, err := artifacts.Save(context.Background(), "prefix/", doc, "cfg-hash"); err != nil {
		t.Fatalf("SVG 资产应降级跳过而非失败: %v", err)
	}
	if !doc.Assets[0].Omitted {
		t.Error("SVG 资产应被标记 Omitted")
	}
	if len(doc.Warnings) == 0 || !strings.Contains(doc.Warnings[0], "不受支持") {
		t.Errorf("应追加类型不受支持 warning，实际: %v", doc.Warnings)
	}
}

// TestValidateAssetsPreCheckOversized 验证校验器在解码前按 base64 长度拒绝超限资产。
func TestValidateAssetsPreCheckOversized(t *testing.T) {
	limits := DefaultValidateLimits()
	limits.MaxAssetBytes = 16
	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/png",
		DataBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)),
	})
	err := ValidateParsedDocument(doc, "", limits)
	if err == nil {
		t.Fatal("超限资产应被校验拒绝")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Kind != ParseErrorInvalidResponse {
		t.Errorf("错误应为 ParseErrorInvalidResponse，实际: %v", err)
	}
}

// TestIsAllowedAssetMIME 验证资产 MIME 白名单。
func TestIsAllowedAssetMIME(t *testing.T) {
	allowed := []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "IMAGE/PNG"}
	for _, mime := range allowed {
		if !isAllowedAssetMIME(mime) {
			t.Errorf("isAllowedAssetMIME(%q) = false, 期望 true", mime)
		}
	}
	rejected := []string{"image/svg+xml", "IMAGE/SVG+XML", "text/html", "application/pdf", ""}
	for _, mime := range rejected {
		if isAllowedAssetMIME(mime) {
			t.Errorf("isAllowedAssetMIME(%q) = true, 期望 false", mime)
		}
	}
}

// TestMarkdownParserRejectsSVGDataURI 验证 Markdown 内嵌 SVG data URI 被拒绝并记录 warning。
func TestMarkdownParserRejectsSVGDataURI(t *testing.T) {
	p := NewMarkdownParser(1 << 20)
	svg := base64.StdEncoding.EncodeToString([]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"><script>alert(1)</script></svg>"))
	content := "![图标](data:image/svg+xml;base64," + svg + ")\n\n正文"
	doc, err := p.Parse(context.Background(), ParseInput{
		FileName: "test.md",
		Content:  strings.NewReader(content),
		Size:     int64(len(content)),
		Options:  DefaultParseOptions(),
	})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(doc.Assets) != 0 {
		t.Errorf("SVG 不应生成 Asset，实际 %d 个", len(doc.Assets))
	}
	if len(doc.Warnings) == 0 || !strings.Contains(doc.Warnings[0], "不受支持") {
		t.Errorf("应记录 SVG 不受支持 warning，实际: %v", doc.Warnings)
	}
}

// flakyObjectStore 是带故障注入的 ObjectStore 包装：指定 key 前 N 次 PutObject 失败。
type flakyObjectStore struct {
	inner    ObjectStore
	mu       sync.Mutex
	failures map[string]int
	attempts map[string]int
}

func (f *flakyObjectStore) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	f.mu.Lock()
	f.attempts[key]++
	attempt := f.attempts[key]
	failed := f.failures[key]
	f.mu.Unlock()
	if attempt <= failed {
		return errors.New("注入的上传失败")
	}
	return f.inner.PutObject(ctx, key, reader, size, contentType)
}

func (f *flakyObjectStore) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return f.inner.OpenObject(ctx, key)
}

func (f *flakyObjectStore) StatObject(ctx context.Context, key string) (*ObjectInfo, error) {
	return f.inner.StatObject(ctx, key)
}

func (f *flakyObjectStore) RemoveObject(ctx context.Context, key string) error {
	return f.inner.RemoveObject(ctx, key)
}

func (f *flakyObjectStore) Bucket() string { return f.inner.Bucket() }

// TestArtifactStoreSaveDegradesOverBudgetAssets 验证资产总量超预算时降级：
// 优先丢弃未被 Block 引用的资产，Save 不再整体失败。
func TestArtifactStoreSaveDegradesOverBudgetAssets(t *testing.T) {
	store := newMemoryObjectStore()
	limits := DefaultValidateLimits()
	limits.MaxTotalAssetBytes = 100
	artifacts := NewArtifactStore(store, limits)

	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/png",
		DataBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x41}, 60)),
	})
	doc.Assets = append(doc.Assets, Asset{
		ID:         "asset-000002",
		Kind:       "picture",
		MIMEType:   "image/png",
		DataBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 60)),
	})

	if _, err := artifacts.Save(context.Background(), "prefix/", doc, "cfg-hash"); err != nil {
		t.Fatalf("总量超预算应降级跳过而非失败: %v", err)
	}
	if doc.Assets[0].Omitted {
		t.Error("被 Block 引用的资产应优先保留")
	}
	if !doc.Assets[1].Omitted {
		t.Error("未被引用的超预算资产应被标记 Omitted")
	}
	if doc.Assets[1].DataBase64 != "" {
		t.Error("被跳过资产的 base64 应被清空")
	}
	found := false
	for _, warning := range doc.Warnings {
		if strings.Contains(warning, "超出资产总量预算") {
			found = true
		}
	}
	if !found {
		t.Errorf("应追加总量预算 warning，实际: %v", doc.Warnings)
	}
}

// TestArtifactStoreSaveRetriesAssetUpload 验证单资产上传失败后重试并最终成功。
func TestArtifactStoreSaveRetriesAssetUpload(t *testing.T) {
	inner := newMemoryObjectStore()
	flaky := &flakyObjectStore{inner: inner, failures: map[string]int{}, attempts: map[string]int{}}
	// 资产对象 key 首次写入注入一次失败，重试后成功。
	assetKey := "prefix/assets/asset-000001.png"
	flaky.failures[assetKey] = 1
	artifacts := NewArtifactStore(flaky, DefaultValidateLimits())

	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/png",
		DataBase64: base64.StdEncoding.EncodeToString([]byte("png-bytes")),
	})

	if _, err := artifacts.Save(context.Background(), "prefix/", doc, "cfg-hash"); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if flaky.attempts[assetKey] != 2 {
		t.Errorf("资产上传尝试次数 = %d, 期望 2（失败 1 次 + 重试成功）", flaky.attempts[assetKey])
	}
	if _, err := inner.StatObject(context.Background(), assetKey); err != nil {
		t.Errorf("重试成功后资产对象应存在: %v", err)
	}
}

// TestArtifactStoreSaveRetriesConcurrently 验证并发上传下资产完整性与 manifest 顺序。
func TestArtifactStoreSaveRetriesConcurrently(t *testing.T) {
	store := newMemoryObjectStore()
	artifacts := NewArtifactStore(store, DefaultValidateLimits())

	doc := testParsedDocument(Asset{
		ID:         "asset-000001",
		Kind:       "picture",
		MIMEType:   "image/png",
		DataBase64: base64.StdEncoding.EncodeToString([]byte("one")),
	})
	for i := 2; i <= 8; i++ {
		doc.Assets = append(doc.Assets, Asset{
			ID:         fmt.Sprintf("asset-%06d", i),
			Kind:       "picture",
			MIMEType:   "image/png",
			DataBase64: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("data-%d", i))),
		})
	}

	ref, err := artifacts.Save(context.Background(), "prefix/", doc, "cfg-hash")
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if len(ref.Manifest.Assets) != len(doc.Assets) {
		t.Fatalf("manifest 资产数 = %d, 期望 %d", len(ref.Manifest.Assets), len(doc.Assets))
	}
	for i, info := range ref.Manifest.Assets {
		if info.ID != doc.Assets[i].ID {
			t.Errorf("manifest 顺序错乱: 位置 %d ID = %q, 期望 %q", i, info.ID, doc.Assets[i].ID)
		}
		if info.ObjectKey == "" || info.SHA256 == "" {
			t.Errorf("资产 %q 元信息不完整: %+v", info.ID, info)
		}
		if _, err := store.StatObject(context.Background(), info.ObjectKey); err != nil {
			t.Errorf("资产 %q 对象缺失: %v", info.ID, err)
		}
	}
	for _, asset := range doc.Assets {
		if asset.DataBase64 != "" || asset.ObjectKey == "" {
			t.Errorf("资产 %q 保存后应替换为 ObjectKey: %+v", asset.ID, asset)
		}
	}
}
