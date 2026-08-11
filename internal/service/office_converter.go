package service

import (
	"context"
	"errors"
	"fmt"
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
	if _, statErr := os.Stat(pdfPath); statErr != nil {
		return "", fmt.Errorf("LibreOffice 未生成 PDF 输出: %v", statErr)
	}
	return pdfPath, nil
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
