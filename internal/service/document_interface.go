package service

import (
	"context"
	"io"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

// UploadFileInput 描述单个待上传文件。
type UploadFileInput struct {
	FileName string
	Size     int64
	Reader   io.Reader
}

// DocumentService 定义文档管理业务逻辑的接口。
type DocumentService interface {
	// CreateManual 手工创建只读知识文档。
	CreateManual(ctx context.Context, userID, kbID string, req *request.CreateDocumentRequest) (*dto.DocumentResponse, error)
	// List 分页查询知识库文档列表。
	List(ctx context.Context, userID, kbID string, page, pageSize int, filter request.DocumentListFilter) (*dto.DocumentList, error)
	// Get 查询文档详情。
	Get(ctx context.Context, userID, documentID string) (*dto.DocumentResponse, error)
	// Delete 软删除文档。
	Delete(ctx context.Context, userID, documentID string) error
	// UploadFiles 文件导入：先创建 import_tasks，再流式上传 MinIO，最后更新任务对象信息。
	UploadFiles(ctx context.Context, userID, kbID string, directoryID *string, duplicatePolicy string, files []UploadFileInput) (*dto.UploadFilesResponse, error)
	// ImportURL 创建 URL 导入任务，网页抓取和解析由 Worker 异步执行。
	ImportURL(ctx context.Context, userID, kbID string, req *request.ImportURLRequest) (*dto.UploadTaskItem, error)
}
