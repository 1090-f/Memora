package pipeline

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/transformer"
	"github.com/klauspost/compress/zstd"
)

// fakeStore 是内存版 ObjectStore。
type fakeStore struct {
	objects map[string][]byte
	content map[string]string
	opens   map[string]int
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, content: map[string]string{}, opens: map[string]int{}}
}

func (f *fakeStore) OpenObject(_ context.Context, objectKey string) (io.ReadCloser, error) {
	f.opens[objectKey]++
	data, ok := f.objects[objectKey]
	if !ok {
		return nil, parser.ErrObjectNotFound
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

func (f *fakeStore) StatObject(_ context.Context, objectKey string) (*parser.ObjectInfo, error) {
	data, ok := f.objects[objectKey]
	if !ok {
		return nil, parser.ErrObjectNotFound
	}
	return &parser.ObjectInfo{Key: objectKey, Size: int64(len(data)), ContentType: f.content[objectKey]}, nil
}

func (f *fakeStore) RemoveObject(_ context.Context, objectKey string) error {
	delete(f.objects, objectKey)
	return nil
}

func (f *fakeStore) Bucket() string { return "memora" }

// fakeChunkWriter 记录批量插入。
type fakeChunkWriter struct {
	inserts [][]*entity.DocumentChunk
}

func (w *fakeChunkWriter) BatchInsert(_ context.Context, chunks []*entity.DocumentChunk) ([]string, error) {
	w.inserts = append(w.inserts, chunks)
	ids := make([]string, len(chunks))
	for i := range chunks {
		ids[i] = "chunk-" + itoa(i)
	}
	return ids, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func testPipelineConfig(store *fakeStore, chunkConfig string) DocumentPipelineConfig {
	return DocumentPipelineConfig{
		Store:        store,
		Chunks:       &fakeChunkWriter{},
		ChunkConfig:  chunkConfig,
		ChunkOptions: chunking.DefaultChunkOptions(),
		Tokenizer:    chunking.NewHeuristicTokenizer(),
		ParseOptions: parser.DefaultParseOptions(),
		// BaseURL 指向不可达端口：TXT/Markdown 不应调用 Python 服务。
		ParserConfig:   parser.PythonParserConfig{BaseURL: "http://127.0.0.1:1", Timeout: 1},
		ValidateLimits: parser.DefaultValidateLimits(),
	}
}

func processInput(store *fakeStore, fileName string) ProcessInput {
	return ProcessInput{
		ObjectKey: "documents/u1/kb1/t1/" + fileName,
		FileName:  fileName,
		DocMeta: transformer.DocMeta{
			UserID:          "u1",
			KnowledgeBaseID: "kb1",
			DocumentID:      "d1",
			IndexVersion:    1,
			ContentVersion:  1,
			ChunkVersion:    1,
			DocumentTitle:   fileName,
			SourceHash:      "",
		},
	}
}

func TestPipelineParsesTextPersistsChunksAndArtifact(t *testing.T) {
	store := newFakeStore()
	content := "# 标题\n\n第一章的内容正文。\n\n## 小节\n\n小节内容。"
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/a.md", strings.NewReader(content), int64(len(content)), "text/markdown")

	p, err := NewDocumentPipeline(testPipelineConfig(store, `{"splitter":"structure-v1","max_tokens":1000}`))
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	out, err := p.Run(context.Background(), processInput(store, "a.md"))
	if err != nil {
		t.Fatalf("运行 pipeline 失败: %v", err)
	}
	if out.ChunkCount == 0 {
		t.Fatal("未产生 Chunk")
	}
	// Artifact 已保存：manifest + 压缩文档。
	if _, ok := store.objects["derived/u1/d1/content-1/parse-"+hashOfParseOptions()+"/manifest.json"]; !ok {
		t.Error("Artifact manifest 未保存")
	}
	if _, ok := store.objects["derived/u1/d1/content-1/parse-"+hashOfParseOptions()+"/parsed-document.json.zst"]; !ok {
		t.Error("ParsedDocument 未保存")
	}
}

func TestPipelineReusesArtifactWithoutReparsing(t *testing.T) {
	store := newFakeStore()
	content := "# 标题\n\n" + strings.Repeat("正文内容内容。", 300)
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/a.md", strings.NewReader(content), int64(len(content)), "text/markdown")

	cfgA := testPipelineConfig(store, `{"splitter":"structure-v1","max_tokens":1000}`)
	pA, err := NewDocumentPipeline(cfgA)
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	if _, err := pA.Run(context.Background(), processInput(store, "a.md")); err != nil {
		t.Fatalf("首次运行失败: %v", err)
	}
	opensAfterFirst := store.opens["documents/u1/kb1/t1/a.md"]

	// 修改分块配置（chunk_config_hash 变化）：应复用 Artifact，不再读取原始文件。
	cfgB := testPipelineConfig(store, `{"splitter":"structure-v1","max_tokens":500}`)
	pB, err := NewDocumentPipeline(cfgB)
	if err != nil {
		t.Fatalf("构造 pipeline B 失败: %v", err)
	}
	if _, err := pB.Run(context.Background(), processInput(store, "a.md")); err != nil {
		t.Fatalf("重分块运行失败: %v", err)
	}
	if store.opens["documents/u1/kb1/t1/a.md"] != opensAfterFirst {
		t.Errorf("重分块不应重新读取原始文件（调用次数 %d → %d）", opensAfterFirst, store.opens["documents/u1/kb1/t1/a.md"])
	}
	// 两个 pipeline 的 chunk_config_hash 不同（配置变化）。
	if pA.ChunkConfigHash() == pB.ChunkConfigHash() {
		t.Error("chunk 配置变化后 chunk_config_hash 应变化")
	}
}

func TestPipelineChunkEntityFields(t *testing.T) {
	store := newFakeStore()
	content := "# 标题\n\n第一章内容。\n\n## 小节\n\n小节正文。"
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/a.md", strings.NewReader(content), int64(len(content)), "text/markdown")

	writer := &fakeChunkWriter{}
	cfg := testPipelineConfig(store, `{"splitter":"structure-v1","max_tokens":1000}`)
	cfg.Chunks = writer
	p, err := NewDocumentPipeline(cfg)
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	if _, err := p.Run(context.Background(), processInput(store, "a.md")); err != nil {
		t.Fatalf("运行失败: %v", err)
	}
	if len(writer.inserts) != 1 || len(writer.inserts[0]) == 0 {
		t.Fatalf("未写入 Chunk")
	}
	chunk := writer.inserts[0][0]
	if chunk.UserID != "u1" || chunk.DocumentID != "d1" || chunk.IndexVersion != 1 {
		t.Errorf("实体归属字段错误: %+v", chunk)
	}
	if chunk.TokenCount <= 0 || chunk.CharCount <= 0 {
		t.Errorf("计数异常: token=%d char=%d", chunk.TokenCount, chunk.CharCount)
	}
	if chunk.ChunkConfigHash != p.ChunkConfigHash() {
		t.Errorf("chunk_config_hash 不一致: %s != %s", chunk.ChunkConfigHash, p.ChunkConfigHash())
	}
	if len(chunk.HeadingPath) == 0 {
		t.Error("heading_path 为空")
	}
	if len(chunk.SourceLocation) == 0 {
		t.Error("source_location 为空")
	}
}

func TestPipelineTextWorksWithoutPython(t *testing.T) {
	store := newFakeStore()
	content := "纯文本内容。"
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/a.txt", strings.NewReader(content), int64(len(content)), "text/plain")
	p, err := NewDocumentPipeline(testPipelineConfig(store, `{}`))
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	if _, err := p.Run(context.Background(), processInput(store, "a.txt")); err != nil {
		t.Fatalf("TXT 不应依赖 Python 服务: %v", err)
	}
}

// TestPipelineParsesManualContentWithoutObject 验证手工文档正文直接进入流水线：
// ProcessInput.Content 非空时不访问 MinIO 对象，完成解析、分块与 Artifact 持久化。
func TestPipelineParsesManualContentWithoutObject(t *testing.T) {
	store := newFakeStore()
	p, err := NewDocumentPipeline(testPipelineConfig(store, `{}`))
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	input := ProcessInput{
		Content:  "# 手工标题\n\n这是手工文档的正文内容，需要被分块索引。",
		FileName: "手工文档.markdown",
		DocMeta: transformer.DocMeta{
			UserID: "u1", KnowledgeBaseID: "kb1", DocumentID: "d1",
			IndexVersion: 1, ContentVersion: 1, ChunkVersion: 1, DocumentTitle: "手工文档",
		},
	}
	out, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("运行 pipeline 失败: %v", err)
	}
	if out.ChunkCount == 0 {
		t.Fatal("手工文档未产生 Chunk")
	}
	// 未读取任何 MinIO 对象（正文直接来自 Content）。
	for key, count := range store.opens {
		if count > 0 {
			t.Errorf("手工文档不应打开 MinIO 对象 %s（次数 %d）", key, count)
		}
	}
	// Artifact 已保存，重试/重新分块可复用。
	if _, ok := store.objects["derived/u1/d1/content-1/parse-"+hashOfParseOptions()+"/parsed-document.json.zst"]; !ok {
		t.Error("手工文档 ParsedDocument 未保存")
	}
}

// TestPipelineAllowsZeroChunksForPictureOnlyDocument 验证纯图片文档（图片无
// OCR/caption 文字）允许 0 Chunk 成功导入：资产与原文件保留，Artifact 持久化，
// 不再把"无文字图片"当作分块器故障。
func TestPipelineAllowsZeroChunksForPictureOnlyDocument(t *testing.T) {
	store := newFakeStore()
	// 空 alt 的 data-URI 图片：MarkdownParser 产出 picture Block + Asset（Caption 为空），
	// 分块器对无文字图片不产出单元 → 0 Chunk。
	content := "![](data:image/png;base64,iVBORw0KGgo=)"
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/pic.md", strings.NewReader(content), int64(len(content)), "text/markdown")

	p, err := NewDocumentPipeline(testPipelineConfig(store, `{}`))
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	out, err := p.Run(context.Background(), processInput(store, "pic.md"))
	if err != nil {
		t.Fatalf("纯图片文档应成功导入（0 Chunk）: %v", err)
	}
	if out.ChunkCount != 0 {
		t.Errorf("ChunkCount = %d, want 0", out.ChunkCount)
	}
	// 资产与 Artifact 仍持久化：重试/重新分块可复用。
	if _, ok := store.objects["derived/u1/d1/content-1/parse-"+hashOfParseOptions()+"/parsed-document.json.zst"]; !ok {
		t.Error("纯图片文档 ParsedDocument 未保存")
	}
}

func TestAssetOnlyDocument(t *testing.T) {
	pic := parser.Block{ID: "b1", Type: parser.BlockTypePicture, AssetRefs: []string{"a1"}}
	heading := parser.Block{ID: "b2", Type: parser.BlockTypeHeading, Text: "标题"}
	paragraph := parser.Block{ID: "b3", Type: parser.BlockTypeParagraph, Text: "正文内容"}

	tests := []struct {
		name string
		doc  *parser.ParsedDocument
		want bool
	}{
		{name: "nil 文档", doc: nil, want: false},
		{name: "空文档（无任何块）", doc: &parser.ParsedDocument{}, want: true},
		{name: "只有图片", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic}}, want: true},
		{name: "图片+标题", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic, heading}}, want: true},
		{name: "空白段落", doc: &parser.ParsedDocument{Blocks: []parser.Block{{ID: "b4", Type: parser.BlockTypeParagraph, Text: "   "}}}, want: true},
		{name: "含正文段落", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic, paragraph}}, want: false},
		{name: "含表格引用", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic, {ID: "b5", Type: parser.BlockTypeTable, TableRef: "t1"}}}, want: false},
		{name: "含列表项", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic, {ID: "b6", Type: parser.BlockTypeListItem, Text: "条目"}}}, want: false},
		{name: "含代码块", doc: &parser.ParsedDocument{Blocks: []parser.Block{pic, {ID: "b7", Type: parser.BlockTypeCode, Text: "code"}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assetOnlyDocument(tt.doc); got != tt.want {
				t.Errorf("assetOnlyDocument() = %v, want %v", got, tt.want)
			}
		})
	}
}

func hashOfParseOptions() string {
	h, _ := parser.ParseConfigHash(parser.DefaultParseOptions())
	return h
}

// TestGenericFigureCaption 验证通用编号 caption 的识别。
func TestGenericFigureCaption(t *testing.T) {
	generic := []string{"图 1", "图 12", "图片", "图"}
	descriptive := []string{"", "图 1 系统架构", "架构图", "图1所示结构"}
	for _, caption := range generic {
		if !genericFigureCaption(caption) {
			t.Errorf("genericFigureCaption(%q) = false, 期望 true", caption)
		}
	}
	for _, caption := range descriptive {
		if genericFigureCaption(caption) {
			t.Errorf("genericFigureCaption(%q) = true, 期望 false", caption)
		}
	}
}

// TestPipelineOCRBackfillsGenericCaption 验证 OCR 文本回填空/通用编号 caption：
// 图片 alt 为通用编号 "图 1" 时，OCR 结果写入 caption 并随 Artifact 持久化。
func TestPipelineOCRBackfillsGenericCaption(t *testing.T) {
	ocrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ocr" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"lines": []string{"发票", "金额"}, "languages": []string{"zh"}, "engine": "test",
		})
	}))
	defer ocrSrv.Close()

	store := newFakeStore()
	png := base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G'})
	content := "![图 1](data:image/png;base64," + png + ")\n\n正文"
	_ = store.PutObject(context.Background(), "documents/u1/kb1/t1/a.md", strings.NewReader(content), int64(len(content)), "text/markdown")

	cfg := testPipelineConfig(store, `{}`)
	cfg.ParserConfig.BaseURL = ocrSrv.URL
	cfg.ParserConfig.Timeout = 5 * time.Second
	p, err := NewDocumentPipeline(cfg)
	if err != nil {
		t.Fatalf("构造 pipeline 失败: %v", err)
	}
	if _, err := p.Run(context.Background(), processInput(store, "a.md")); err != nil {
		t.Fatalf("运行 pipeline 失败: %v", err)
	}

	artifactKey := "derived/u1/d1/content-1/parse-" + hashOfParseOptions() + "/parsed-document.json.zst"
	compressed, ok := store.objects[artifactKey]
	if !ok {
		t.Fatalf("Artifact 未保存: %s", artifactKey)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatalf("构造 zstd 解码器失败: %v", err)
	}
	defer decoder.Close()
	raw, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		t.Fatalf("zstd 解码失败: %v", err)
	}
	var artifactDoc parser.ParsedDocument
	if err := json.Unmarshal(raw, &artifactDoc); err != nil {
		t.Fatalf("解析 Artifact 失败: %v", err)
	}
	if len(artifactDoc.Assets) != 1 {
		t.Fatalf("Assets 数量 = %d, 期望 1", len(artifactDoc.Assets))
	}
	if artifactDoc.Assets[0].Caption != "发票\n金额" {
		t.Errorf("通用编号 caption 应被 OCR 文本回填，实际 %q", artifactDoc.Assets[0].Caption)
	}
	if artifactDoc.Assets[0].Metadata["ocr_text"] != "发票\n金额" {
		t.Errorf("ocr_text 元数据缺失: %v", artifactDoc.Assets[0].Metadata)
	}
}
