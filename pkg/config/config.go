package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config 是应用程序的完整配置结构，包含所有子模块的配置项
type Config struct {
	App             AppConfig             `mapstructure:"app"`
	Database        DatabaseConfig        `mapstructure:"database"`
	Redis           RedisConfig           `mapstructure:"redis"`
	MinIO           MinIOConfig           `mapstructure:"minio"`
	JWT             JWTConfig             `mapstructure:"jwt"`
	Worker          WorkerConfig          `mapstructure:"worker"`
	MCP             MCPConfig             `mapstructure:"mcp"`
	Log             LogConfig             `mapstructure:"log"`
	CORS            CORSConfig            `mapstructure:"cors"`
	DocumentParser  DocumentParserConfig  `mapstructure:"document_parser"`
	Chunking        ChunkingConfig        `mapstructure:"chunking"`
	AssetEnrichment AssetEnrichmentConfig `mapstructure:"asset_enrichment"`
}

// AppConfig 定义应用程序基础配置，包括名称、版本、运行模式和超时设置
type AppConfig struct {
	Name            string        `mapstructure:"name"`
	Version         string        `mapstructure:"version"`
	Mode            string        `mapstructure:"mode"`
	Address         string        `mapstructure:"address"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig 定义PostgreSQL数据库连接配置
type DatabaseConfig struct {
	URL          string `mapstructure:"url"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// RedisConfig 定义Redis连接配置
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// MinIOConfig 定义MinIO对象存储连接配置
type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// JWTConfig 定义JWT令牌配置，包括密钥和访问令牌有效期
type JWTConfig struct {
	Secret    string        `mapstructure:"secret"`
	AccessTTL time.Duration `mapstructure:"access_ttl"`
}

// WorkerConfig 定义后台Worker任务配置，包括并发数、轮询间隔和超时设置
type WorkerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	MaxRetryDelay  time.Duration `mapstructure:"max_retry_delay"`
	IdempotencyTTL time.Duration `mapstructure:"idempotency_ttl"`
}

// MCPConfig 是 MCP 导入与调用的安全配置。
type MCPConfig struct {
	EncryptionKey         string   `mapstructure:"encryption_key"`
	StdioCommandWhitelist []string `mapstructure:"stdio_command_whitelist"`
	AllowLocalHTTP        bool     `mapstructure:"allow_local_http"`
}

// LogConfig 定义日志系统配置，包括日志级别、文件输出和滚动策略
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// CORSConfig 定义跨域资源共享配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// DocumentParserConfig 定义 Python document-parser 客户端与解析选项。
// 注意：本段配置不包含任何 Chunk 参数。
type DocumentParserConfig struct {
	// BaseURL 是 Python 服务地址；空表示未配置（PDF/DOCX 解析报错，TXT/MD 不受影响）。
	BaseURL string `mapstructure:"base_url"`
	// Timeout 是单次解析请求超时（必须小于 Worker 总超时）。
	Timeout time.Duration `mapstructure:"timeout"`
	// MaxResponseBytes 是解析响应体大小上限。
	MaxResponseBytes int64 `mapstructure:"max_response_size"`
	// MaxFileBytes 是原始文件大小上限（Python 侧限制一致或更严格）。
	MaxFileBytes int64 `mapstructure:"max_file_bytes"`
	// MaxAssetBytes 是单张图片大小上限。
	MaxAssetBytes int64 `mapstructure:"max_asset_bytes"`
	// OCRLanguages 是 OCR 语言列表。
	OCRLanguages []string `mapstructure:"ocr_languages"`
	// DoOCR 是否启用 OCR。
	DoOCR bool `mapstructure:"do_ocr"`
	// TableStructure 是否启用表格结构识别。
	TableStructure bool `mapstructure:"table_structure"`
	// ExtractPictures 是否提取图片。
	ExtractPictures bool `mapstructure:"extract_pictures"`
	// IncludeBBoxes 是否返回 bbox。
	IncludeBBoxes bool `mapstructure:"include_bboxes"`
}

// ChunkingConfig 定义 Go 分块策略配置（进入 chunk_config_hash，不进入 parse_config_hash）。
type ChunkingConfig struct {
	// StrategyVersion 是分块策略版本。
	StrategyVersion string `mapstructure:"strategy_version"`
	// MaxTokens 是单个 Chunk 的 token 上限。
	MaxTokens int `mapstructure:"max_tokens"`
	// MinTokens 是过短合并阈值。
	MinTokens int `mapstructure:"min_tokens"`
	// OverlapTokens 是长文本内部拆分重叠 token 数。
	OverlapTokens int `mapstructure:"overlap_tokens"`
	// RepeatTableHead 是大表子 Chunk 是否重复表头。
	RepeatTableHead bool `mapstructure:"repeat_table_header"`
}

// AssetEnrichmentConfig 定义图片资产增强配置（独立 config hash，不触发重新解析）。
type AssetEnrichmentConfig struct {
	// Mode 是增强模式：none（默认）/ 未来可扩展。
	Mode string `mapstructure:"mode"`
	// Timeout 是单次增强调用超时。
	Timeout time.Duration `mapstructure:"timeout"`
}

// Validate 校验所有配置项，收集所有错误后返回合并的错误信息
func (c Config) Validate() error {
	var errs []error
	if c.App.Address == "" {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_HTTP_ADDRESS"))
	}
	if err := c.ValidateDatabase(); err != nil {
		errs = append(errs, err)
	}
	if c.Redis.Address == "" {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_REDIS_ADDRESS"))
	}
	if c.MinIO.Endpoint == "" || c.MinIO.Bucket == "" {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_MINIO_ENDPOINT 和 MEMORA_MINIO_BUCKET"))
	}
	if c.MinIO.AccessKey == "" || c.MinIO.SecretKey == "" {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_MINIO_ACCESS_KEY 和 MEMORA_MINIO_SECRET_KEY"))
	}
	if c.JWT.Secret == "" || c.JWT.AccessTTL <= 0 {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_JWT_SECRET 且 MEMORA_ACCESS_TTL 必须为正数"))
	}
	if c.App.Mode == "release" && (len(c.JWT.Secret) < 32 || strings.HasPrefix(strings.ToLower(c.JWT.Secret), "change-me")) {
		errs = append(errs, errors.New("MEMORA_JWT_SECRET 至少需要 32 个字符，且在发布模式下不能使用示例值"))
	}
	if c.Worker.Concurrency <= 0 || c.Worker.PollInterval <= 0 || c.Worker.DefaultTimeout <= 0 || c.Worker.IdempotencyTTL <= 0 {
		errs = append(errs, errors.New("Worker 并发数和各项时长必须为正数"))
	}
	if c.App.Mode == "release" && len(c.MCP.EncryptionKey) < 32 {
		errs = append(errs, errors.New("MEMORA_MCP_ENCRYPTION_KEY must be at least 32 characters in release mode"))
	}
	if c.App.Mode != "debug" && c.App.Mode != "release" && c.App.Mode != "test" {
		errs = append(errs, errors.New("MEMORA_GIN_MODE 必须是 debug、release 或 test"))
	}
	for _, origin := range c.CORS.AllowOrigins {
		if c.CORS.AllowCredentials && origin == "*" {
			errs = append(errs, errors.New("CORS 通配符来源不能与凭据模式同时使用"))
		}
	}
	if c.Chunking.MaxTokens <= 0 || c.Chunking.MinTokens < 0 || c.Chunking.OverlapTokens < 0 {
		errs = append(errs, errors.New("chunking.max_tokens 必须为正数，min/overlap 不能为负"))
	}
	if c.Chunking.MinTokens > c.Chunking.MaxTokens {
		errs = append(errs, errors.New("chunking.min_tokens 不能大于 max_tokens"))
	}
	if c.DocumentParser.MaxFileBytes <= 0 || c.DocumentParser.MaxResponseBytes <= 0 || c.DocumentParser.MaxAssetBytes <= 0 {
		errs = append(errs, errors.New("document_parser 大小限制必须为正数"))
	}
	if c.DocumentParser.Timeout <= 0 {
		errs = append(errs, errors.New("document_parser.timeout 必须为正数"))
	}
	if c.AssetEnrichment.Mode != "" && c.AssetEnrichment.Mode != "none" {
		errs = append(errs, errors.New("asset_enrichment.mode 仅支持 none"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("配置无效: %w", errors.Join(errs...))
	}
	return nil
}

// ValidateDatabase 校验数据库相关配置项
func (c Config) ValidateDatabase() error {
	var errs []error
	if c.Database.URL == "" {
		errs = append(errs, errors.New("缺少环境变量 MEMORA_DATABASE_URL"))
	}
	if c.Database.MaxIdleConns < 0 || c.Database.MaxOpenConns <= 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, errors.New("数据库连接池配置无效"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("数据库配置无效: %w", errors.Join(errs...))
	}
	return nil
}
