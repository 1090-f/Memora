package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadExampleConfig 验证示例配置文件可完整加载且包含新模块配置。
func TestLoadExampleConfig(t *testing.T) {
	// 示例文件扩展名为 .example，复制为临时 .yaml 再加载。
	content, err := os.ReadFile("../../configs/config.yaml.example")
	if err != nil {
		t.Fatalf("读取示例配置失败: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	cfg, err := load(tmp, func(c *Config) error { return c.Validate() })
	if err != nil {
		t.Fatalf("加载示例配置失败: %v", err)
	}
	if cfg.DocumentParser.BaseURL == "" {
		t.Error("document_parser.base_url 应为默认值")
	}
	if !cfg.DocumentParser.AutoStart {
		t.Error("document_parser.auto_start 应默认启用")
	}
	if cfg.DocumentParser.AutoStartCommand == "" || len(cfg.DocumentParser.AutoStartArgs) == 0 {
		t.Error("document_parser 自动启动命令应完整配置")
	}
	if cfg.DocumentParser.Timeout <= 0 {
		t.Error("document_parser.timeout 应为正数")
	}
	if len(cfg.DocumentParser.OCRLanguages) == 0 {
		t.Error("ocr_languages 应为默认值")
	}
	if cfg.Chunking.MaxTokens <= 0 || cfg.Chunking.MinTokens < 0 || cfg.Chunking.OverlapTokens < 0 {
		t.Error("chunking 参数不合法")
	}
	if cfg.Chunking.StrategyVersion == "" {
		t.Error("strategy_version 应为默认值")
	}
	if cfg.Chunking.Strategy != "auto" {
		t.Errorf("chunking.strategy = %q，期望 auto", cfg.Chunking.Strategy)
	}
	if cfg.AssetEnrichment.Mode != "none" {
		t.Errorf("asset_enrichment.mode = %q，期望 none", cfg.AssetEnrichment.Mode)
	}
}

// TestChunkingConfigValidation 验证分块配置校验规则。
func TestChunkingConfigValidation(t *testing.T) {
	base := Config{}
	base.Chunking = ChunkingConfig{MaxTokens: 100, MinTokens: 200}
	if err := base.Validate(); err == nil {
		t.Error("min_tokens > max_tokens 应校验失败")
	}
	base.Chunking = ChunkingConfig{MaxTokens: 0}
	if err := base.Validate(); err == nil {
		t.Error("max_tokens = 0 应校验失败")
	}
	base.Chunking = ChunkingConfig{MaxTokens: 100, MinTokens: 50}
	base.AssetEnrichment.Mode = "remote"
	if err := base.Validate(); err == nil {
		t.Error("不支持的 asset_enrichment.mode 应校验失败")
	}
	base.AssetEnrichment.Mode = "none"
	if err := base.Validate(); err == nil {
		t.Error("缺少核心配置（app.address 等）应校验失败")
	}
}
