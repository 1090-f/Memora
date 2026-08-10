"""docling_adapter 测试：格式检测、资产限制、结构转换（真实模型用例标记为 models）。"""

from __future__ import annotations

import pytest
from docling.datamodel.base_models import InputFormat
from fixtures import build_docx, build_fake_docx, build_fake_pdf, build_pdf

import schemas
from docling_adapter import (
    DoclingAdapter,
    DocumentParserError,
    ParseFailureCode,
    _detect_format,
    _ocr_lang_codes,
)


def test_detect_format_pdf():
    assert _detect_format("a.PDF", build_pdf()) == InputFormat.PDF
    assert _detect_format("a.pdf", build_fake_pdf()) is None


def test_detect_format_docx():
    assert _detect_format("a.DOCX", build_docx()) == InputFormat.DOCX
    assert _detect_format("a.docx", build_fake_docx()) is None


def test_detect_format_unsupported():
    assert _detect_format("a.txt", b"hello") is None
    assert _detect_format("a.md", b"# t") is None
    assert _detect_format("", b"%PDF-1.4") is None


def test_ocr_lang_codes_mapping():
    assert _ocr_lang_codes(["zh", "en"]) == ["ch", "en"]
    assert _ocr_lang_codes(["en", "en"]) == ["en"]
    assert _ocr_lang_codes(["chinese"]) == ["ch"]
    with pytest.raises(DocumentParserError) as exc:
        _ocr_lang_codes(["klingon"])
    assert exc.value.code == ParseFailureCode.LIMIT_EXCEEDED


def test_parse_rejects_fake_format():
    adapter = DoclingAdapter()
    with pytest.raises(DocumentParserError) as exc:
        adapter.parse(
            file_name="fake.pdf",
            content=build_fake_pdf(),
            options=schemas.ParseOptions(),
            docling_version="test",
        )
    assert exc.value.code == ParseFailureCode.INVALID_FORMAT


def test_parse_rejects_empty():
    adapter = DoclingAdapter()
    with pytest.raises(DocumentParserError) as exc:
        adapter.parse(
            file_name="empty.pdf",
            content=b"",
            options=schemas.ParseOptions(),
            docling_version="test",
        )
    assert exc.value.code == ParseFailureCode.INVALID_FORMAT


def test_validate_references_catches_missing():
    adapter = DoclingAdapter()
    parsed = schemas.ParsedDocument(
        schema_version=schemas.SCHEMA_VERSION,
        parser=schemas.ParserInfo(name="docling", version="1", adapter_version="1.0"),
        source=schemas.SourceInfo(file_name="x.pdf", format="pdf", sha256="a", size=1),
        document=schemas.DocumentInfo(title="x"),
        blocks=[
            schemas.Block(id="b1", type="paragraph", text="t", table_ref="table-999"),
        ],
    )
    with pytest.raises(DocumentParserError) as exc:
        adapter.validate_references(parsed)
    assert "table-999" in exc.value.message


def test_validate_references_ok():
    adapter = DoclingAdapter()
    parsed = schemas.ParsedDocument(
        schema_version=schemas.SCHEMA_VERSION,
        parser=schemas.ParserInfo(name="docling", version="1", adapter_version="1.0"),
        source=schemas.SourceInfo(file_name="x.pdf", format="pdf", sha256="a", size=1),
        document=schemas.DocumentInfo(title="x"),
        tables=[schemas.Table(id="t1", row_count=1, column_count=1)],
        assets=[schemas.Asset(id="a1", mime_type="image/png")],
        blocks=[
            schemas.Block(id="b1", type="paragraph", text="t", table_ref="t1", asset_refs=["a1"]),
        ],
    )
    adapter.validate_references(parsed)


@pytest.mark.models
def test_parse_docx_structures():
    """DOCX 真实转换：标题路径、段落、表格结构。需要模型已下载。"""
    adapter = DoclingAdapter(max_asset_count=10)
    parsed = adapter.parse(
        file_name="sample.docx",
        content=build_docx(heading="第一章", paragraph="正文内容", table_rows=2, table_cols=2),
        options=schemas.ParseOptions(do_ocr=False),
        docling_version="test",
    )
    assert parsed.document.title
    assert len(parsed.blocks) >= 3
    # 表格块存在且引用完整
    table_block = next((b for b in parsed.blocks if b.type == "table"), None)
    assert table_block is not None
    table = next((t for t in parsed.tables if t.id == table_block.table_ref), None)
    assert table is not None
    assert table.row_count == 2
    assert table.column_count == 2
    # 标题路径保留
    heading = next((b for b in parsed.blocks if b.type == "heading"), None)
    assert heading is not None
    assert "第一章" in heading.text
    # 无 Chunk 字段
    assert "chunk" not in parsed.model_fields_set


@pytest.mark.models
def test_parse_pdf_structures():
    """PDF 真实转换：文本层提取与页数。需要模型已下载。"""
    adapter = DoclingAdapter()
    parsed = adapter.parse(
        file_name="min.pdf",
        content=build_pdf("Hello Docling"),
        options=schemas.ParseOptions(do_ocr=False),
        docling_version="test",
    )
    assert parsed.source.format == "pdf"
    assert parsed.document.page_count >= 1
    all_text = " ".join(b.text for b in parsed.blocks if b.text)
    assert "Hello Docling" in all_text
