"""app 层测试：health、格式/大小限制、options 校验、错误码、无 Chunk 字段。"""

from __future__ import annotations

import json
import time

import pytest
from fastapi.testclient import TestClient
from fixtures import (
    build_docx,
    build_fake_docx,
    build_fake_pdf,
    build_jpeg,
    build_pdf,
    build_png,
    build_pptx,
    build_xlsx,
)
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter

import app as app_module
import parser_observability
from app import app

DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

INIT_TIMEOUT_S = 120


def _wait_ready() -> None:
    """等待常驻模型初始化完成（首次启动需要下载/加载模型）。"""
    deadline = time.monotonic() + INIT_TIMEOUT_S
    while not app_module._ready.is_set():
        if time.monotonic() > deadline:
            raise TimeoutError("document-parser 模型初始化超时")
        time.sleep(0.2)


@pytest.fixture(scope="module")
def client():
    with TestClient(app) as c:
        _wait_ready()
        yield c


def test_health_live(client):
    resp = client.get("/health/live")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"


def test_parse_continues_incoming_traceparent(client):
    exporter = InMemorySpanExporter()
    assert parser_observability._provider is not None
    parser_observability._provider.add_span_processor(SimpleSpanProcessor(exporter))
    resp = client.post(
        "/v1/parse",
        headers={"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"},
        files={"file": ("notes.txt", b"hello", "text/plain")},
        data={"options": "{}"},
    )
    assert resp.status_code == 422
    spans = exporter.get_finished_spans()
    assert any(format(span.context.trace_id, "032x") == "0123456789abcdef0123456789abcdef" for span in spans)


def test_unknown_format_rejected(client):
    resp = client.post(
        "/v1/parse",
        files={"file": ("notes.txt", b"hello", "text/plain")},
        data={"options": json.dumps({"schema_version": "1.0"})},
    )
    assert resp.status_code == 422
    assert "不支持的格式" in resp.json()["detail"]


def test_fake_pdf_rejected(client):
    resp = client.post(
        "/v1/parse",
        files={"file": ("fake.pdf", build_fake_pdf(), "application/pdf")},
        data={"options": "{}"},
    )
    assert resp.status_code == 422
    assert "伪造格式" in resp.json()["detail"]


def test_fake_docx_rejected(client):
    resp = client.post(
        "/v1/parse",
        files={"file": ("fake.docx", build_fake_docx(), DOCX_MIME)},
        data={"options": "{}"},
    )
    assert resp.status_code == 422
    assert "伪造格式" in resp.json()["detail"]


def test_empty_file_rejected(client):
    resp = client.post(
        "/v1/parse",
        files={"file": ("empty.pdf", b"", "application/pdf")},
        data={"options": "{}"},
    )
    assert resp.status_code == 422
    assert "空文件" in resp.json()["detail"]


def test_invalid_options_rejected(client):
    resp = client.post(
        "/v1/parse",
        files={"file": ("x.pdf", build_pdf(), "application/pdf")},
        data={"options": "{not json"},
    )
    assert resp.status_code == 422
    assert "options 无效" in resp.json()["detail"]


def test_oversize_file_rejected(monkeypatch):
    monkeypatch.setattr(app_module, "MAX_FILE_BYTES", 8)
    with TestClient(app) as c:
        _wait_ready()
        resp = c.post(
            "/v1/parse",
            files={"file": ("big.pdf", b"x" * 64, "application/pdf")},
            data={"options": "{}"},
        )
    assert resp.status_code == 413


def test_parse_docx_returns_protocol_document(client):
    """DOCX 真实转换：校验协议结构与引用完整性（需要模型已下载）。"""
    resp = client.post(
        "/v1/parse",
        files={"file": ("sample.docx", build_docx(), DOCX_MIME)},
        data={"options": json.dumps({"schema_version": "1.0", "do_ocr": False})},
    )
    assert resp.status_code == 200, resp.text
    doc = resp.json()
    assert doc["schema_version"] == "1.0"
    assert doc["parser"]["name"] == "docling"
    assert doc["source"]["format"] == "docx"
    assert doc["source"]["sha256"]
    assert doc["document"]["title"]
    assert doc["document"]["markdown"]
    # 不含任何 Chunk/RAG 字段
    assert "chunks" not in doc
    assert "chunk_size" not in doc
    assert "overlap" not in doc
    assert "token_count" not in doc
    # 引用完整
    table_ids = {t["id"] for t in doc["tables"]}
    asset_ids = {a["id"] for a in doc["assets"]}
    for block in doc["blocks"]:
        if block.get("table_ref"):
            assert block["table_ref"] in table_ids
        for ref in block.get("asset_refs", []):
            assert ref in asset_ids


@pytest.mark.parametrize(
    "options",
    [
        {"schema_version": "1.0", "ocr_languages": ["zh", "en"]},
        {"schema_version": "1.0", "ocr_languages": ["bad-lang"]},
    ],
)
def test_options_language_mapping(client, options):
    """语言映射验证：非法语言应报错；合法语言可进入解析。"""
    resp = client.post(
        "/v1/parse",
        files={"file": ("l.pdf", build_pdf(), "application/pdf")},
        data={"options": json.dumps(options)},
    )
    if options["ocr_languages"] == ["bad-lang"]:
        assert resp.status_code == 422
        assert "OCR 语言" in resp.json()["detail"]


@pytest.mark.parametrize(
    ("file_name", "content_builder", "want_format"),
    [
        ("sample.xlsx", build_xlsx, "xlsx"),
        ("sample.pptx", build_pptx, "pptx"),
        ("sample.png", build_png, "png"),
        ("sample.jpg", build_jpeg, "jpeg"),
        ("sample.pdf", build_pdf, "pdf"),
        ("sample.docx", build_docx, "docx"),
    ],
)
def test_source_format_matches_real_format(client, file_name, content_builder, want_format):
    """真实格式解析后 source.format 必须按实际格式标记，禁止统一写成 docx。"""
    resp = client.post(
        "/v1/parse",
        files={"file": (file_name, content_builder(), "application/octet-stream")},
        data={"options": json.dumps({"schema_version": "1.0", "do_ocr": False, "extract_pictures": False})},
    )
    assert resp.status_code == 200, resp.text
    doc = resp.json()
    assert doc["source"]["format"] == want_format


def test_png_not_marked_docx(client):
    """图片解析结果不得被误标为 docx（历史回归保护）。"""
    resp = client.post(
        "/v1/parse",
        files={"file": ("photo.png", build_png(), "image/png")},
        data={"options": json.dumps({"schema_version": "1.0", "do_ocr": False, "extract_pictures": False})},
    )
    assert resp.status_code == 200, resp.text
    assert resp.json()["source"]["format"] != "docx"


def test_fake_extension_image_rejected(client):
    """伪造图片扩展名（内容不是图片）应拒绝。"""
    resp = client.post(
        "/v1/parse",
        files={"file": ("fake.png", b"this is not an image", "image/png")},
        data={"options": "{}"},
    )
    assert resp.status_code == 422
    assert "伪造格式" in resp.json()["detail"]
