// Package parser 定义文档解析的统一契约与版本规则。
//
// 契约说明：
//   - ParsedDocument 是 Go RAG Pipeline 与 Python document-parser 之间唯一的稳定协议；
//   - 同一 schema 主版本只允许新增可选字段，删除字段或改变字段语义必须升级主版本；
//   - 解析协议与分块（Chunk）完全解耦：本包不出现任何 chunk size/overlap/tokenizer 概念。
package parser

// Schema 版本常量。schema_version 为 "主版本.次版本"，
// 主版本不兼容必须升级主版本号，Go 只接受 SupportedSchemaVersions 中的版本。
const (
	// SchemaVersion 是当前 ParsedDocument 协议版本。
	SchemaVersion = "1.0"
	// ArtifactSchemaVersion 是 Artifact manifest 协议版本。
	ArtifactSchemaVersion = "1.0"
	// AdapterVersion 是 Docling → ParsedDocument 转换语义版本。
	AdapterVersion = "1.0"
	// GoParserVersion 是内置 TXT/Markdown Parser 的转换语义版本。
	GoParserVersion = "1.0"
	// DocumentParserServiceVersion 是 Python document-parser 服务协议实现版本。
	DocumentParserServiceVersion = "0.1.0"
	// DoclingParserVersion 必须与 services/document-parser/uv.lock 的锁定版本同步。
	DoclingParserVersion = "2.118.1"
)

// Parser 名称常量。
const (
	// ParserNameDocling 是 Python document-parser 使用的解析引擎名。
	ParserNameDocling = "docling"
	// ParserNameGoText 是 Go TextParser 使用的解析引擎名。
	ParserNameGoText = "go-text"
	// ParserNameGoMarkdown 是 Go MarkdownParser 使用的解析引擎名。
	ParserNameGoMarkdown = "go-markdown"
)

// SupportedSchemaVersions 列出 Go 明确支持的 ParsedDocument schema 版本。
var SupportedSchemaVersions = map[string]bool{SchemaVersion: true}

// ParsedDocument 是解析产物稳定协议：统一、无 Chunk 概念。
type ParsedDocument struct {
	SchemaVersion string       `json:"schema_version"`
	Parser        ParserInfo   `json:"parser"`
	Source        SourceInfo   `json:"source"`
	Document      DocumentInfo `json:"document"`
	Blocks        []Block      `json:"blocks"`
	Tables        []Table      `json:"tables"`
	Assets        []Asset      `json:"assets"`
	Warnings      []string     `json:"warnings"`
}

// ParserInfo 描述生成该解析产物的解析引擎与转换语义版本。
type ParserInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	AdapterVersion string `json:"adapter_version"`
}

// SourceInfo 描述被解析的原始文件信息。
type SourceInfo struct {
	FileName string `json:"file_name"`
	Format   string `json:"format"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

// DocumentInfo 描述解析后的文档级信息。
type DocumentInfo struct {
	Title     string         `json:"title"`
	Markdown  string         `json:"markdown"`
	PageCount int            `json:"page_count"`
	Metadata  map[string]any `json:"metadata"`
}

// BlockType 是 Block 的稳定类型集合。
const (
	BlockTypeTitle      = "title"
	BlockTypeHeading    = "heading"
	BlockTypeParagraph  = "paragraph"
	BlockTypeListItem   = "list_item"
	BlockTypeCode       = "code"
	BlockTypeFormula    = "formula"
	BlockTypeTable      = "table"
	BlockTypePicture    = "picture"
	BlockTypeCaption    = "caption"
	BlockTypeFootnote   = "footnote"
	BlockTypePageHeader = "page_header"
	BlockTypePageFooter = "page_footer"
	BlockTypeUnknown    = "unknown"
)

// Block 是阅读顺序中的结构单元，保留来源信息与结构引用。
// 未知 label 映射为 BlockTypeUnknown 并写入 warning，禁止丢弃正文。
type Block struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Text        string         `json:"text"`
	Markdown    string         `json:"markdown"`
	HeadingPath []string       `json:"heading_path"`
	Source      SourceLocation `json:"source"`
	TableRef    string         `json:"table_ref,omitempty"`
	AssetRefs   []string       `json:"asset_refs"`
}

// SourceLocation 描述 Block/Table/Asset 在原文中的位置。
type SourceLocation struct {
	Page       int       `json:"page"`
	BBox       []float64 `json:"bbox"`
	DoclingRef string    `json:"docling_ref,omitempty"`
}

// Table 是独立的表格结构，只表达表格内容，不决定分块策略。
type Table struct {
	ID          string      `json:"id"`
	Caption     string      `json:"caption"`
	PageStart   int         `json:"page_start"`
	PageEnd     int         `json:"page_end"`
	BBox        []float64   `json:"bbox"`
	Headers     [][]string  `json:"headers"`
	Rows        [][]string  `json:"rows"`
	Cells       []TableCell `json:"cells"`
	RowCount    int         `json:"row_count"`
	ColumnCount int         `json:"column_count"`
	Markdown    string      `json:"markdown"`
}

// TableCell 描述带合并关系的单元格。
type TableCell struct {
	Row     int    `json:"row"`
	Column  int    `json:"column"`
	RowSpan int    `json:"row_span"`
	ColSpan int    `json:"col_span"`
	Text    string `json:"text"`
}

// Asset 是文档中提取的图片等二进制资产。
// 保存 Artifact 时 data_base64 被替换为 ObjectKey；日志禁止记录 base64。
type Asset struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	MIMEType   string    `json:"mime_type"`
	SHA256     string    `json:"sha256"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Page       int       `json:"page"`
	BBox       []float64 `json:"bbox"`
	Caption    string    `json:"caption"`
	DataBase64 string    `json:"data_base64,omitempty"`
	ObjectKey  string    `json:"object_key,omitempty"`
	Omitted    bool      `json:"omitted"`
	// SourceRef 是 Markdown 图片的原始引用（预览重写用）；PDF/DOCX 资产为空。
	SourceRef string         `json:"source_ref,omitempty"`
	Metadata  map[string]any `json:"metadata"`
}

// ArtifactManifest 是 Parsed Artifact 的完整性清单：
// 只有 manifest 存在且全部哈希一致，Artifact 才算完整。
type ArtifactManifest struct {
	ArtifactSchemaVersion       string      `json:"artifact_schema_version"`
	ParsedDocumentSchemaVersion string      `json:"parsed_document_schema_version"`
	SourceSHA256                string      `json:"source_sha256"`
	ParserName                  string      `json:"parser_name"`
	ParserVersion               string      `json:"parser_version"`
	AdapterVersion              string      `json:"adapter_version"`
	ParseConfigHash             string      `json:"parse_config_hash"`
	ParsedDocumentSHA256        string      `json:"parsed_document_sha256"`
	Assets                      []AssetInfo `json:"assets"`
	CreatedAt                   string      `json:"created_at"`
}

// AssetInfo 是 Artifact 内资产文件的元信息。
type AssetInfo struct {
	ID        string `json:"id"`
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
}

// ParseOptions 是解析请求选项，进入 parse_config_hash。
// 不包含任何 Chunk/Embedding 参数；改变此处字段语义必须升级 SchemaVersion。
type ParseOptions struct {
	SchemaVersion   string   `json:"schema_version"`
	OCRLanguages    []string `json:"ocr_languages"`
	DoOCR           bool     `json:"do_ocr"`
	DoImageOCR      bool     `json:"do_image_ocr"`
	TableStructure  bool     `json:"table_structure"`
	ExtractPictures bool     `json:"extract_pictures"`
	IncludeBBoxes   bool     `json:"include_bboxes"`
}

// DefaultParseOptions 返回默认解析选项（与 Python 服务默认值保持一致）。
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		SchemaVersion:   SchemaVersion,
		OCRLanguages:    []string{"zh", "en"},
		DoOCR:           true,
		DoImageOCR:      true,
		TableStructure:  true,
		ExtractPictures: true,
		IncludeBBoxes:   true,
	}
}
