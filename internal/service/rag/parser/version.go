package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ParseConfigHash 计算解析配置的确定性哈希（进入 parse_config_hash）。
// 相同配置必须产生相同哈希；配置变化必须产生不同哈希。
func ParseConfigHash(options ParseOptions) (string, error) {
	canonical, err := json.Marshal(options)
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
