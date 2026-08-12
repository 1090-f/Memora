package preview

import (
	"testing"

	"github.com/1090-f/Memora/internal/model/entity"
)

func TestBuildTablePageKeepsSparseRowsAndPagination(t *testing.T) {
	doc := &entity.Document{ContentVersion: 2}
	doc.ID = "d1"
	workbook := &Workbook{Sheets: []WorkbookSheet{{
		Index: 0, Name: "Sheet1", RowCount: 25, ColumnCount: 3,
		Rows:        []TableRow{{Row: 0, Cells: []TableCell{{Column: 0, Value: "A1"}}}, {Row: 24, Cells: []TableCell{{Column: 2, Value: "C25"}}}},
		MergedCells: []MergedCell{{StartRow: 0, StartColumn: 0, RowSpan: 2, ColumnSpan: 2}},
	}}}

	page, err := buildTablePage(doc, workbook, TableQuery{SheetIndex: 0, RowOffset: 0, RowLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 20 || page.Rows[1].Row != 1 || len(page.Rows[1].Cells) != 0 {
		t.Fatalf("sparse rows were not retained: %#v", page.Rows)
	}
	if page.NextRowOffset == nil || *page.NextRowOffset != 20 {
		t.Fatalf("next row offset = %v, want 20", page.NextRowOffset)
	}
	if len(page.MergedCells) != 1 || page.Sheets[0].RowCount != 25 {
		t.Fatalf("metadata missing: %#v", page)
	}
}
