package parser

import (
	"context"
	"errors"
	"path"
	"strings"
)

// ParserRouter 按扩展名路由到具体 Parser。
// TXT/Markdown → TextParser；PDF/DOCX/XLSX/PPTX → PythonDocumentParser；
// 其余格式直接报错，禁止静默回退到其它解析器。
type ParserRouter struct {
	textParser   Parser
	pythonParser Parser
}

// NewParserRouter 构造路由。
func NewParserRouter(textParser, pythonParser Parser) *ParserRouter {
	return &ParserRouter{textParser: textParser, pythonParser: pythonParser}
}

// Route 返回适合该文件名的 Parser。
func (r *ParserRouter) Route(fileName string) (Parser, error) {
	ext := strings.ToLower(path.Ext(fileName))
	switch ext {
	case ".txt":
		return r.textParser, nil
	case ".md", ".markdown":
		return r.textParser, nil
	case ".pdf", ".docx", ".xlsx", ".pptx":
		return r.pythonParser, nil
	default:
		return nil, ParseErrorf(ParseErrorUnsupportedFormat, "不支持的文件格式 %q（仅支持 txt/md/pdf/docx/xlsx/pptx）", fileName)
	}
}

// Parse 路由解析：统一错误分类出口。
func (r *ParserRouter) Parse(ctx context.Context, input ParseInput) (*ParsedDocument, error) {
	p, err := r.Route(input.FileName)
	if err != nil {
		return nil, err
	}
	doc, err := p.Parse(ctx, input)
	if err != nil {
		var parseErr *ParseError
		if errors.As(err, &parseErr) {
			return nil, parseErr
		}
		return nil, NewParseError(ParseErrorInternal, err)
	}
	return doc, nil
}
