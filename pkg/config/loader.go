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
	v.SetDefault("app.read_timeout", "15s")
	v.SetDefault("app.write_timeout", "15s")
	v.SetDefault("app.shutdown_timeout", "10s")
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("minio.use_ssl", false)
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("worker.concurrency", 4)
	v.SetDefault("worker.poll_interval", "2s")
	v.SetDefault("worker.default_timeout", "5m")
	v.SetDefault("worker.max_retry_delay", "1m")
	v.SetDefault("worker.idempotency_ttl", "24h")
	v.SetDefault("mcp.encryption_key", "")
	v.SetDefault("mcp.stdio_command_whitelist", []string{"npx", "python", "python3", "uvx", "node"})
	v.SetDefault("mcp.allow_local_http", false)
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
		"worker.concurrency": "MEMORA_WORKER_CONCURRENCY", "worker.poll_interval": "MEMORA_WORKER_POLL_INTERVAL",
		"worker.default_timeout": "MEMORA_WORKER_DEFAULT_TIMEOUT", "worker.max_retry_delay": "MEMORA_WORKER_MAX_RETRY_DELAY",
		"worker.idempotency_ttl": "MEMORA_WORKER_IDEMPOTENCY_TTL",
		"log.level":              "MEMORA_LOG_LEVEL", "log.filename": "MEMORA_LOG_FILENAME",
		"log.max_size": "MEMORA_LOG_MAX_SIZE", "log.max_backups": "MEMORA_LOG_MAX_BACKUPS",
		"log.max_age": "MEMORA_LOG_MAX_AGE", "log.compress": "MEMORA_LOG_COMPRESS",
		"mcp.encryption_key": "MEMORA_MCP_ENCRYPTION_KEY", "mcp.allow_local_http": "MEMORA_MCP_ALLOW_LOCAL_HTTP",
	}
	for key, environment := range bindings {
		_ = v.BindEnv(key, environment)
	}
}
