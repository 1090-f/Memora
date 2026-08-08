package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

// contractFixture 是与 Python Pydantic 模型共用的契约样例：
// 字段名与语义必须与 services/document-parser/schemas.py 保持一致。
const contractFixture = `{
  "schema_version": "1.0",
  "parser": {
    "name": "docling",
    "version": "2.118.1",
    "adapter_version": "1.0"
  },
  "source": {
    "file_name": "example.pdf",
    "format": "pdf",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "size": 123456
  },
  "document": {
    "title": "示例文档",
    "markdown": "# 示例文档\n正文",
    "page_count": 10,
    "metadata": {"author": "test"}
  },
  "blocks": [
    {
      "id": "block-000001",
      "type": "heading",
      "text": "第一章",
      "markdown": "第一章",
      "heading_path": ["第一章"],
      "source": {"page": 1, "bbox": [0, 0, 100, 50], "docling_ref": "#/texts/1"},
      "asset_refs": []
    },
    {
      "id": "block-000002",
      "type": "paragraph",
      "text": "正文段落",
      "markdown": "正文段落",
      "heading_path": ["第一章"],
      "source": {"page": 1, "bbox": [0, 60, 100, 90]},
      "table_ref": "table-000001",
      "asset_refs": ["asset-000001"]
    }
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
        {"row": 0, "column": 1, "row_span": 1, "col_span": 1, "text": "销售额"}
      ],
      "row_count": 1,
      "column_count": 2,
      "markdown": "| 地区 | 销售额 |"
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
      "omitted": false,
      "metadata": {}
    }
  ],
  "warnings": []
}`

// TestContractFixtureUnmarshal 验证契约样例可被 Go DTO 正确解码。
func TestContractFixtureUnmarshal(t *testing.T) {
	var doc ParsedDocument
	if err := json.Unmarshal([]byte(contractFixture), &doc); err != nil {
		t.Fatalf("解码契约样例失败: %v", err)
	}
	if err := CheckSchemaVersion(doc.SchemaVersion); err != nil {
		t.Fatalf("schema 版本校验失败: %v", err)
	}
	if doc.Parser.Name != ParserNameDocling {
		t.Errorf("parser.name = %q, 期望 %q", doc.Parser.Name, ParserNameDocling)
	}
	if len(doc.Blocks) != 2 {
		t.Fatalf("blocks 数量 = %d, 期望 2", len(doc.Blocks))
	}
	// 校验引用：table_ref 与 asset_refs 指向的实体必须存在。
	if doc.Blocks[1].TableRef != doc.Tables[0].ID {
		t.Errorf("block table_ref = %q, 期望 %q", doc.Blocks[1].TableRef, doc.Tables[0].ID)
	}
	if doc.Blocks[1].AssetRefs[0] != doc.Assets[0].ID {
		t.Errorf("block asset_refs[0] = %q, 期望 %q", doc.Blocks[1].AssetRefs[0], doc.Assets[0].ID)
	}
}

// TestSchemaVersionOnlyExactMatch 验证只接受明确支持的版本。
func TestSchemaVersionOnlyExactMatch(t *testing.T) {
	for _, version := range []string{"1.0", ""} {
		if version == "" {
			if err := CheckSchemaVersion(version); err == nil {
				t.Error("空 schema_version 应返回错误")
			}
			continue
		}
		if err := CheckSchemaVersion(version); err != nil {
			t.Errorf("受支持版本 %q 校验失败: %v", version, err)
		}
	}
	for _, version := range []string{"0.9", "2.0", "1.1", "v1.0"} {
		if err := CheckSchemaVersion(version); err == nil {
			t.Errorf("不支持的版本 %q 应返回错误", version)
		} else if !strings.Contains(err.Error(), version) {
			t.Errorf("错误信息应包含版本号 %q: %v", version, err)
		}
	}
}

// TestParseConfigHashDeterministic 验证解析配置哈希确定性与敏感性。
func TestParseConfigHashDeterministic(t *testing.T) {
	options := DefaultParseOptions()
	first, err := ParseConfigHash(options)
	if err != nil {
		t.Fatalf("计算解析配置哈希失败: %v", err)
	}
	second, err := ParseConfigHash(options)
	if err != nil {
		t.Fatalf("重复计算解析配置哈希失败: %v", err)
	}
	if first != second {
		t.Errorf("相同配置哈希不一致: %s != %s", first, second)
	}
	// 改变任意选项必须产生不同哈希。
	changed := DefaultParseOptions()
	changed.TableStructure = false
	different, err := ParseConfigHash(changed)
	if err != nil {
		t.Fatalf("计算变更配置哈希失败: %v", err)
	}
	if different == first {
		t.Error("变更解析配置后哈希应变化")
	}
}

// TestParsedDocumentRoundTrip 验证 ParsedDocument 完整序列化往返一致。
func TestParsedDocumentRoundTrip(t *testing.T) {
	var original ParsedDocument
	if err := json.Unmarshal([]byte(contractFixture), &original); err != nil {
		t.Fatalf("解码契约样例失败: %v", err)
	}
	encoded, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var decoded ParsedDocument
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if decoded.SchemaVersion != original.SchemaVersion ||
		decoded.Source.SHA256 != original.Source.SHA256 ||
		len(decoded.Blocks) != len(original.Blocks) ||
		decoded.Blocks[1].TableRef != original.Blocks[1].TableRef ||
		decoded.Assets[0].Caption != original.Assets[0].Caption {
		t.Error("序列化往返后关键字段不一致")
	}
}

// TestArtifactManifestRoundTrip 验证 Artifact manifest 序列化往返一致。
func TestArtifactManifestRoundTrip(t *testing.T) {
	manifest := ArtifactManifest{
		ArtifactSchemaVersion:       ArtifactSchemaVersion,
		ParsedDocumentSchemaVersion: SchemaVersion,
		SourceSHA256:                "src-hash",
		ParserName:                  ParserNameDocling,
		ParserVersion:               "2.118.1",
		AdapterVersion:              AdapterVersion,
		ParseConfigHash:             "cfg-hash",
		ParsedDocumentSHA256:        "doc-hash",
		Assets: []AssetInfo{
			{ID: "asset-000001", ObjectKey: "derived/a/d/c-1/p-h/assets/asset-000001.png", SHA256: "a-hash", Size: 1024},
		},
		CreatedAt: "2026-08-08T00:00:00Z",
	}
	encoded, err := json.Marshal(&manifest)
	if err != nil {
		t.Fatalf("序列化 manifest 失败: %v", err)
	}
	var decoded ArtifactManifest
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("反序列化 manifest 失败: %v", err)
	}
	if decoded.ParserName != ParserNameDocling ||
		decoded.Assets[0].ObjectKey != manifest.Assets[0].ObjectKey ||
		decoded.ParsedDocumentSHA256 != "doc-hash" {
		t.Error("manifest 往返后关键字段不一致")
	}
}
