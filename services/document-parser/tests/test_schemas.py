"""契约测试：字段名与语义必须与 Go internal/service/rag/parser/contract.go 保持一致。"""

from __future__ import annotations

import json

import pytest

from schemas import (
    SCHEMA_VERSION,
    Asset,
    Block,
    ParsedDocument,
    ParseOptions,
    Table,
)

# 与 Go 侧 contract_test.go 的 contractFixture 字段保持一致。
CONTRACT_FIXTURE = {
    "schema_version": "1.0",
    "parser": {"name": "docling", "version": "2.118.1", "adapter_version": "1.0"},
    "source": {
        "file_name": "example.pdf",
        "format": "pdf",
        "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        "size": 123456,
    },
    "document": {
        "title": "示例文档",
        "markdown": "# 示例文档\n正文",
        "page_count": 10,
        "metadata": {"author": "test"},
    },
    "blocks": [
        {
            "id": "block-000001",
            "type": "heading",
            "text": "第一章",
            "markdown": "第一章",
            "heading_path": ["第一章"],
            "source": {"page": 1, "bbox": [0, 0, 100, 50], "docling_ref": "#/texts/1"},
            "asset_refs": [],
        },
        {
            "id": "block-000002",
            "type": "paragraph",
            "text": "正文段落",
            "markdown": "正文段落",
            "heading_path": ["第一章"],
            "source": {"page": 1, "bbox": [0, 60, 100, 90]},
            "table_ref": "table-000001",
            "asset_refs": ["asset-000001"],
        },
    ],
    "tables": [
        {
            "id": "table-000001",
            "caption": "表 1 销售数据",
            "page_start": 1,
            "page_end": 1,
            "bbox": [0, 100, 300, 220],
            "headers": [["地区", "销售额"]],
            "rows": [["华东", "100"]],
            "cells": [
                {"row": 0, "column": 0, "row_span": 1, "col_span": 1, "text": "地区"},
                {"row": 0, "column": 1, "row_span": 1, "col_span": 1, "text": "销售额"},
            ],
            "row_count": 1,
            "column_count": 2,
            "markdown": "| 地区 | 销售额 |",
        }
    ],
    "assets": [
        {
            "id": "asset-000001",
            "kind": "picture",
            "mime_type": "image/png",
            "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
            "width": 1200,
            "height": 800,
            "page": 1,
            "bbox": [0, 0, 300, 220],
            "caption": "图 1 系统架构",
            "omitted": False,
            "metadata": {},
        }
    ],
    "warnings": [],
}


def test_contract_fixture_parses():
    parsed = ParsedDocument.model_validate(CONTRACT_FIXTURE)
    assert parsed.schema_version == SCHEMA_VERSION
    assert parsed.parser.name == "docling"
    assert len(parsed.blocks) == 2
    assert parsed.blocks[1].table_ref == parsed.tables[0].id
    assert parsed.blocks[1].asset_refs[0] == parsed.assets[0].id


def test_contract_json_round_trip():
    parsed = ParsedDocument.model_validate(CONTRACT_FIXTURE)
    dumped = json.loads(parsed.model_dump_json())
    assert dumped["blocks"][1]["table_ref"] == "table-000001"
    assert dumped["blocks"][1]["asset_refs"] == ["asset-000001"]
    assert dumped["assets"][0]["caption"] == "图 1 系统架构"


def test_schema_version_only_exact_match():
    with pytest.raises(ValueError):
        ParsedDocument.model_validate({**CONTRACT_FIXTURE, "schema_version": "2.0"})
    with pytest.raises(ValueError):
        ParsedDocument.model_validate({**CONTRACT_FIXTURE, "schema_version": ""})


def test_block_type_must_be_known():
    with pytest.raises(ValueError):
        Block(id="b1", type="not-a-type", text="x")
    Block(id="b1", type="unknown", text="x")


def test_parse_options_defaults():
    opts = ParseOptions()
    assert opts.ocr_languages == ["zh", "en"]
    assert opts.do_ocr is True
    assert opts.table_structure is True
    assert opts.extract_pictures is True
    assert opts.include_bboxes is True


def test_parse_options_rejects_bad_version_and_empty_langs():
    with pytest.raises(ValueError):
        ParseOptions(schema_version="0.9")
    with pytest.raises(ValueError):
        ParseOptions(ocr_languages=[])


def test_table_cells_with_spans():
    table = Table(
        id="table-000001",
        headers=[["a"]],
        rows=[],
        cells=[{"row": 0, "column": 0, "row_span": 2, "col_span": 1, "text": "merged"}],
        row_count=2,
        column_count=1,
    )
    assert table.cells[0].row_span == 2
    assert table.rows == []


def test_asset_optional_base64_and_object_key():
    asset = Asset(id="a1", mime_type="image/png")
    assert asset.data_base64 is None
    assert asset.object_key is None
    assert asset.omitted is False
    with_object_key = Asset(id="a1", mime_type="image/png", object_key="derived/...")
    assert with_object_key.object_key == "derived/..."
