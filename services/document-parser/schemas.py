"""ParsedDocument 稳定协议模型。

字段名与 Go 侧 internal/service/rag/parser/contract.go 保持一一对应。
同一 schema 主版本只允许新增可选字段；删除字段或改变字段语义必须升级主版本。
本模块不包含任何 Chunk/RAG 分块概念。
"""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field, model_validator

SCHEMA_VERSION = "1.0"
ADAPTER_VERSION = "1.0"
PARSER_NAME = "docling"

BLOCK_TYPES = frozenset(
    {
        "title",
        "heading",
        "paragraph",
        "list_item",
        "code",
        "formula",
        "table",
        "picture",
        "caption",
        "footnote",
        "page_header",
        "page_footer",
        "unknown",
    }
)

KNOWN_BLOCK_LABELS: dict[str, str] = {
    "title": "title",
    "section_header": "heading",
    "paragraph": "paragraph",
    "text": "paragraph",
    "list_item": "list_item",
    "code": "code",
    "formula": "formula",
    "table": "table",
    "picture": "picture",
    "caption": "caption",
    "footnote": "footnote",
    "page_header": "page_header",
    "page_footer": "page_footer",
    "document_index": "unknown",
    "checkbox_selected": "unknown",
    "checkbox_unselected": "unknown",
    "form": "unknown",
    "key_value_region": "unknown",
    "grading_scale": "unknown",
    "handwritten_text": "paragraph",
    "empty_value": "unknown",
    "reference": "unknown",
    "field_region": "unknown",
    "field_heading": "heading",
    "field_item": "unknown",
    "field_key": "unknown",
    "field_value": "unknown",
    "field_hint": "unknown",
    "marker": "unknown",
}


class ParserInfo(BaseModel):
    name: str
    version: str
    adapter_version: str


class SourceInfo(BaseModel):
    file_name: str
    format: str
    sha256: str
    size: int


class DocumentInfo(BaseModel):
    title: str = ""
    markdown: str = ""
    page_count: int = 0
    metadata: dict[str, Any] = Field(default_factory=dict)


class SourceLocation(BaseModel):
    page: int = 0
    bbox: list[float] = Field(default_factory=list)
    docling_ref: str | None = None


class Block(BaseModel):
    id: str
    type: str
    text: str
    markdown: str = ""
    heading_path: list[str] = Field(default_factory=list)
    source: SourceLocation = Field(default_factory=SourceLocation)
    table_ref: str | None = None
    asset_refs: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def _validate_type(self) -> Block:
        if self.type not in BLOCK_TYPES:
            raise ValueError(f"未知 Block 类型: {self.type!r}")
        return self


class TableCell(BaseModel):
    row: int
    column: int
    row_span: int = 1
    col_span: int = 1
    text: str = ""


class Table(BaseModel):
    id: str
    caption: str = ""
    page_start: int = 0
    page_end: int = 0
    bbox: list[float] = Field(default_factory=list)
    headers: list[list[str]] = Field(default_factory=list)
    rows: list[list[str]] = Field(default_factory=list)
    cells: list[TableCell] = Field(default_factory=list)
    row_count: int = 0
    column_count: int = 0
    markdown: str = ""


class Asset(BaseModel):
    id: str
    kind: str = "picture"
    mime_type: str = ""
    sha256: str = ""
    width: int = 0
    height: int = 0
    page: int = 0
    bbox: list[float] = Field(default_factory=list)
    caption: str = ""
    data_base64: str | None = None
    object_key: str | None = None
    omitted: bool = False
    metadata: dict[str, Any] = Field(default_factory=dict)


class ParsedDocument(BaseModel):
    schema_version: str
    parser: ParserInfo
    source: SourceInfo
    document: DocumentInfo
    blocks: list[Block] = Field(default_factory=list)
    tables: list[Table] = Field(default_factory=list)
    assets: list[Asset] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def _validate_version(self) -> ParsedDocument:
        if self.schema_version != SCHEMA_VERSION:
            raise ValueError(
                f"不支持的 schema_version {self.schema_version!r}（支持: {SCHEMA_VERSION}）"
            )
        return self


class ParseOptions(BaseModel):
    """解析请求选项。不包含 chunk size/overlap/tokenizer/Embedding 参数。"""

    schema_version: str = SCHEMA_VERSION
    ocr_languages: list[str] = Field(default_factory=lambda: ["zh", "en"])
    do_ocr: bool = True
    table_structure: bool = True
    extract_pictures: bool = True
    include_bboxes: bool = True

    @model_validator(mode="after")
    def _validate_version(self) -> ParseOptions:
        if self.schema_version != SCHEMA_VERSION:
            raise ValueError(
                f"不支持的 schema_version {self.schema_version!r}（支持: {SCHEMA_VERSION}）"
            )
        if not self.ocr_languages:
            raise ValueError("ocr_languages 不能为空")
        return self


class ParseError(BaseModel):
    code: str
    message: str
