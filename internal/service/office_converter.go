package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// OfficeConverter 使用 LibreOffice 将 Office 文档（PPTX/DOCX/XLSX）转换为 PDF，
// 供在线预览使用。soffice 转换需串行执行（LibreOffice 单实例限制）。
type OfficeConverter struct {
	soffice string
	timeout time.Duration
	mu      sync.Mutex
}

// NewOfficeConverter 定位 soffice 可执行文件；找不到时返回错误。
func NewOfficeConverter() (*OfficeConverter, error) {
	path, err := findSoffice()
	if err != nil {
		return nil, err
	}
	return &OfficeConverter{soffice: path, timeout: 5 * time.Minute}, nil
}

// Available 报告转换能力是否可用。
func (c *OfficeConverter) Available() bool { return c != nil && c.soffice != "" }

// ConvertToPDF 将 srcPath 转换为 PDF，输出到 outDir，返回生成的 PDF 路径。
func (c *OfficeConverter) ConvertToPDF(ctx context.Context, srcPath, outDir string) (string, error) {
	if !c.Available() {
		return "", errors.New("LibreOffice 不可用，无法转换 Office 文档")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	convCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// --norestore 避免恢复上次会话；outdir 指定输出目录；参数化路径无 shell 注入面。
	cmd := exec.CommandContext(convCtx, c.soffice,
		"--headless", "--norestore", "--convert-to", "pdf", "--outdir", outDir, srcPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if convCtx.Err() != nil {
			return "", fmt.Errorf("Office 转 PDF 超时（%s）", c.timeout)
		}
		return "", fmt.Errorf("LibreOffice 转换失败: %v: %s", err, truncateOutput(output))
	}

	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	pdfPath := filepath.Join(outDir, base+".pdf")
	info, statErr := os.Stat(pdfPath)
	if statErr != nil {
		return "", fmt.Errorf("LibreOffice 未生成 PDF 输出: %v", statErr)
	}
	if info.Size() < 16 {
		return "", fmt.Errorf("LibreOffice 输出的 PDF 过小（%d 字节），可能转换不完整", info.Size())
	}
	// 校验 PDF 结尾标记：防 soffice 被终止/异常时输出不完整文件。
	if err := verifyPDFEnding(pdfPath); err != nil {
		return "", err
	}
	return pdfPath, nil
}

// verifyPDFEnding 读取 PDF 文件尾部并确认包含 %%EOF 结束标记。
func verifyPDFEnding(pdfPath string) error {
	file, err := os.Open(pdfPath)
	if err != nil {
		return fmt.Errorf("打开转换 PDF 失败: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("读取转换 PDF 信息失败: %w", err)
	}
	const tailSize = 2048
	offset := info.Size() - tailSize
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("定位 PDF 尾部失败: %w", err)
	}
	tail, err := io.ReadAll(io.LimitReader(file, tailSize))
	if err != nil {
		return fmt.Errorf("读取 PDF 尾部失败: %w", err)
	}
	if !strings.Contains(string(tail), "%%EOF") {
		return fmt.Errorf("转换 PDF 缺少 %%EOF 结束标记，输出不完整")
	}
	return nil
}

// findSoffice 在 PATH 与常见安装路径中查找 soffice。
func findSoffice() (string, error) {
	if path, err := exec.LookPath("soffice"); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("soffice.exe"); err == nil {
			return path, nil
		}
		candidates := []string{
			`C:\Program Files\LibreOffice\program\soffice.exe`,
			`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	} else {
		for _, candidate := range []string{
			"/usr/bin/soffice",
			"/usr/local/bin/soffice",
			"/Applications/LibreOffice.app/Contents/MacOS/soffice",
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", errors.New("未找到 LibreOffice（soffice），请安装后重试")
}

func truncateOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	if text == "" {
		return "无输出"
	}
	return text
}
