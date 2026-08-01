package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	MinIO    MinIOConfig    `mapstructure:"minio"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
}

type AppConfig struct {
	Name            string        `mapstructure:"name"`
	Version         string        `mapstructure:"version"`
	Mode            string        `mapstructure:"mode"`
	Address         string        `mapstructure:"address"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	URL          string `mapstructure:"url"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

type JWTConfig struct {
	Secret    string        `mapstructure:"secret"`
	AccessTTL time.Duration `mapstructure:"access_ttl"`
}

type WorkerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	MaxRetryDelay  time.Duration `mapstructure:"max_retry_delay"`
	IdempotencyTTL time.Duration `mapstructure:"idempotency_ttl"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

func (c Config) Validate() error {
	var errs []error
	if c.App.Address == "" {
		errs = append(errs, errors.New("MEMORA_HTTP_ADDRESS is required"))
	}
	if err := c.ValidateDatabase(); err != nil {
		errs = append(errs, err)
	}
	if c.Redis.Address == "" {
		errs = append(errs, errors.New("MEMORA_REDIS_ADDRESS is required"))
	}
	if c.MinIO.Endpoint == "" || c.MinIO.Bucket == "" {
		errs = append(errs, errors.New("MEMORA_MINIO_ENDPOINT and MEMORA_MINIO_BUCKET are required"))
	}
	if c.MinIO.AccessKey == "" || c.MinIO.SecretKey == "" {
		errs = append(errs, errors.New("MEMORA_MINIO_ACCESS_KEY and MEMORA_MINIO_SECRET_KEY are required"))
	}
	if c.JWT.Secret == "" || c.JWT.AccessTTL <= 0 {
		errs = append(errs, errors.New("MEMORA_JWT_SECRET and a positive MEMORA_ACCESS_TTL are required"))
	}
	if c.App.Mode == "release" && (len(c.JWT.Secret) < 32 || strings.HasPrefix(strings.ToLower(c.JWT.Secret), "change-me")) {
		errs = append(errs, errors.New("MEMORA_JWT_SECRET must be at least 32 characters and must not use an example value in release mode"))
	}
	if c.Worker.Concurrency <= 0 || c.Worker.PollInterval <= 0 || c.Worker.DefaultTimeout <= 0 || c.Worker.IdempotencyTTL <= 0 {
		errs = append(errs, errors.New("worker concurrency and durations must be positive"))
	}
	if c.App.Mode != "debug" && c.App.Mode != "release" && c.App.Mode != "test" {
		errs = append(errs, errors.New("MEMORA_GIN_MODE must be debug, release or test"))
	}
	for _, origin := range c.CORS.AllowOrigins {
		if c.CORS.AllowCredentials && origin == "*" {
			errs = append(errs, errors.New("CORS wildcard origin cannot be used with credentials"))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return nil
}

func (c Config) ValidateDatabase() error {
	var errs []error
	if c.Database.URL == "" {
		errs = append(errs, errors.New("MEMORA_DATABASE_URL is required"))
	}
	if c.Database.MaxIdleConns < 0 || c.Database.MaxOpenConns <= 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, errors.New("database connection pool settings are invalid"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid database configuration: %w", errors.Join(errs...))
	}
	return nil
}
