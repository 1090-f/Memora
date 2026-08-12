package renderer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/service/preview"
)

type LibreOffice struct {
	soffice   string
	timeout   time.Duration
	info      preview.RendererInfo
	semaphore chan struct{}
}

func NewLibreOffice(enabled bool, maxConcurrency int, timeout time.Duration) (*LibreOffice, error) {
	if !enabled {
		return &LibreOffice{info: preview.RendererInfo{Enabled: false, Name: "libreoffice", Version: "disabled", StrategyVersion: "office-pdf-v1"}}, nil
	}
	path, err := findSoffice()
	if err != nil {
		return nil, err
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	version := sofficeVersion(path)
	return &LibreOffice{
		soffice: path, timeout: timeout, semaphore: make(chan struct{}, maxConcurrency),
		info: preview.RendererInfo{Enabled: true, Name: "libreoffice", Version: version, StrategyVersion: "office-pdf-v1"},
	}, nil
}

func (r *LibreOffice) Info() preview.RendererInfo { return r.info }

func (r *LibreOffice) Render(ctx context.Context, sourceName string, source io.Reader) (*preview.RenderResult, error) {
	if r == nil || !r.info.Enabled || r.soffice == "" {
		return nil, fmt.Errorf("LibreOffice 不可用")
	}
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	tempDir, err := os.MkdirTemp("", "memora-preview-office-*")
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tempDir)
		}
	}()

	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(sourceName), "\\", "/"))
	if base == "." || base == "" {
		base = "source.docx"
	}
	sourcePath := filepath.Join(tempDir, base)
	file, err := os.Create(sourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}

	profileDir := filepath.Join(tempDir, "profile")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, err
	}
	profileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(profileDir)}).String()
	convCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(convCtx, r.soffice,
		"--headless", "--norestore", "--nodefault", "--nolockcheck",
		"-env:UserInstallation="+profileURL,
		"--convert-to", "pdf", "--outdir", tempDir, sourcePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if convCtx.Err() != nil {
			return nil, fmt.Errorf("%w: %s", preview.ErrRenderTimeout, r.timeout)
		}
		return nil, fmt.Errorf("LibreOffice 转换失败: %v: %s", err, truncateOutput(output))
	}
	pdfPath := filepath.Join(tempDir, strings.TrimSuffix(base, filepath.Ext(base))+".pdf")
	if err := verifyPDF(pdfPath); err != nil {
		return nil, err
	}
	pdf, err := os.Open(pdfPath)
	if err != nil {
		return nil, err
	}
	info, err := pdf.Stat()
	if err != nil {
		_ = pdf.Close()
		return nil, err
	}
	cleanup = false
	return &preview.RenderResult{
		Name: "rendered.pdf", MediaType: "application/pdf", Size: info.Size(),
		Reader: &cleanupReader{ReadCloser: pdf, dir: tempDir},
	}, nil
}

type cleanupReader struct {
	io.ReadCloser
	dir string
}

func (r *cleanupReader) Close() error {
	err := r.ReadCloser.Close()
	return errors.Join(err, os.RemoveAll(r.dir))
}

func verifyPDF(pdfPath string) error {
	file, err := os.Open(pdfPath)
	if err != nil {
		return fmt.Errorf("LibreOffice 未生成 PDF: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < 1024 {
		return fmt.Errorf("LibreOffice 输出 PDF 过小（%d 字节）", info.Size())
	}
	head := make([]byte, 5)
	if _, err := io.ReadFull(file, head); err != nil || string(head) != "%PDF-" {
		return fmt.Errorf("LibreOffice 输出不是有效 PDF")
	}
	offset := info.Size() - 2048
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	tail, err := io.ReadAll(io.LimitReader(file, 2048))
	if err != nil {
		return err
	}
	if !strings.Contains(string(tail), "%%EOF") {
		return fmt.Errorf("LibreOffice 输出 PDF 不完整")
	}
	return nil
}

func sofficeVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	value := strings.TrimSpace(string(output))
	if len(value) > 128 {
		value = value[:128]
	}
	if value == "" {
		return "unknown"
	}
	return value
}

func findSoffice() (string, error) {
	for _, name := range []string{"soffice", "soffice.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	candidates := []string{"/usr/bin/soffice", "/usr/local/bin/soffice", "/Applications/LibreOffice.app/Contents/MacOS/soffice"}
	if runtime.GOOS == "windows" {
		candidates = []string{`C:\Program Files\LibreOffice\program\soffice.exe`, `C:\Program Files (x86)\LibreOffice\program\soffice.exe`}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 LibreOffice（soffice）")
}

func truncateOutput(output []byte) string {
	value := strings.TrimSpace(string(output))
	if len(value) > 500 {
		return value[:500] + "..."
	}
	if value == "" {
		return "无输出"
	}
	return value
}
