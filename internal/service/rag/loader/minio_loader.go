// Package loader 提供从 MinIO 读取文档并解析为 Eino schema.Document 的 Loader 实现。
package loader

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

// MinIOObjectReader 是 MinIOLoader 依赖的最小对象读取接口。
type MinIOObjectReader interface {
	// OpenObject 返回可关闭的对象读取流。
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	// Bucket 返回当前存储桶名称。
	Bucket() string
}

// MinIOLoader 从 MinIO 读取文档对象，按扩展名选择 Eino Parser 解析为 schema.Document。
// 流式读取，不将完整文件读入内存；对象 key 由上游注入，不在 Loader 内拼接。
type MinIOLoader struct {
	store  MinIOObjectReader
	parser parser.Parser
}

// NewMinIOLoader 构造 MinIO Loader。parser 为空时使用 ExtParser（fallback TextParser）。
func NewMinIOLoader(ctx context.Context, store MinIOObjectReader, p parser.Parser) (*MinIOLoader, error) {
	if store == nil {
		return nil, fmt.Errorf("MinIO 对象读取器不能为空")
	}
	if p == nil {
		extParser, err := parser.NewExtParser(ctx, &parser.ExtParserConfig{
			FallbackParser: parser.TextParser{},
		})
		if err != nil {
			return nil, fmt.Errorf("创建默认解析器失败: %w", err)
		}
		p = extParser
	}
	return &MinIOLoader{store: store, parser: p}, nil
}

// Load 实现 Eino document.Loader：打开 MinIO 对象并交给 Parser 解析。
// Source.URI 约定为 MinIO 对象 key。
func (l *MinIOLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error) {
	ctx = callbacks.EnsureRunInfo(ctx, l.GetType(), components.ComponentOfLoader)
	ctx = callbacks.OnStart(ctx, &document.LoaderCallbackInput{Source: src})
	var err error
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	// 去除存储桶前缀得到对象 key；URI 完整路径由上游注入，Loader 不自行拼接。
	objectKey := strings.TrimPrefix(src.URI, l.store.Bucket()+"/")
	reader, err := l.store.OpenObject(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("打开 MinIO 对象 %q 失败: %w", src.URI, err)
	}
	// 延迟关闭读取流；仅在没有主错误时才上报关闭失败，避免掩盖解析结果。
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("关闭 MinIO 对象读取流失败: %w", closeErr)
		}
	}()

	common := document.GetLoaderCommonOptions(&document.LoaderOptions{}, opts...)
	fileName := path.Base(objectKey)
	// 注入来源元数据作为基础信息，业务元数据由 pipeline 的 load 节点补充。
	meta := map[string]any{
		einoadapter.MetaDocumentID:     src.URI,
		einoadapter.MetaSourceLocation: map[string]any{"object_key": objectKey, "file_name": fileName},
	}
	docs, err := l.parser.Parse(ctx, reader, append([]parser.Option{parser.WithURI(objectKey), parser.WithExtraMeta(meta)}, common.ParserOptions...)...)
	if err != nil {
		return nil, fmt.Errorf("解析文档 %q 失败: %w", src.URI, err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("解析文档 %q 未产生任何内容", src.URI)
	}
	_ = callbacks.OnEnd(ctx, &document.LoaderCallbackOutput{Source: src, Docs: docs})
	return docs, nil
}

// GetType 返回组件类型名。
func (l *MinIOLoader) GetType() string { return "MinIOLoader" }

// IsCallbacksEnabled 启用 Eino Callbacks。
func (l *MinIOLoader) IsCallbacksEnabled() bool { return true }
