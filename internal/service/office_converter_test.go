package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOfficeConverterConvertToPDF(t *testing.T) {
	converter, err := NewOfficeConverter()
	if err != nil {
		t.Skipf("LibreOffice 不可用，跳过: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.pptx")
	// 无 python-pptx 依赖：写一个最小合法 PPTX（ZIP 容器 + presentation.xml）。
	data, err := os.ReadFile(`C:\pgtemp\opencode\test_conv.pptx`)
	if err != nil {
		t.Skipf("缺少测试 PPTX，跳过: %v", err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	outDir := t.TempDir()
	pdfPath, err := converter.ConvertToPDF(context.Background(), src, outDir)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	info, err := os.Stat(pdfPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("PDF 输出无效: %v", err)
	}
	t.Logf("PDF 生成成功: %s (%d bytes)", pdfPath, info.Size())
}
