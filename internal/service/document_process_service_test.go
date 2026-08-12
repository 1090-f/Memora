package service

import (
	"context"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/logger"
)

// stubTaskRepo 只实现手工文档加工路径用到的导入任务仓储方法。
type stubTaskRepo struct {
	repository.ImportTaskRepository
	task      *entity.ImportTask
	completed bool
	steps     []string
}

func (s *stubTaskRepo) FindByIDInternal(_ context.Context, _ string) (*entity.ImportTask, error) {
	return s.task, nil
}

func (s *stubTaskRepo) CompleteSucceeded(_ context.Context, _ string, _ *string) error {
	s.completed = true
	return nil
}

func (s *stubTaskRepo) SetRunningStep(_ context.Context, _ string, step string) error {
	s.steps = append(s.steps, step)
	return nil
}

// stubDocRepo 只实现手工文档加工路径用到的文档仓储方法。
type stubDocRepo struct {
	repository.DocumentRepository
	doc     *entity.Document
	updates []map[string]any
}

func (s *stubDocRepo) FindByImportTask(_ context.Context, _ string) (*entity.Document, error) {
	return s.doc, nil
}

func (s *stubDocRepo) UpdateProcessing(_ context.Context, _ string, updates map[string]any) error {
	s.updates = append(s.updates, updates)
	return nil
}

// stubChunkRepo 只实现 DeleteByVersion 的分块仓储替身。
type stubChunkRepo struct {
	repository.DocumentChunkRepository
	deleted []int
}

func (s *stubChunkRepo) DeleteByVersion(_ context.Context, _ string, indexVersion int) error {
	s.deleted = append(s.deleted, indexVersion)
	return nil
}

// stubProcessor 记录最后一次加工输入。
type stubProcessor struct {
	lastInput pipeline.ProcessInput
}

func (s *stubProcessor) Run(_ context.Context, input pipeline.ProcessInput) (pipeline.ProcessOutput, error) {
	s.lastInput = input
	return pipeline.ProcessOutput{ChunkCount: 3}, nil
}

func (s *stubProcessor) ChunkConfigHash() string { return "chunk-hash" }
func (s *stubProcessor) EmbeddingModelID() string { return "" }

// TestProcessImportTaskManualDocumentRunsPipelineWithContent 验证手工文档任务
// 进入 Worker 后：以 documents.content 为正文执行加工流水线（分块 + 索引），
// 完成后文档状态切换为 succeeded 并激活新索引版本。
func TestProcessImportTaskManualDocumentRunsPipelineWithContent(t *testing.T) {
	// ProcessImportTask 成功路径写日志，测试内初始化日志避免 nil panic。
	if err := logger.Init(&config.LogConfig{Level: "error", MaxSize: 1, MaxBackups: 1, MaxAge: 1}); err != nil {
		t.Fatalf("初始化测试日志失败: %v", err)
	}
	content := "# 手工标题\n\n这是手工文档正文，用于验证索引流程。"
	doc := &entity.Document{
		UserID: "u1", KnowledgeBaseID: "kb1",
		Title: "手工文档", Content: &content, ContentFormat: "markdown",
		SourceType: string(contracts.DocumentSourceManual),
		ProcessingStatus: string(contracts.ProcessingPending),
		ContentVersion:   1, ChunkVersion: 1,
	}
	doc.ID = "doc-1"
	fileName := "手工文档.markdown"
	task := &entity.ImportTask{
		UserID: "u1", KnowledgeBaseID: "kb1",
		SourceType: string(contracts.DocumentSourceManual), FileName: &fileName,
		DuplicatePolicy: "create_new", Status: string(contracts.TaskStatusRunning), DocumentID: strPtr("doc-1"),
	}
	task.ID = "task-1"
	tasks := &stubTaskRepo{task: task}
	docs := &stubDocRepo{doc: doc}
	chunks := &stubChunkRepo{}
	processor := &stubProcessor{}
	svc := &documentProcessService{tasks: tasks, docs: docs, chunks: chunks, processor: processor}

	if err := svc.ProcessImportTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("ProcessImportTask 失败: %v", err)
	}

	// 流水线收到手工正文与解析文件名（按内容格式路由到 Markdown 解析器）。
	if processor.lastInput.Content != content {
		t.Errorf("流水线未收到手工正文：%q", processor.lastInput.Content)
	}
	if processor.lastInput.FileName != fileName {
		t.Errorf("流水线 FileName = %q, want %q", processor.lastInput.FileName, fileName)
	}
	if processor.lastInput.ObjectKey != "" || processor.lastInput.SourceURL != "" {
		t.Error("手工文档不应访问 MinIO 对象或 URL")
	}
	if processor.lastInput.DocMeta.DocumentID != "doc-1" || processor.lastInput.DocMeta.IndexVersion != 1 {
		t.Errorf("DocMeta 错误: %+v", processor.lastInput.DocMeta)
	}

	// 成功路径：切换处理状态并激活索引版本 1，任务标记完成。
	if len(docs.updates) < 2 {
		t.Fatalf("文档状态更新次数异常: %d", len(docs.updates))
	}
	last := docs.updates[len(docs.updates)-1]
	if last["processing_status"] != string(contracts.ProcessingSucceeded) {
		t.Errorf("processing_status = %v", last["processing_status"])
	}
	if last["active_index_version"] != 1 {
		t.Errorf("active_index_version = %v", last["active_index_version"])
	}
	if len(chunks.deleted) != 1 || chunks.deleted[0] != 1 {
		t.Errorf("应先清理同版本残留 Chunk: %v", chunks.deleted)
	}
	if !tasks.completed {
		t.Error("任务应标记为 succeeded")
	}
}

// TestManualTaskFileNameByContentFormat 验证手工文档文件名按正文格式追加扩展名。
func TestManualTaskFileNameByContentFormat(t *testing.T) {
	if got := manualTaskFileName(&entity.Document{Title: "笔记", ContentFormat: "markdown"}); got != "笔记.markdown" {
		t.Errorf("markdown 格式 = %q", got)
	}
	if got := manualTaskFileName(&entity.Document{Title: "笔记", ContentFormat: "txt"}); got != "笔记.txt" {
		t.Errorf("txt 格式 = %q", got)
	}
	if got := manualTaskFileName(&entity.Document{Title: "  笔记  ", ContentFormat: "markdown"}); got != "笔记.markdown" {
		t.Errorf("标题去空白 = %q", got)
	}
}
