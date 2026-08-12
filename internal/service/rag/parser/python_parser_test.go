package parser

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakePythonService 模拟 Python document-parser 服务。
type fakePythonService struct {
	t       *testing.T
	handler func(t *testing.T, w http.ResponseWriter, r *http.Request)
}

func (f *fakePythonService) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.handler(f.t, w, r)
	}))
}

// testDocument 构造最小合法 ParsedDocument（含一张表与一张图）。
func testDocument() *ParsedDocument {
	return &ParsedDocument{
		SchemaVersion: SchemaVersion,
		Parser:        ParserInfo{Name: ParserNameDocling, Version: "2.118.1", AdapterVersion: AdapterVersion},
		Source:        SourceInfo{FileName: "a.pdf", Format: "pdf", SHA256: strings.Repeat("ab", 32), Size: 10},
		Document:      DocumentInfo{Title: "t", PageCount: 1},
		Blocks: []Block{
			{ID: "block-000001", Type: BlockTypeHeading, Text: "第一章", HeadingPath: []string{"第一章"}},
			{ID: "block-000002", Type: BlockTypeTable, Text: "| a | b |", TableRef: "table-000001"},
			{ID: "block-000003", Type: BlockTypePicture, AssetRefs: []string{"asset-000001"}},
		},
		Tables: []Table{{ID: "table-000001", RowCount: 1, ColumnCount: 2, Headers: [][]string{{"a", "b"}}}},
		Assets: []Asset{{
			ID: "asset-000001", Kind: "picture", MIMEType: "image/png",
			DataBase64: "iVBORw0KGgo=",
			Width:      1, Height: 1, Page: 1,
		}},
	}
}

func TestPythonParserSendsMultipartAndParsesResponse(t *testing.T) {
	service := &fakePythonService{t: t}
	service.handler = func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/parse" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("multipart 解析失败: %v", err)
		}
		if got := r.FormValue("options"); !strings.Contains(got, `"schema_version":"1.0"`) {
			t.Errorf("options 字段 = %q", got)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("file 字段缺失: %v", err)
		}
		defer file.Close()
		buf, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("读取上传内容失败: %v", err)
		}
		if string(buf) != "pdf-bytes" {
			t.Errorf("上传内容 = %q", string(buf))
		}
		// 回显固定 ParsedDocument。
		doc := testDocument()
		doc.Source.FileName = "a.pdf"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}
	srv := service.server()
	defer srv.Close()

	client, err := NewPythonDocumentParser(PythonParserConfig{BaseURL: srv.URL, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	input := ParseInput{
		FileName: "a.pdf",
		Content:  strings.NewReader("pdf-bytes"),
		Size:     int64(len("pdf-bytes")),
		Options:  DefaultParseOptions(),
	}
	doc, err := client.Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.Parser.Name != ParserNameDocling {
		t.Errorf("parser 名 = %q", doc.Parser.Name)
	}
	if len(doc.Blocks) != 3 {
		t.Errorf("blocks = %d", len(doc.Blocks))
	}
}

func TestPythonParserMapsErrors(t *testing.T) {
	service := &fakePythonService{t: t}
	service.handler = func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"invalid_format: 伪造格式"}`))
	}
	srv := service.server()
	defer srv.Close()

	client, err := NewPythonDocumentParser(PythonParserConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	_, err = client.Parse(context.Background(), ParseInput{FileName: "x.pdf", Content: strings.NewReader("data"), Size: 4, Options: DefaultParseOptions()})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("期望 ParseError，实际 %v", err)
	}
	if parseErr.Kind != ParseErrorRemoteFailure {
		t.Errorf("错误分类 = %v，期望 RemoteFailure", parseErr.Kind)
	}
}

func TestPythonParserRejectsUnreachable(t *testing.T) {
	client, err := NewPythonDocumentParser(PythonParserConfig{BaseURL: "http://127.0.0.1:1", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	_, err = client.Parse(context.Background(), ParseInput{FileName: "x.pdf", Content: strings.NewReader("data"), Size: 4, Options: DefaultParseOptions()})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("期望 ParseError，实际 %v", err)
	}
	if parseErr.Kind != ParseErrorRemoteFailure {
		t.Errorf("错误分类 = %v，期望 RemoteFailure", parseErr.Kind)
	}
}

func TestPythonParserRejectsInvalidJSON(t *testing.T) {
	service := &fakePythonService{t: t}
	service.handler = func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{invalid`))
	}
	srv := service.server()
	defer srv.Close()

	client, err := NewPythonDocumentParser(PythonParserConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	_, err = client.Parse(context.Background(), ParseInput{FileName: "x.pdf", Content: strings.NewReader("data"), Size: 4, Options: DefaultParseOptions()})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("期望 ParseError，实际 %v", err)
	}
	if parseErr.Kind != ParseErrorInvalidResponse {
		t.Errorf("错误分类 = %v，期望 InvalidResponse", parseErr.Kind)
	}
}
