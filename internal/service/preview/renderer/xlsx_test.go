package renderer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/1090-f/Memora/internal/service/preview"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/klauspost/compress/zstd"
)

func TestXLSXRenderPreservesDimensionsSharedStringsAndMerges(t *testing.T) {
	source := makeWorkbook(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="数据" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst><si><t>标题</t></si><si><r><t>复合</t></r><r><t>文本</t></r></si></sst>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><dimension ref="A1:C4"/><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row><row r="4"><c r="C4" t="inlineStr"><is><t>尾行</t></is></c></row></sheetData><mergeCells><mergeCell ref="A1:B2"/></mergeCells></worksheet>`,
	})
	renderer := NewXLSX(config.XLSXPreviewConfig{Enabled: true, MaxSheets: 5, MaxRowsPerSheet: 100, MaxColumnsPerSheet: 20, MaxCells: 1000, MaxUncompressedBytes: 1 << 20})

	result, err := renderer.Render(context.Background(), "report.xlsx", bytes.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	defer result.Reader.Close()
	compressed, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	body, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		t.Fatal(err)
	}
	var workbook preview.Workbook
	if err := json.Unmarshal(body, &workbook); err != nil {
		t.Fatal(err)
	}
	if len(workbook.Sheets) != 1 {
		t.Fatalf("sheet count = %d, want 1", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if sheet.Name != "数据" || sheet.RowCount != 4 || sheet.ColumnCount != 3 {
		t.Fatalf("sheet summary = %#v", sheet)
	}
	if len(sheet.Rows) != 2 || sheet.Rows[0].Cells[0].Value != "标题" || sheet.Rows[0].Cells[1].Value != "复合文本" || sheet.Rows[1].Cells[0].Value != "尾行" {
		t.Fatalf("unexpected rows: %#v", sheet.Rows)
	}
	if len(sheet.MergedCells) != 1 || sheet.MergedCells[0].RowSpan != 2 || sheet.MergedCells[0].ColumnSpan != 2 {
		t.Fatalf("unexpected merged cells: %#v", sheet.MergedCells)
	}
}

func TestXLSXRenderRejectsDimensionsOverLimit(t *testing.T) {
	source := makeWorkbook(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="urn:r"><sheets><sheet name="large" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><dimension ref="A1:Z100"/><sheetData/></worksheet>`,
	})
	renderer := NewXLSX(config.XLSXPreviewConfig{Enabled: true, MaxSheets: 1, MaxRowsPerSheet: 10, MaxColumnsPerSheet: 10, MaxCells: 100, MaxUncompressedBytes: 1 << 20})

	_, err := renderer.Render(context.Background(), "large.xlsx", bytes.NewReader(source))
	if !errors.Is(err, preview.ErrTableTooLarge) {
		t.Fatalf("Render() error = %v, want ErrTableTooLarge", err)
	}
}

func makeWorkbook(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
