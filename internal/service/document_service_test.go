package service

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/pkg/asseturl"
	"github.com/1090-f/Memora/pkg/objectstore"
)

func TestDocumentIndexMode(t *testing.T) {
	activeVersion := 1
	embeddingModelID := "embedding-model"

	tests := []struct {
		name string
		doc  *entity.Document
		want string
	}{
		{name: "nil document", want: string(contracts.DocumentIndexNone)},
		{name: "no active index", doc: &entity.Document{}, want: string(contracts.DocumentIndexNone)},
		{name: "keyword index", doc: &entity.Document{ActiveIndexVersion: &activeVersion}, want: string(contracts.DocumentIndexKeyword)},
		{name: "hybrid index", doc: &entity.Document{ActiveIndexVersion: &activeVersion, EmbeddingModelID: &embeddingModelID}, want: string(contracts.DocumentIndexHybrid)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := documentIndexMode(tt.doc); got != tt.want {
				t.Fatalf("documentIndexMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteMarkdownImageRefs(t *testing.T) {
	assets := []parser.Asset{
		{ID: "asset-aaaa", SourceRef: "img/logo.png", ObjectKey: "o1", MIMEType: "image/png"},
		{ID: "asset-bbbb", SourceRef: "https://example.com/pic.jpg", ObjectKey: "o2"},
		{ID: "asset-cccc", SourceRef: "missing.png", ObjectKey: "", Omitted: false},
	}
	content := strings.Join([]string{
		"# 标题",
		"![Logo](img/logo.png)",
		"![网络图](https://example.com/pic.jpg)",
		"![缺图](missing.png)",
		"![绝对路径](C:\\Users\\foo\\Desktop\\a.jpg)",
	}, "\n")

	svc := &documentService{assetSignKey: "test-secret"}
	got := svc.rewriteMarkdownImageRefs(content, assets, "doc-1")

	wantContains := []string{
		"![Logo](/api/v1/documents/doc-1/assets/asset-aaaa?exp=",
		"![网络图](/api/v1/documents/doc-1/assets/asset-bbbb?exp=",
		"![缺图](missing.png)",
		"![绝对路径](C:\\Users\\foo\\Desktop\\a.jpg)",
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("重写结果缺少 %q：\n%s", want, got)
		}
	}
}

func TestRewriteDoclingImagePlaceholders(t *testing.T) {
	assets := []parser.Asset{
		{ID: "asset-aaaa", ObjectKey: "o1"},
		{ID: "asset-omitted", ObjectKey: "", Omitted: true},
		{ID: "asset-bbbb", ObjectKey: "o2"},
	}
	blocks := []parser.Block{
		{Type: parser.BlockTypeHeading},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-aaaa"}},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-omitted"}},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-bbbb"}},
	}
	markdown := strings.Join([]string{
		"# 标题",
		"<!-- image -->",
		"中间文字",
		"<!-- image -->",
		"<!-- image -->",
	}, "\n")

	svc := &documentService{assetSignKey: "test-secret"}
	got := svc.rewriteDoclingImagePlaceholders(markdown, blocks, assets, "doc-1")

	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[1], "/api/v1/documents/doc-1/assets/asset-aaaa?exp=") {
		t.Errorf("第一个占位符应替换为 asset-aaaa 签名 URL: %q", lines[1])
	}
	if lines[3] != "<!-- image -->" {
		t.Errorf("omitted 图片的占位符应保持原样: %q", lines[3])
	}
	if !strings.Contains(lines[4], "/api/v1/documents/doc-1/assets/asset-bbbb?exp=") {
		t.Errorf("第三个占位符应替换为 asset-bbbb 签名 URL: %q", lines[4])
	}
}

func TestAssetURLSignAndVerify(t *testing.T) {
	exp, sig, err := asseturl.Sign("secret", "doc-1", "asset-1", time.Hour)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if !asseturl.Verify("secret", "doc-1", "asset-1", exp, sig) {
		t.Error("合法签名应通过校验")
	}
	if asseturl.Verify("wrong", "doc-1", "asset-1", exp, sig) {
		t.Error("错误密钥应校验失败")
	}
	if asseturl.Verify("secret", "doc-2", "asset-1", exp, sig) {
		t.Error("篡改文档 ID 应校验失败")
	}
	expiredExp, expiredSig, _ := asseturl.Sign("secret", "doc-1", "asset-1", 2*time.Second)
	time.Sleep(3500 * time.Millisecond)
	if asseturl.Verify("secret", "doc-1", "asset-1", expiredExp, expiredSig) {
		t.Error("过期签名应校验失败")
	}
}

// ---------------------------------------------------------------- 预览渲染测试

// strPtr 返回字符串指针。
func strPtr(value string) *string { return &value }

// memoryObjectStore 是内存版 ObjectStore（预览缓存测试用）。
type memoryObjectStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemoryObjectStore() *memoryObjectStore {
	return &memoryObjectStore{objects: map[string][]byte{}}
}

func (m *memoryObjectStore) Bucket() string { return "test-bucket" }

func (m *memoryObjectStore) PutObject(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = data
	return nil
}

func (m *memoryObjectStore) OpenObject(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return nil, objectstore.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memoryObjectStore) StatObject(_ context.Context, key string) (*objectstore.ObjectInfo, error) {
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return nil, objectstore.ErrObjectNotFound
	}
	return &objectstore.ObjectInfo{Key: key, Size: int64(len(data))}, nil
}

func (m *memoryObjectStore) RemoveObject(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// fakeOfficeConverter 是 OfficePDFConverter 的测试替身：转换一次生成固定 PDF。
type fakeOfficeConverter struct {
	available bool
	converted int
	pdfBytes  []byte
	convErr   error
}

func (f *fakeOfficeConverter) Available() bool { return f.available }

func (f *fakeOfficeConverter) ConvertToPDF(_ context.Context, _ string, outDir string) (string, error) {
	if f.convErr != nil {
		return "", f.convErr
	}
	f.converted++
	pdfPath := filepath.Join(outDir, "source.pdf")
	if err := os.WriteFile(pdfPath, f.pdfBytes, 0o644); err != nil {
		return "", err
	}
	return pdfPath, nil
}

// stubDocLookup 只实现 FindByID 的文档仓储替身。
type stubDocLookup struct {
	repository.DocumentRepository
	doc *entity.Document
}

func (s *stubDocLookup) FindByID(_ context.Context, _, _ string) (*entity.Document, error) {
	return s.doc, nil
}

// validPreviewPDF 构造可被预览缓存校验通过的 PDF（>1KB，以 %PDF- 开头）。
func validPreviewPDF() []byte {
	head := []byte("%PDF-1.7\n")
	body := bytes.Repeat([]byte("preview content padding.\n"), 128)
	tail := []byte("%%EOF\n")
	return append(append(head, body...), tail...)
}

func readAllCloser(reader io.ReadCloser) ([]byte, error) {
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

func TestOpenRenderedPDFReturnsOriginalWithoutConversion(t *testing.T) {
	store := newMemoryObjectStore()
	original := validPreviewPDF()
	store.objects["original/a.pdf"] = original
	doc := &entity.Document{
		UserID: "u1", SourceType: string(contracts.DocumentSourceFile),
		MinIOObjectKey: strPtr("original/a.pdf"), OriginalFileName: strPtr("a.pdf"),
		MIMEType: strPtr("application/pdf"), Title: "a.pdf", ContentVersion: 1,
	}
	doc.ID = "doc-1"
	converter := &fakeOfficeConverter{available: true}
	svc := &documentService{docs: &stubDocLookup{doc: doc}, store: store, office: converter}

	file, err := svc.OpenRendered(context.Background(), "u1", "doc-1")
	if err != nil {
		t.Fatalf("OpenRendered 失败: %v", err)
	}
	data, err := readAllCloser(file.Reader)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Error("PDF 应原样返回 MinIO 原文件")
	}
	if converter.converted != 0 {
		t.Errorf("PDF 不应触发 LibreOffice 转换（转换次数 %d）", converter.converted)
	}
	if file.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q", file.ContentType)
	}
}

func TestOpenRenderedOfficeFirstGenerationCachesAndReuses(t *testing.T) {
	store := newMemoryObjectStore()
	pdf := validPreviewPDF()
	store.objects["original/b.pptx"] = []byte("ppt binary")
	doc := &entity.Document{
		UserID: "u1", SourceType: string(contracts.DocumentSourceFile),
		MinIOObjectKey: strPtr("original/b.pptx"), OriginalFileName: strPtr("b.pptx"),
		MIMEType: strPtr("application/vnd.openxmlformats-officedocument.presentationml.presentation"),
		Title:    "b.pptx", ContentVersion: 1,
	}
	doc.ID = "doc-1"
	converter := &fakeOfficeConverter{available: true, pdfBytes: pdf}
	svc := &documentService{docs: &stubDocLookup{doc: doc}, store: store, office: converter}
	ctx := context.Background()

	first, err := svc.OpenRendered(ctx, "u1", "doc-1")
	if err != nil {
		t.Fatalf("首次 OpenRendered 失败: %v", err)
	}
	firstData, err := readAllCloser(first.Reader)
	if err != nil {
		t.Fatalf("读取首次结果失败: %v", err)
	}
	if !bytes.Equal(firstData, pdf) {
		t.Error("首次结果应为转换生成的 PDF")
	}
	if converter.converted != 1 {
		t.Errorf("首次应触发一次转换，实际 %d", converter.converted)
	}
	if _, ok := store.objects["rendered/u1/doc-1/v1.pdf"]; !ok {
		t.Error("转换结果未写入缓存对象 rendered/u1/doc-1/v1.pdf")
	}

	second, err := svc.OpenRendered(ctx, "u1", "doc-1")
	if err != nil {
		t.Fatalf("二次 OpenRendered 失败: %v", err)
	}
	secondData, err := readAllCloser(second.Reader)
	if err != nil {
		t.Fatalf("读取二次结果失败: %v", err)
	}
	if converter.converted != 1 {
		t.Errorf("缓存命中后不应再次转换（转换次数 %d）", converter.converted)
	}
	if !bytes.Equal(secondData, pdf) {
		t.Error("二次结果应为缓存 PDF")
	}
}

func TestOpenRenderedCacheStreamKeepsPDFHeader(t *testing.T) {
	store := newMemoryObjectStore()
	pdf := validPreviewPDF()
	store.objects["original/b.docx"] = []byte("docx binary")
	doc := &entity.Document{
		UserID: "u1", SourceType: string(contracts.DocumentSourceFile),
		MinIOObjectKey: strPtr("original/b.docx"), OriginalFileName: strPtr("b.docx"),
		MIMEType: strPtr("application/vnd.openxmlformats-officedocument.wordprocessingml.document"),
		Title:    "b.docx", ContentVersion: 1,
	}
	doc.ID = "doc-1"
	svc := &documentService{docs: &stubDocLookup{doc: doc}, store: store, office: &fakeOfficeConverter{available: true, pdfBytes: pdf}}
	ctx := context.Background()

	// 首次生成缓存。
	if _, err := svc.OpenRendered(ctx, "u1", "doc-1"); err != nil {
		t.Fatalf("首次生成失败: %v", err)
	}
	// 缓存命中时必须从第 0 字节返回（%PDF- 头不能被魔数校验消费掉）。
	cached, err := svc.OpenRendered(ctx, "u1", "doc-1")
	if err != nil {
		t.Fatalf("读取缓存失败: %v", err)
	}
	head := make([]byte, 5)
	if _, err := io.ReadFull(cached.Reader, head); err != nil {
		t.Fatalf("读取缓存头部失败: %v", err)
	}
	_ = cached.Reader.Close()
	if !bytes.Equal(head, []byte("%PDF-")) {
		t.Errorf("缓存流头部丢失：%q", head)
	}
}

func TestOpenRenderedDiscardsInvalidCacheAndReconverts(t *testing.T) {
	store := newMemoryObjectStore()
	pdf := validPreviewPDF()
	store.objects["original/b.xlsx"] = []byte("xlsx binary")
	store.objects["rendered/u1/doc-1/v1.pdf"] = []byte("broken cache without pdf magic")
	doc := &entity.Document{
		UserID: "u1", SourceType: string(contracts.DocumentSourceFile),
		MinIOObjectKey: strPtr("original/b.xlsx"), OriginalFileName: strPtr("b.xlsx"),
		MIMEType: strPtr("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
		Title:    "b.xlsx", ContentVersion: 1,
	}
	doc.ID = "doc-1"
	converter := &fakeOfficeConverter{available: true, pdfBytes: pdf}
	svc := &documentService{docs: &stubDocLookup{doc: doc}, store: store, office: converter}

	file, err := svc.OpenRendered(context.Background(), "u1", "doc-1")
	if err != nil {
		t.Fatalf("OpenRendered 失败: %v", err)
	}
	data, err := readAllCloser(file.Reader)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if converter.converted != 1 {
		t.Errorf("坏缓存应被剔除并重新转换（转换次数 %d）", converter.converted)
	}
	if !bytes.Equal(data, pdf) {
		t.Error("返回的应为重新转换的 PDF")
	}
	if cached, ok := store.objects["rendered/u1/doc-1/v1.pdf"]; !ok || !bytes.Equal(cached, pdf) {
		t.Error("坏缓存应被新 PDF 覆盖")
	}
}

func TestOpenRenderedOfficeUnavailableReturnsError(t *testing.T) {
	store := newMemoryObjectStore()
	store.objects["original/b.pptx"] = []byte("ppt binary")
	doc := &entity.Document{
		UserID: "u1", SourceType: string(contracts.DocumentSourceFile),
		MinIOObjectKey: strPtr("original/b.pptx"), OriginalFileName: strPtr("b.pptx"),
		MIMEType: strPtr("application/vnd.openxmlformats-officedocument.presentationml.presentation"),
		Title:    "b.pptx", ContentVersion: 1,
	}
	doc.ID = "doc-1"
	// office 为 nil：LibreOffice 不可用时应明确报错而不是返回坏数据。
	svc := &documentService{docs: &stubDocLookup{doc: doc}, store: store, office: nil}
	if _, err := svc.OpenRendered(context.Background(), "u1", "doc-1"); err == nil {
		t.Fatal("LibreOffice 不可用时应返回错误")
	}
}

func TestIsOfficeRenderableExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{ext: ".docx", want: true},
		{ext: ".xlsx", want: true},
		{ext: ".pptx", want: true},
		{ext: ".pdf", want: false},
		{ext: ".png", want: false},
		{ext: ".txt", want: false},
		{ext: ".DOCX", want: false}, // 调用方统一小写
		{ext: "", want: false},
	}
	for _, tt := range tests {
		if got := isOfficeRenderableExt(tt.ext); got != tt.want {
			t.Errorf("isOfficeRenderableExt(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}
