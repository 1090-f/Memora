package renderer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/1090-f/Memora/internal/service/preview"
	"github.com/1090-f/Memora/pkg/config"
)

type XLSX struct {
	limits config.XLSXPreviewConfig
	info   preview.RendererInfo
}

func NewXLSX(cfg config.XLSXPreviewConfig) *XLSX {
	return &XLSX{limits: cfg, info: preview.RendererInfo{Enabled: cfg.Enabled, Name: "go-openxml", Version: "1.0", StrategyVersion: "xlsx-table-v1"}}
}

func (r *XLSX) Info() preview.RendererInfo { return r.info }

func (r *XLSX) Render(ctx context.Context, _ string, source io.Reader) (*preview.RenderResult, error) {
	if !r.info.Enabled {
		return nil, fmt.Errorf("XLSX 结构化预览未启用")
	}
	maxInput := r.limits.MaxUncompressedBytes
	if maxInput < 64<<20 {
		maxInput = 64 << 20
	}
	data, err := io.ReadAll(io.LimitReader(source, maxInput+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxInput {
		return nil, fmt.Errorf("%w: XLSX 文件过大", preview.ErrTableTooLarge)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("XLSX ZIP 结构无效: %w", err)
	}
	files := make(map[string]*zip.File, len(archive.File))
	var totalUncompressed uint64
	for _, file := range archive.File {
		totalUncompressed += file.UncompressedSize64
		if totalUncompressed > uint64(r.limits.MaxUncompressedBytes) {
			return nil, fmt.Errorf("%w: XLSX 解压内容过大", preview.ErrTableTooLarge)
		}
		files[path.Clean(file.Name)] = file
	}
	shared, err := readSharedStrings(files)
	if err != nil {
		return nil, err
	}
	sheets, err := readWorkbookSheets(files)
	if err != nil {
		return nil, err
	}
	if len(sheets) > r.limits.MaxSheets {
		return nil, fmt.Errorf("%w: Sheet 数 %d", preview.ErrTableTooLarge, len(sheets))
	}
	workbook := &preview.Workbook{Sheets: make([]preview.WorkbookSheet, 0, len(sheets))}
	totalCells := 0
	for index, ref := range sheets {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		file := files[path.Clean(ref.Target)]
		if file == nil {
			return nil, fmt.Errorf("XLSX Sheet %q 对象缺失", ref.Name)
		}
		sheet, cells, err := r.readSheet(file, index, ref.Name, shared)
		if err != nil {
			return nil, err
		}
		totalCells += cells
		if totalCells > r.limits.MaxCells {
			return nil, fmt.Errorf("%w: 单元格数超过 %d", preview.ErrTableTooLarge, r.limits.MaxCells)
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	compressed, err := preview.EncodeWorkbook(workbook)
	if err != nil {
		return nil, err
	}
	return &preview.RenderResult{Name: "sheet-data.json.zst", MediaType: "application/zstd", Size: int64(len(compressed)), Reader: io.NopCloser(bytes.NewReader(compressed))}, nil
}

type sheetRef struct{ Name, Target string }

func readWorkbookSheets(files map[string]*zip.File) ([]sheetRef, error) {
	relsBody, err := readZipFile(files["xl/_rels/workbook.xml.rels"], 4<<20)
	if err != nil {
		return nil, err
	}
	var rels struct {
		Relationships []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(relsBody, &rels); err != nil {
		return nil, err
	}
	byID := make(map[string]string, len(rels.Relationships))
	for _, rel := range rels.Relationships {
		target := strings.ReplaceAll(rel.Target, "\\", "/")
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Join("xl", target)
		}
		byID[rel.ID] = path.Clean(target)
	}
	body, err := readZipFile(files["xl/workbook.xml"], 4<<20)
	if err != nil {
		return nil, err
	}
	var workbook struct {
		Sheets []struct {
			Name  string `xml:"name,attr"`
			RelID string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(body, &workbook); err != nil {
		return nil, err
	}
	result := make([]sheetRef, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		if target := byID[sheet.RelID]; target != "" {
			result = append(result, sheetRef{Name: sheet.Name, Target: target})
		}
	}
	return result, nil
}

func readSharedStrings(files map[string]*zip.File) ([]string, error) {
	file := files["xl/sharedStrings.xml"]
	if file == nil {
		return nil, nil
	}
	body, err := readZipFile(file, 32<<20)
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var result []string
	var inSI bool
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "si" {
				inSI = true
				builder.Reset()
			}
			if inSI && value.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return nil, err
				}
				builder.WriteString(text)
			}
		case xml.EndElement:
			if value.Name.Local == "si" && inSI {
				result = append(result, builder.String())
				inSI = false
			}
		}
	}
	return result, nil
}

func (r *XLSX) readSheet(file *zip.File, index int, name string, shared []string) (preview.WorkbookSheet, int, error) {
	body, err := readZipFile(file, r.limits.MaxUncompressedBytes)
	if err != nil {
		return preview.WorkbookSheet{}, 0, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	sheet := preview.WorkbookSheet{Index: index, Name: name}
	var currentRow *preview.TableRow
	var cellRef, cellType, cellValue, inlineValue string
	cellCount := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sheet, cellCount, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "dimension":
				for _, attr := range value.Attr {
					if attr.Name.Local == "ref" {
						if rows, columns, ok := parseDimension(attr.Value); ok {
							if rows > r.limits.MaxRowsPerSheet || columns > r.limits.MaxColumnsPerSheet {
								return sheet, cellCount, fmt.Errorf("%w: Sheet %q 尺寸 %dx%d", preview.ErrTableTooLarge, name, rows, columns)
							}
							sheet.RowCount, sheet.ColumnCount = rows, columns
						}
					}
				}
			case "row":
				rowIndex := sheet.RowCount
				for _, attr := range value.Attr {
					if attr.Name.Local == "r" {
						if n, e := strconv.Atoi(attr.Value); e == nil && n > 0 {
							rowIndex = n - 1
						}
					}
				}
				currentRow = &preview.TableRow{Row: rowIndex}
			case "c":
				cellRef, cellType, cellValue, inlineValue = "", "", "", ""
				for _, attr := range value.Attr {
					if attr.Name.Local == "r" {
						cellRef = attr.Value
					}
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
			case "v":
				if err := decoder.DecodeElement(&cellValue, &value); err != nil {
					return sheet, cellCount, err
				}
			case "t":
				if cellType == "inlineStr" {
					if err := decoder.DecodeElement(&inlineValue, &value); err != nil {
						return sheet, cellCount, err
					}
				}
			case "mergeCell":
				for _, attr := range value.Attr {
					if attr.Name.Local == "ref" {
						if merged, ok := parseMerge(attr.Value); ok {
							sheet.MergedCells = append(sheet.MergedCells, merged)
						}
					}
				}
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "c":
				if currentRow == nil {
					continue
				}
				_, col, ok := parseCellRef(cellRef)
				if !ok {
					continue
				}
				value := inlineValue
				if value == "" {
					value = cellValue
				}
				if cellType == "s" {
					if i, e := strconv.Atoi(cellValue); e == nil && i >= 0 && i < len(shared) {
						value = shared[i]
					}
				}
				if value != "" {
					currentRow.Cells = append(currentRow.Cells, preview.TableCell{Column: col, Value: value})
					cellCount++
				}
				if col+1 > sheet.ColumnCount {
					sheet.ColumnCount = col + 1
				}
			case "row":
				if currentRow != nil {
					if len(currentRow.Cells) > 0 {
						sheet.Rows = append(sheet.Rows, *currentRow)
					}
					if currentRow.Row+1 > sheet.RowCount {
						sheet.RowCount = currentRow.Row + 1
					}
					if sheet.RowCount > r.limits.MaxRowsPerSheet || sheet.ColumnCount > r.limits.MaxColumnsPerSheet {
						return sheet, cellCount, fmt.Errorf("%w: Sheet %q 尺寸 %dx%d", preview.ErrTableTooLarge, name, sheet.RowCount, sheet.ColumnCount)
					}
				}
				currentRow = nil
			}
		}
	}
	return sheet, cellCount, nil
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("XLSX 必需对象缺失")
	}
	if int64(file.UncompressedSize64) > limit {
		return nil, fmt.Errorf("%w: XML 对象过大", preview.ErrTableTooLarge)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: XML 对象过大", preview.ErrTableTooLarge)
	}
	return body, nil
}

func parseCellRef(ref string) (row, col int, ok bool) {
	letters := 0
	for letters < len(ref) && ((ref[letters] >= 'A' && ref[letters] <= 'Z') || (ref[letters] >= 'a' && ref[letters] <= 'z')) {
		letters++
	}
	if letters == 0 || letters == len(ref) {
		return 0, 0, false
	}
	for _, ch := range strings.ToUpper(ref[:letters]) {
		col = col*26 + int(ch-'A'+1)
	}
	n, err := strconv.Atoi(ref[letters:])
	if err != nil || n <= 0 {
		return 0, 0, false
	}
	return n - 1, col - 1, true
}

func parseMerge(ref string) (preview.MergedCell, bool) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return preview.MergedCell{}, false
	}
	r1, c1, ok1 := parseCellRef(parts[0])
	r2, c2, ok2 := parseCellRef(parts[1])
	if !ok1 || !ok2 || r2 < r1 || c2 < c1 {
		return preview.MergedCell{}, false
	}
	return preview.MergedCell{StartRow: r1, StartColumn: c1, RowSpan: r2 - r1 + 1, ColumnSpan: c2 - c1 + 1}, true
}

func parseDimension(ref string) (rows, columns int, ok bool) {
	parts := strings.Split(ref, ":")
	last := parts[len(parts)-1]
	row, col, valid := parseCellRef(last)
	if !valid {
		return 0, 0, false
	}
	return row + 1, col + 1, true
}
