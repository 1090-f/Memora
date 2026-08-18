package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config 是应用程序的完整配置结构，包含所有子模块的配置项
type Config struct {
	App              AppConfig              `mapstructure:"app"`
	Database         DatabaseConfig         `mapstructure:"database"`
	Redis            RedisConfig            `mapstructure:"redis"`
	MinIO            MinIOConfig            `mapstructure:"minio"`
	JWT              JWTConfig              `mapstructure:"jwt"`
	DocumentConsumer DocumentConsumerConfig `mapstructure:"document_consumer"`
	Outbox           OutboxConfig           `mapstructure:"outbox"`
	IndexCleanup     IndexCleanupConfig     `mapstructure:"index_cleanup"`
	MCP              MCPConfig              `mapstructure:"mcp"`
	Log              LogConfig              `mapstructure:"log"`
	CORS             CORSConfig             `mapstructure:"cors"`
	DocumentParser   DocumentParserConfig   `mapstructure:"document_parser"`
	Chunking         ChunkingConfig         `mapstructure:"chunking"`
	AssetEnrichment  AssetEnrichmentConfig  `mapstructure:"asset_enrichment"`
	URLImport        URLImportConfig        `mapstructure:"url_import"`
	Preview          PreviewConfig          `mapstructure:"preview"`
	AI               AIConfig               `mapstructure:"ai"`
	Agent            AgentConfig            `mapstructure:"agent"`
	AgentWorker      AgentWorkerConfig      `mapstructure:"agent_worker"`
	AgentEvents      AgentEventsConfig      `mapstructure:"agent_events"`
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

// DocumentConsumerConfig 定义 API 进程内 Redis Stream 文档消费者。
type DocumentConsumerConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Stream            string        `mapstructure:"stream"`
	Group             string        `mapstructure:"group"`
	Concurrency       int           `mapstructure:"concurrency"`
	BlockTimeout      time.Duration `mapstructure:"block_timeout"`
	ProcessingTimeout time.Duration `mapstructure:"processing_timeout"`
	ClaimIdle         time.Duration `mapstructure:"claim_idle"`
	MaxAttempts       int           `mapstructure:"max_attempts"`
}

// OutboxConfig 定义数据库 Outbox 到 Redis Stream 的发布参数。
type OutboxConfig struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
	BatchSize    int           `mapstructure:"batch_size"`
}

// IndexCleanupConfig 定义旧索引版本与已删除文档索引数据的后台清理参数。
type IndexCleanupConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Interval  time.Duration `mapstructure:"interval"`
	Retention int           `mapstructure:"retention"` // 保留的旧版本数（0 表示只保留当前 active 版本）
}

// PreviewConfig 定义视觉预览的异步消费者与渲染资源上限。
type PreviewConfig struct {
	Enabled  bool                  `mapstructure:"enabled"`
	Consumer PreviewConsumerConfig `mapstructure:"consumer"`
	Office   OfficePreviewConfig   `mapstructure:"office"`
	XLSX     XLSXPreviewConfig     `mapstructure:"xlsx"`
}

type PreviewConsumerConfig struct {
	Stream            string        `mapstructure:"stream"`
	Group             string        `mapstructure:"group"`
	Concurrency       int           `mapstructure:"concurrency"`
	BlockTimeout      time.Duration `mapstructure:"block_timeout"`
	ProcessingTimeout time.Duration `mapstructure:"processing_timeout"`
	ClaimIdle         time.Duration `mapstructure:"claim_idle"`
	MaxAttempts       int           `mapstructure:"max_attempts"`
}

type OfficePreviewConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	MaxConcurrency int           `mapstructure:"max_concurrency"`
	Timeout        time.Duration `mapstructure:"timeout"`
}

type XLSXPreviewConfig struct {
	Enabled              bool  `mapstructure:"enabled"`
	MaxSheets            int   `mapstructure:"max_sheets"`
	MaxRowsPerSheet      int   `mapstructure:"max_rows_per_sheet"`
	MaxColumnsPerSheet   int   `mapstructure:"max_columns_per_sheet"`
	MaxCells             int   `mapstructure:"max_cells"`
	MaxUncompressedBytes int64 `mapstructure:"max_uncompressed_bytes"`
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
	// AutoStart 仅用于本地开发：主程序在服务未就绪时托管启动解析进程。
	AutoStart bool `mapstructure:"auto_start"`
	// AutoStartCommand/Args 是直接执行的命令与参数，不经过 shell。
	AutoStartCommand string   `mapstructure:"auto_start_command"`
	AutoStartArgs    []string `mapstructure:"auto_start_args"`
	// AutoStartWorkingDirectory 是解析服务工作目录。
	AutoStartWorkingDirectory string `mapstructure:"auto_start_working_directory"`
	// AutoStartTimeout 是首次依赖同步、模型初始化与健康检查的总等待时间。
	AutoStartTimeout time.Duration `mapstructure:"auto_start_timeout"`
	// AutoStartEnvironment 是追加到子进程的环境变量。
	AutoStartEnvironment map[string]string `mapstructure:"auto_start_environment"`
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
	// DoImageOCR 是否对文档内提取的图片做二级 OCR（区别于整页/整图文档的 DoOCR）。
	DoImageOCR bool `mapstructure:"do_image_ocr"`
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

// URLImportConfig 定义 Worker 网页抓取的 SSRF 相关资源上限。
type URLImportConfig struct {
	Timeout          time.Duration `mapstructure:"timeout"`
	MaxResponseBytes int64         `mapstructure:"max_response_bytes"`
	MaxRedirects     int           `mapstructure:"max_redirects"`
}

// AIConfig 保存模型 API Key 的应用级加密密钥。
type AIConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

// AgentConfig 定义 Agent 执行的配置。
type AgentConfig struct {
	MaxPlanSteps       int `mapstructure:"max_plan_steps"`        // 计划最大步骤数
	MaxReplans         int `mapstructure:"max_replans"`           // 允许重新规划的最大次数
	ReviewerRuns       int `mapstructure:"reviewer_runs"`         // 计划评审运行次数
	MaxToolCalls       int `mapstructure:"max_tool_calls"`        // 单次运行最大工具调用次数
	MaxToolResultBytes int `mapstructure:"max_tool_result_bytes"` // 工具结果最大字节数
	MaxRunSeconds      int `mapstructure:"max_run_seconds"`       // 单次运行最大时长（秒）
}

// AgentWorkerConfig 定义 Agent 异步 Worker 的执行参数。
type AgentWorkerConfig struct {
	Enabled    bool          `mapstructure:"enabled"`      // 是否启用异步 Worker
	PollPeriod time.Duration `mapstructure:"poll_period"`  // 数据库轮询间隔
	BatchSize  int           `mapstructure:"batch_size"`   // 每次轮询领取的最大运行数
	MaxRunTime time.Duration `mapstructure:"max_run_time"` // 单次运行最大执行时间
}

// AgentEventsConfig 定义 Agent 事件系统的配置。
type AgentEventsConfig struct {
	Channel       string `mapstructure:"channel"`         // Redis Pub/Sub 频道名称
	SubBufferSize int    `mapstructure:"sub_buffer_size"` // 订阅通道缓冲区大小
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
	if c.DocumentConsumer.Enabled && (c.DocumentConsumer.Stream == "" || c.DocumentConsumer.Group == "" || c.DocumentConsumer.Concurrency <= 0 || c.DocumentConsumer.BlockTimeout <= 0 || c.DocumentConsumer.ProcessingTimeout <= 0 || c.DocumentConsumer.ClaimIdle <= 0 || c.DocumentConsumer.MaxAttempts <= 0) {
		errs = append(errs, errors.New("document_consumer 配置无效"))
	}
	if c.DocumentConsumer.Enabled && c.DocumentConsumer.ClaimIdle <= c.DocumentConsumer.ProcessingTimeout {
		errs = append(errs, errors.New("document_consumer.claim_idle 必须大于 processing_timeout"))
	}
	if c.Outbox.PollInterval <= 0 || c.Outbox.BatchSize <= 0 {
		errs = append(errs, errors.New("outbox 配置无效"))
	}
	if c.Preview.Enabled {
		pc := c.Preview.Consumer
		if pc.Stream == "" || pc.Group == "" || pc.Concurrency <= 0 || pc.BlockTimeout <= 0 || pc.ProcessingTimeout <= 0 || pc.ClaimIdle <= pc.ProcessingTimeout || pc.MaxAttempts <= 0 {
			errs = append(errs, errors.New("preview.consumer 配置无效，claim_idle 必须大于 processing_timeout"))
		}
		if c.Preview.Office.Enabled && (c.Preview.Office.MaxConcurrency <= 0 || c.Preview.Office.Timeout <= 0) {
			errs = append(errs, errors.New("preview.office 配置无效"))
		}
		x := c.Preview.XLSX
		if x.Enabled && (x.MaxSheets <= 0 || x.MaxRowsPerSheet <= 0 || x.MaxColumnsPerSheet <= 0 || x.MaxCells <= 0 || x.MaxUncompressedBytes <= 0) {
			errs = append(errs, errors.New("preview.xlsx 资源上限必须为正数"))
		}
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
	if c.DocumentParser.AutoStart && (strings.TrimSpace(c.DocumentParser.AutoStartCommand) == "" || strings.TrimSpace(c.DocumentParser.AutoStartWorkingDirectory) == "" || c.DocumentParser.AutoStartTimeout <= 0) {
		errs = append(errs, errors.New("document_parser 自动启动配置无效"))
	}
	if c.AssetEnrichment.Mode != "" && c.AssetEnrichment.Mode != "none" {
		errs = append(errs, errors.New("asset_enrichment.mode 仅支持 none"))
	}
	if c.URLImport.Timeout <= 0 || c.URLImport.MaxResponseBytes <= 0 || c.URLImport.MaxRedirects <= 0 || c.URLImport.MaxRedirects > 10 {
		errs = append(errs, errors.New("url_import 配置无效"))
	}
	if c.App.Mode == "release" && (len(c.AI.EncryptionKey) < 16 || strings.HasPrefix(strings.ToLower(c.AI.EncryptionKey), "change-me")) {
		errs = append(errs, errors.New("MEMORA_AI_ENCRYPTION_KEY 在发布模式下至少需要 16 个字符且不能使用示例值"))
	}
	if c.Agent.MaxPlanSteps <= 0 {
		c.Agent.MaxPlanSteps = 5 // 默认最大步骤数
	}
	if c.Agent.MaxReplans < 0 {
		c.Agent.MaxReplans = 1 // 默认最大重规划次数
	}
	if c.Agent.MaxToolCalls <= 0 {
		c.Agent.MaxToolCalls = 10 // 默认最大工具调用次数
	}
	if c.Agent.MaxToolResultBytes <= 0 {
		c.Agent.MaxToolResultBytes = 1048576 // 默认 1MB
	}
	if c.Agent.MaxRunSeconds <= 0 {
		c.Agent.MaxRunSeconds = 300 // 默认 5 分钟
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
