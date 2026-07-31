package config

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	Environment string
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	MinIO       MinIOConfig
	Auth        AuthConfig
}

type HTTPConfig struct {
	Address                                    string
	ReadTimeout, WriteTimeout, ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL              string
	MaxOpen, MaxIdle int
}

type RedisConfig struct {
	Address, Password string
	DB                int
}

type MinIOConfig struct {
	Endpoint, AccessKey, SecretKey, Bucket string
	UseSSL                                 bool
}

type AuthConfig struct {
	JWTSecret string
	AccessTTL time.Duration
}

func (c Config) Validate() error {
	var errs []error
	if c.Database.URL == "" {
		errs = append(errs, errors.New("MEMORA_DATABASE_URL is required"))
	}
	if c.Auth.JWTSecret == "" {
		errs = append(errs, errors.New("MEMORA_JWT_SECRET is required"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return nil
}
