package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var globalConfig *Config

// Load 从指定路径或默认位置加载完整配置并执行全部校验
func Load(configPath string) (*Config, error) {
	return load(configPath, func(cfg *Config) error { return cfg.Validate() })
}

// LoadDatabase 从指定路径加载配置并仅校验数据库相关配置项
func LoadDatabase(configPath string) (*Config, error) {
	return load(configPath, func(cfg *Config) error { return cfg.ValidateDatabase() })
}

func load(configPath string, validate func(*Config) error) (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}
	v := viper.New()
	if configPath == "" {
		configPath = os.Getenv("MEMORA_CONFIG_FILE")
	}
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
	}

	setDefaults(v)
	v.SetEnvPrefix("MEMORA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvironment(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	globalConfig = &cfg
	return &cfg, nil
}

func loadDotEnv(path string) error {
	if err := godotenv.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("加载环境文件 %q 失败: %w", path, err)
	}
	return nil
}

// Get 返回全局配置实例，如果未初始化则触发panic
func Get() *Config {
	if globalConfig == nil {
		panic("configuration is not initialized")
	}
	return globalConfig
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "memora")
	v.SetDefault("app.version", "dev")
	v.SetDefault("app.mode", "debug")
	v.SetDefault("app.address", ":8080")
	v.SetDefault("app.read_timeout", "2m")
	v.SetDefault("app.write_timeout", "10m")
	v.SetDefault("app.shutdown_timeout", "10s")
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("redis.db", 3)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("minio.use_ssl", false)
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("document_consumer.enabled", true)
	v.SetDefault("document_consumer.stream", "memora:document:parse")
	v.SetDefault("document_consumer.group", "memora-api")
	v.SetDefault("document_consumer.concurrency", 2)
	v.SetDefault("document_consumer.block_timeout", "5s")
	v.SetDefault("document_consumer.processing_timeout", "15m")
	v.SetDefault("document_consumer.claim_idle", "20m")
	v.SetDefault("document_consumer.max_attempts", 3)
	v.SetDefault("outbox.poll_interval", "500ms")
	v.SetDefault("outbox.batch_size", 100)
	v.SetDefault("index_cleanup.enabled", true)
	v.SetDefault("index_cleanup.interval", "1h")
	v.SetDefault("index_cleanup.retention", 1)
	v.SetDefault("preview.enabled", true)
	v.SetDefault("preview.consumer.stream", "memora:document:preview")
	v.SetDefault("preview.consumer.group", "memora-preview")
	v.SetDefault("preview.consumer.concurrency", 2)
	v.SetDefault("preview.consumer.block_timeout", "5s")
	v.SetDefault("preview.consumer.processing_timeout", "10m")
	v.SetDefault("preview.consumer.claim_idle", "15m")
	v.SetDefault("preview.consumer.max_attempts", 3)
	v.SetDefault("preview.office.enabled", true)
	v.SetDefault("preview.office.max_concurrency", 1)
	v.SetDefault("preview.office.timeout", "5m")
	v.SetDefault("preview.xlsx.enabled", true)
	v.SetDefault("preview.xlsx.max_sheets", 100)
	v.SetDefault("preview.xlsx.max_rows_per_sheet", 100000)
	v.SetDefault("preview.xlsx.max_columns_per_sheet", 500)
	v.SetDefault("preview.xlsx.max_cells", 500000)
	v.SetDefault("preview.xlsx.max_uncompressed_bytes", 67108864)
	v.SetDefault("mcp.encryption_key", "")
	v.SetDefault("mcp.stdio_command_whitelist", []string{"npx", "python", "python3", "uvx", "node"})
	v.SetDefault("mcp.allow_local_http", false)
	v.SetDefault("document_parser.base_url", "http://127.0.0.1:5001")
	v.SetDefault("document_parser.auto_start", true)
	v.SetDefault("document_parser.auto_start_command", "uv")
	v.SetDefault("document_parser.auto_start_args", []string{"run", "--frozen", "--no-dev", "uvicorn", "app:app", "--host", "127.0.0.1", "--port", "5001"})
	v.SetDefault("document_parser.auto_start_working_directory", "./services/document-parser")
	v.SetDefault("document_parser.auto_start_timeout", "10m")
	v.SetDefault("document_parser.timeout", "8m")
	v.SetDefault("document_parser.max_response_size", 134217728)
	v.SetDefault("document_parser.max_file_bytes", 67108864)
	v.SetDefault("document_parser.max_asset_bytes", 33554432)
	v.SetDefault("document_parser.ocr_languages", []string{"zh", "en"})
	v.SetDefault("document_parser.do_ocr", true)
	v.SetDefault("document_parser.do_image_ocr", true)
	v.SetDefault("document_parser.table_structure", true)
	v.SetDefault("document_parser.extract_pictures", true)
	v.SetDefault("document_parser.include_bboxes", true)
	v.SetDefault("chunking.strategy", "auto")
	v.SetDefault("chunking.strategy_version", "structure-v1")
	v.SetDefault("chunking.enable_canonical_chunk_diff", false)
	v.SetDefault("chunking.max_tokens", 1000)
	v.SetDefault("chunking.min_tokens", 100)
	v.SetDefault("chunking.overlap_tokens", 100)
	v.SetDefault("chunking.repeat_table_header", true)
	v.SetDefault("asset_enrichment.mode", "none")
	v.SetDefault("asset_enrichment.timeout", "2m")
	v.SetDefault("url_import.timeout", "30s")
	v.SetDefault("url_import.max_response_bytes", 10485760)
	v.SetDefault("url_import.max_redirects", 5)
	v.SetDefault("ai.encryption_key", "")
	v.SetDefault("observability.enabled", true)
	v.SetDefault("observability.capture_sensitive_content", false)
	v.SetDefault("observability.trace_sample_ratio", 1.0)
	v.SetDefault("observability.retention_days", 30)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.filename", "")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 3)
	v.SetDefault("log.max_age", 28)
	v.SetDefault("log.compress", true)
	v.SetDefault("cors.enabled", true)
	v.SetDefault("cors.allow_origins", []string{"http://localhost:3000"})
	v.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allow_headers", []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "traceparent"})
	v.SetDefault("cors.expose_headers", []string{"X-Request-ID", "X-Trace-ID"})
	v.SetDefault("cors.max_age", 86400)
}

func bindEnvironment(v *viper.Viper) {
	bindings := map[string]string{
		"app.address": "MEMORA_HTTP_ADDRESS", "app.mode": "MEMORA_GIN_MODE",
		"app.read_timeout": "MEMORA_HTTP_READ_TIMEOUT", "app.write_timeout": "MEMORA_HTTP_WRITE_TIMEOUT",
		"app.shutdown_timeout": "MEMORA_HTTP_SHUTDOWN_TIMEOUT", "database.url": "MEMORA_DATABASE_URL",
		"database.max_idle_conns": "MEMORA_DATABASE_MAX_IDLE", "database.max_open_conns": "MEMORA_DATABASE_MAX_OPEN",
		"redis.address": "MEMORA_REDIS_ADDRESS", "redis.password": "MEMORA_REDIS_PASSWORD", "redis.db": "MEMORA_REDIS_DB",
		"redis.pool_size": "MEMORA_REDIS_POOL_SIZE",
		"minio.endpoint":  "MEMORA_MINIO_ENDPOINT", "minio.access_key": "MEMORA_MINIO_ACCESS_KEY",
		"minio.secret_key": "MEMORA_MINIO_SECRET_KEY", "minio.bucket": "MEMORA_MINIO_BUCKET",
		"minio.use_ssl": "MEMORA_MINIO_USE_SSL", "jwt.secret": "MEMORA_JWT_SECRET", "jwt.access_ttl": "MEMORA_ACCESS_TTL",
		"document_consumer.enabled":                    "MEMORA_DOCUMENT_CONSUMER_ENABLED",
		"document_consumer.stream":                     "MEMORA_DOCUMENT_CONSUMER_STREAM",
		"document_consumer.group":                      "MEMORA_DOCUMENT_CONSUMER_GROUP",
		"document_consumer.concurrency":                "MEMORA_DOCUMENT_CONSUMER_CONCURRENCY",
		"document_consumer.block_timeout":              "MEMORA_DOCUMENT_CONSUMER_BLOCK_TIMEOUT",
		"document_consumer.processing_timeout":         "MEMORA_DOCUMENT_CONSUMER_PROCESSING_TIMEOUT",
		"document_consumer.claim_idle":                 "MEMORA_DOCUMENT_CONSUMER_CLAIM_IDLE",
		"document_consumer.max_attempts":               "MEMORA_DOCUMENT_CONSUMER_MAX_ATTEMPTS",
		"outbox.poll_interval":                         "MEMORA_OUTBOX_POLL_INTERVAL",
		"outbox.batch_size":                            "MEMORA_OUTBOX_BATCH_SIZE",
		"index_cleanup.enabled":                        "MEMORA_INDEX_CLEANUP_ENABLED",
		"index_cleanup.interval":                       "MEMORA_INDEX_CLEANUP_INTERVAL",
		"index_cleanup.retention":                      "MEMORA_INDEX_CLEANUP_RETENTION",
		"preview.enabled":                              "MEMORA_PREVIEW_ENABLED",
		"preview.consumer.stream":                      "MEMORA_PREVIEW_CONSUMER_STREAM",
		"preview.consumer.group":                       "MEMORA_PREVIEW_CONSUMER_GROUP",
		"preview.consumer.concurrency":                 "MEMORA_PREVIEW_CONSUMER_CONCURRENCY",
		"preview.consumer.block_timeout":               "MEMORA_PREVIEW_CONSUMER_BLOCK_TIMEOUT",
		"preview.consumer.processing_timeout":          "MEMORA_PREVIEW_CONSUMER_PROCESSING_TIMEOUT",
		"preview.consumer.claim_idle":                  "MEMORA_PREVIEW_CONSUMER_CLAIM_IDLE",
		"preview.consumer.max_attempts":                "MEMORA_PREVIEW_CONSUMER_MAX_ATTEMPTS",
		"preview.office.enabled":                       "MEMORA_PREVIEW_OFFICE_ENABLED",
		"preview.office.max_concurrency":               "MEMORA_PREVIEW_OFFICE_MAX_CONCURRENCY",
		"preview.office.timeout":                       "MEMORA_PREVIEW_OFFICE_TIMEOUT",
		"preview.xlsx.enabled":                         "MEMORA_PREVIEW_XLSX_ENABLED",
		"preview.xlsx.max_sheets":                      "MEMORA_PREVIEW_XLSX_MAX_SHEETS",
		"preview.xlsx.max_rows_per_sheet":              "MEMORA_PREVIEW_XLSX_MAX_ROWS_PER_SHEET",
		"preview.xlsx.max_columns_per_sheet":           "MEMORA_PREVIEW_XLSX_MAX_COLUMNS_PER_SHEET",
		"preview.xlsx.max_cells":                       "MEMORA_PREVIEW_XLSX_MAX_CELLS",
		"preview.xlsx.max_uncompressed_bytes":          "MEMORA_PREVIEW_XLSX_MAX_UNCOMPRESSED_BYTES",
		"document_parser.base_url":                     "MEMORA_DOCUMENT_PARSER_BASE_URL",
		"document_parser.auto_start":                   "MEMORA_DOCUMENT_PARSER_AUTO_START",
		"document_parser.auto_start_command":           "MEMORA_DOCUMENT_PARSER_AUTO_START_COMMAND",
		"document_parser.auto_start_working_directory": "MEMORA_DOCUMENT_PARSER_AUTO_START_WORKING_DIRECTORY",
		"document_parser.auto_start_timeout":           "MEMORA_DOCUMENT_PARSER_AUTO_START_TIMEOUT",
		"document_parser.timeout":                      "MEMORA_DOCUMENT_PARSER_TIMEOUT",
		"document_parser.max_response_size":            "MEMORA_DOCUMENT_PARSER_MAX_RESPONSE_SIZE",
		"document_parser.max_file_bytes":               "MEMORA_DOCUMENT_PARSER_MAX_FILE_BYTES",
		"document_parser.max_asset_bytes":              "MEMORA_DOCUMENT_PARSER_MAX_ASSET_BYTES",
		"document_parser.ocr_languages":                "MEMORA_DOCUMENT_PARSER_OCR_LANGUAGES",
		"document_parser.do_ocr":                       "MEMORA_DOCUMENT_PARSER_DO_OCR",
		"document_parser.do_image_ocr":                 "MEMORA_DOCUMENT_PARSER_DO_IMAGE_OCR",
		"document_parser.table_structure":              "MEMORA_DOCUMENT_PARSER_TABLE_STRUCTURE",
		"document_parser.extract_pictures":             "MEMORA_DOCUMENT_PARSER_EXTRACT_PICTURES",
		"document_parser.include_bboxes":               "MEMORA_DOCUMENT_PARSER_INCLUDE_BBOXES",
		"chunking.strategy":                            "MEMORA_CHUNKING_STRATEGY",
		"chunking.strategy_version":                    "MEMORA_CHUNKING_STRATEGY_VERSION",
		"chunking.enable_canonical_chunk_diff":         "MEMORA_CHUNKING_ENABLE_CANONICAL_CHUNK_DIFF",
		"chunking.max_tokens":                          "MEMORA_CHUNKING_MAX_TOKENS",
		"chunking.min_tokens":                          "MEMORA_CHUNKING_MIN_TOKENS",
		"chunking.overlap_tokens":                      "MEMORA_CHUNKING_OVERLAP_TOKENS",
		"chunking.repeat_table_header":                 "MEMORA_CHUNKING_REPEAT_TABLE_HEADER",
		"asset_enrichment.mode":                        "MEMORA_ASSET_ENRICHMENT_MODE",
		"asset_enrichment.timeout":                     "MEMORA_ASSET_ENRICHMENT_TIMEOUT",
		"url_import.timeout":                           "MEMORA_URL_IMPORT_TIMEOUT",
		"url_import.max_response_bytes":                "MEMORA_URL_IMPORT_MAX_RESPONSE_BYTES",
		"url_import.max_redirects":                     "MEMORA_URL_IMPORT_MAX_REDIRECTS",
		"ai.encryption_key":                            "MEMORA_AI_ENCRYPTION_KEY",
		"observability.enabled":                        "MEMORA_OBSERVABILITY_ENABLED",
		"observability.capture_sensitive_content":      "MEMORA_OBSERVABILITY_CAPTURE_SENSITIVE_CONTENT",
		"observability.trace_sample_ratio":             "MEMORA_OBSERVABILITY_TRACE_SAMPLE_RATIO",
		"observability.retention_days":                 "MEMORA_OBSERVABILITY_RETENTION_DAYS",
		"log.level":                                    "MEMORA_LOG_LEVEL", "log.filename": "MEMORA_LOG_FILENAME",
		"log.max_size": "MEMORA_LOG_MAX_SIZE", "log.max_backups": "MEMORA_LOG_MAX_BACKUPS",
		"log.max_age": "MEMORA_LOG_MAX_AGE", "log.compress": "MEMORA_LOG_COMPRESS",
		"mcp.encryption_key": "MEMORA_MCP_ENCRYPTION_KEY", "mcp.allow_local_http": "MEMORA_MCP_ALLOW_LOCAL_HTTP",
	}
	for key, environment := range bindings {
		_ = v.BindEnv(key, environment)
	}
}
