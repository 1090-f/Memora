package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// ObjectStore 是 Parser 与 ArtifactStore 依赖的最小对象存储接口。
// 由 pkg/objectstore.Client 实现，测试可注入内存实现。
type ObjectStore interface {
	// OpenObject 返回可关闭的对象读取流。
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	// PutObject 流式上传对象；size 为 -1 时使用流式（chunked）上传。
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	// StatObject 查询对象元信息；不存在时返回 ErrObjectNotFound。
	StatObject(ctx context.Context, objectKey string) (*ObjectInfo, error)
	// RemoveObject 删除对象；对象不存在时不视为错误。
	RemoveObject(ctx context.Context, objectKey string) error
	// Bucket 返回当前存储桶名称。
	Bucket() string
}

// ObjectInfo 描述 MinIO 对象的元信息。
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
}

// ErrObjectNotFound 表示对象不存在。
var ErrObjectNotFound = fmt.Errorf("对象不存在")

// ParseInput 是 Parser.Parse 的输入。
type ParseInput struct {
	// FileName 是原始文件名（用于格式路由与协议 source.file_name）。
	FileName string
	// Content 是原始文件内容流；调用方负责关闭（text 解析器会完整读取）。
	Content io.Reader
	// Size 是内容字节数；小于 0 表示未知。
	Size int64
	// Options 是解析选项（进入 parse_config_hash）。
	Options ParseOptions
}

// Parser 将原始文件解析为 ParsedDocument。实现不得输出 Chunk。
type Parser interface {
	Parse(ctx context.Context, input ParseInput) (*ParsedDocument, error)
}

// ParseErrorKind 是解析错误的稳定分类，供 Pipeline 决策，禁止静默回退。
type ParseErrorKind int

const (
	// ParseErrorUnsupportedFormat 是不支持的/伪造的文件格式。
	ParseErrorUnsupportedFormat ParseErrorKind = iota
	// ParseErrorInvalidInput 是空文件、超限文件等输入错误。
	ParseErrorInvalidInput
	// ParseErrorRemoteFailure 是 Python 服务不可达或返回失败。
	ParseErrorRemoteFailure
	// ParseErrorInvalidResponse 是响应违反协议（schema/哈希/引用校验失败）。
	ParseErrorInvalidResponse
	// ParseErrorInternal 是解析器内部错误。
	ParseErrorInternal
)

// ParseError 是带稳定分类的解析错误。
type ParseError struct {
	Kind ParseErrorKind
	Err  error
}

func (e *ParseError) Error() string {
	if e.Err == nil {
		return "解析失败"
	}
	return e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

// NewParseError 构造带分类的解析错误。
func NewParseError(kind ParseErrorKind, err error) *ParseError {
	return &ParseError{Kind: kind, Err: err}
}

// ParseErrorf 构造带分类与格式化消息的解析错误。
func ParseErrorf(kind ParseErrorKind, format string, args ...any) *ParseError {
	return &ParseError{Kind: kind, Err: fmt.Errorf(format, args...)}
}

// HashReader 包装 io.Reader 并在读取时计算 sha256，支持单遍流式哈希。
type HashReader struct {
	reader io.Reader
	hasher hash.Hash
}

// NewHashReader 构造带 sha256 计数的包装读取器。
func NewHashReader(reader io.Reader) *HashReader {
	return &HashReader{reader: reader, hasher: sha256.New()}
}

// Read 实现 io.Reader。
func (h *HashReader) Read(p []byte) (int, error) {
	n, err := h.reader.Read(p)
	if n > 0 {
		_, _ = h.hasher.Write(p[:n])
	}
	return n, err
}

// Sum 返回已读取内容的 sha256 十六进制。
func (h *HashReader) Sum() string {
	return hex.EncodeToString(h.hasher.Sum(nil))
}
