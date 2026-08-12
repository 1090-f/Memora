package pipeline

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/transformer"
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
	if chunk.FTSTokens == "" {
		t.Error("fts_tokens 为空")
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

func hashOfParseOptions() string {
	h, _ := parser.ParseConfigHash(parser.DefaultParseOptions())
	return h
}
