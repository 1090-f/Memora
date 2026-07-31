package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func Load() (Config, error) {
	accessTTL, err := durationFromEnv("MEMORA_ACCESS_TTL", 2*time.Hour)
	if err != nil {
		return Config{}, err
	}

	readTimeout, err := durationFromEnv("MEMORA_HTTP_READ_TIMEOUT", 0)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := durationFromEnv("MEMORA_HTTP_WRITE_TIMEOUT", 0)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationFromEnv("MEMORA_HTTP_SHUTDOWN_TIMEOUT", 0)
	if err != nil {
		return Config{}, err
	}
	maxOpen, err := intFromEnv("MEMORA_DATABASE_MAX_OPEN", 0)
	if err != nil {
		return Config{}, err
	}
	maxIdle, err := intFromEnv("MEMORA_DATABASE_MAX_IDLE", 0)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := intFromEnv("MEMORA_REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	minIOUseSSL, err := boolFromEnv("MEMORA_MINIO_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment: stringFromEnv("MEMORA_ENVIRONMENT", ""),
		HTTP: HTTPConfig{
			Address:         stringFromEnv("MEMORA_HTTP_ADDRESS", ":8080"),
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			URL:     stringFromEnv("MEMORA_DATABASE_URL", ""),
			MaxOpen: maxOpen,
			MaxIdle: maxIdle,
		},
		Redis: RedisConfig{
			Address:  stringFromEnv("MEMORA_REDIS_ADDRESS", ""),
			Password: stringFromEnv("MEMORA_REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		MinIO: MinIOConfig{
			Endpoint:  stringFromEnv("MEMORA_MINIO_ENDPOINT", ""),
			AccessKey: stringFromEnv("MEMORA_MINIO_ACCESS_KEY", ""),
			SecretKey: stringFromEnv("MEMORA_MINIO_SECRET_KEY", ""),
			Bucket:    stringFromEnv("MEMORA_MINIO_BUCKET", ""),
			UseSSL:    minIOUseSSL,
		},
		Auth: AuthConfig{
			JWTSecret: stringFromEnv("MEMORA_JWT_SECRET", ""),
			AccessTTL: accessTTL,
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func stringFromEnv(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

func durationFromEnv(name string, defaultValue time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return duration, nil
}

func intFromEnv(name string, defaultValue int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func boolFromEnv(name string, defaultValue bool) (bool, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}
