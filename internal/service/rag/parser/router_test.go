package parser

import (
	"context"
	"testing"
)

// failingPythonParser 记录是否被调用，用于路由断言。
type failingPythonParser struct {
	called bool
}

func (p *failingPythonParser) Parse(context.Context, ParseInput) (*ParsedDocument, error) {
	p.called = true
	return nil, nil
}

// TestParserRouterRoutesImageAndOfficeExtensions 验证图片与 Office 扩展名路由到
// Python 解析器（与 docling 侧 source.format 修复配套的格式判断）。
func TestParserRouterRoutesImageAndOfficeExtensions(t *testing.T) {
	python := &failingPythonParser{}
	router := NewParserRouter(&noopParser{}, &noopParser{}, python)

	for _, fileName := range []string{
		"a.pdf", "a.docx", "a.xlsx", "a.pptx",
		"a.png", "a.jpg", "a.jpeg", "a.bmp", "a.tiff", "a.tif", "a.gif", "a.webp",
	} {
		p, err := router.Route(fileName)
		if err != nil {
			t.Errorf("Route(%q) 不应报错: %v", fileName, err)
			continue
		}
		if _, err := p.Parse(context.Background(), ParseInput{}); err != nil {
			t.Errorf("Route(%q) 应路由到 Python 解析器", fileName)
		}
	}
	if !python.called {
		t.Error("图片/Office 文件未路由到 Python 解析器")
	}
}

// noopParser 是其它分支占位。
type noopParser struct{}

func (p *noopParser) Parse(context.Context, ParseInput) (*ParsedDocument, error) {
	return nil, nil
}
