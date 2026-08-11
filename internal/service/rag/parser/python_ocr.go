package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// PythonOcrClient 调用常驻 Python document-parser 的 /v1/ocr 识别单张图片文字。
// 只发送图片字节与语言代码；OCR 失败时返回可降级错误（不阻断文档加工）。
type PythonOcrClient struct {
	baseURL string
	timeout time.Duration
	client  *http.Client
}

// NewPythonOcrClient 构造图片 OCR 客户端。
func NewPythonOcrClient(baseURL string, timeout time.Duration) *PythonOcrClient {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &PythonOcrClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

// OcrResult 是 /v1/ocr 的响应。
type OcrResult struct {
	Lines     []string `json:"lines"`
	Languages []string `json:"languages"`
	Engine    string   `json:"engine"`
}

// OcrImage 识别单张图片文字；baseURL 为空时返回 ErrOCRNotConfigured。
func (c *PythonOcrClient) OcrImage(ctx context.Context, image []byte, languages []string) (*OcrResult, error) {
	if c.baseURL == "" {
		return nil, ErrOCRNotConfigured
	}
	if len(image) == 0 {
		return &OcrResult{}, nil
	}
	langJSON, err := json.Marshal(languages)
	if err != nil {
		return nil, fmt.Errorf("序列化 OCR 语言失败: %w", err)
	}

	var buf bytes.Buffer
	multipartWriter := multipart.NewWriter(&buf)
	if err := multipartWriter.WriteField("languages", string(langJSON)); err != nil {
		return nil, fmt.Errorf("写入 OCR 语言字段失败: %w", err)
	}
	part, err := multipartWriter.CreateFormFile("file", "image.png")
	if err != nil {
		return nil, fmt.Errorf("创建 OCR 图片字段失败: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return nil, fmt.Errorf("写入 OCR 图片失败: %w", err)
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("关闭 OCR 表单失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/ocr", &buf)
	if err != nil {
		return nil, fmt.Errorf("构造 OCR 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 OCR 服务失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("OCR 服务返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result OcrResult
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16*1024*1024)).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 OCR 响应失败: %w", err)
	}
	return &result, nil
}

// ErrOCRNotConfigured 表示 Python 服务未配置（OCR 跳过）。
var ErrOCRNotConfigured = fmt.Errorf("OCR 服务未配置（document_parser.base_url 为空）")
