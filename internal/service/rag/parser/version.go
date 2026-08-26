package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ParseRuntimeIdentity 描述所有可能参与解析的实现版本。
// Parser 依赖升级必须修改对应版本，确保旧 Parsed Artifact 不会被错误复用。
type ParseRuntimeIdentity struct {
	SchemaVersion                string            `json:"schema_version"`
	AdapterVersion               string            `json:"adapter_version"`
	DocumentParserServiceVersion string            `json:"document_parser_service_version"`
	ParserVersions               map[string]string `json:"parser_versions"`
}

func DefaultParseRuntimeIdentity() ParseRuntimeIdentity {
	return ParseRuntimeIdentity{
		SchemaVersion: SchemaVersion, AdapterVersion: AdapterVersion,
		DocumentParserServiceVersion: DocumentParserServiceVersion,
		ParserVersions: map[string]string{
			ParserNameGoText: GoParserVersion, ParserNameGoMarkdown: GoParserVersion,
			ParserNameDocling: DoclingParserVersion,
		},
	}
}

// ParseConfigHash 计算解析配置与默认 Parser/Adapter 版本的确定性哈希。
func ParseConfigHash(options ParseOptions) (string, error) {
	return ParseConfigHashWithIdentity(options, DefaultParseRuntimeIdentity())
}

// ParseConfigHashWithIdentity 供测试和显式运行时版本配置使用。
func ParseConfigHashWithIdentity(options ParseOptions, identity ParseRuntimeIdentity) (string, error) {
	payload := struct {
		Options ParseOptions         `json:"options"`
		Runtime ParseRuntimeIdentity `json:"runtime"`
	}{Options: options, Runtime: identity}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化解析配置失败: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// CheckSchemaVersion 校验 ParsedDocument schema 版本是否受支持。
// 只接受明确支持的版本；主版本不兼容时返回错误，禁止静默降级。
func CheckSchemaVersion(version string) error {
	if version == "" {
		return fmt.Errorf("ParsedDocument 缺少 schema_version")
	}
	if !SupportedSchemaVersions[version] {
		return fmt.Errorf("不支持 ParsedDocument schema_version %q（支持: %v）", version, schemaVersionsList())
	}
	return nil
}

// schemaVersionsList 返回受支持版本的稳定列表（供错误信息使用）。
func schemaVersionsList() []string {
	out := make([]string, 0, len(SupportedSchemaVersions))
	for v := range SupportedSchemaVersions {
		out = append(out, v)
	}
	return out
}
