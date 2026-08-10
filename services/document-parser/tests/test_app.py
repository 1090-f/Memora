"""app 层测试：health、格式/大小限制、options 校验、错误码、无 Chunk 字段。"""

from __future__ import annotations

import json

import pytest
from fastapi.testclient import TestClient
from fixtures import build_docx, build_fake_docx, build_fake_pdf, build_pdf

import app as app_module
from app import app

DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"


@pytest.fixture(scope="module")
def client():
    with TestClient(app) as c:
        yield c


def test_health_live(client):
    resp = client.get("/health/live")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"


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
