package parser

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// PythonParserConfig 定义 Python document-parser 客户端配置。
type PythonParserConfig struct {
	// BaseURL 是 Python 服务地址，如 http://localhost:5001。
	BaseURL string
	// Timeout 是单次解析请求超时（必须小于 Worker 总超时）。
	Timeout time.Duration
	// MaxResponseBytes 是响应体大小上限。
	MaxResponseBytes int64
}

// PythonDocumentParser 调用常驻 Python document-parser 服务解析 PDF/DOCX。
// 只发送文件字节与解析选项；不发送任何 Chunk/Embedding 参数。
type PythonDocumentParser struct {
	cfg    PythonParserConfig
	client *http.Client
}

// NewPythonDocumentParser 构造 Python 解析客户端。
// BaseURL 为空时允许构造，但 Parse 返回错误（保证 Worker 在未配置 parser 时仍可启动）。
func NewPythonDocumentParser(cfg PythonParserConfig) (*PythonDocumentParser, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Minute
	}
	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = 128 * 1024 * 1024
	}
	return &PythonDocumentParser{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}, nil
}

// Parse 实现 Parser：流式 multipart 上传，单遍读取。
func (p *PythonDocumentParser) Parse(ctx context.Context, input ParseInput) (*ParsedDocument, error) {
	if p.cfg.BaseURL == "" {
		return nil, ParseErrorf(ParseErrorRemoteFailure, "Python 解析服务未配置（document_parser.base_url 为空）")
	}
	optionsJSON, err := json.Marshal(input.Options)
	if err != nil {
		return nil, ParseErrorf(ParseErrorInternal, "序列化解析选项失败: %v", err)
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)

	// 后台线程：写入 multipart 表单并关闭写入端；出错时传递错误。
	errCh := make(chan error, 1)
	go func() {
		defer func() {
			_ = pipeWriter.CloseWithError(nil)
			close(errCh)
		}()
		if err := multipartWriter.WriteField("options", string(optionsJSON)); err != nil {
			errCh <- err
			return
		}
		part, err := multipartWriter.CreateFormFile("file", input.FileName)
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, input.Content); err != nil {
			errCh <- err
			return
		}
		if err := multipartWriter.Close(); err != nil {
			errCh <- err
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/v1/parse", pipeReader)
	if err != nil {
		return nil, ParseErrorf(ParseErrorInternal, "构造解析请求失败: %v", err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, ParseErrorf(ParseErrorRemoteFailure, "调用 Python 解析服务失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := <-errCh; err != nil {
		return nil, ParseErrorf(ParseErrorInternal, "流式上传解析输入失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, ParseErrorf(ParseErrorRemoteFailure,
			"Python 解析服务返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, p.cfg.MaxResponseBytes+1))
	if err != nil {
		return nil, ParseErrorf(ParseErrorRemoteFailure, "读取解析响应失败: %v", err)
	}
	if int64(len(body)) > p.cfg.MaxResponseBytes {
		return nil, ParseErrorf(ParseErrorInvalidResponse, "解析响应超过大小限制 %d 字节", p.cfg.MaxResponseBytes)
	}

	var doc ParsedDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, ParseErrorf(ParseErrorInvalidResponse, "解析响应 JSON 失败: %v", err)
	}
	return &doc, nil
}
