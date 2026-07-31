package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

var ErrInvalidAddress = errors.New("redis address is required")

const pingTimeout = 5 * time.Second

// Open creates a Redis client and verifies that it is reachable before returning it.
func Open(cfg config.RedisConfig) (*redis.Client, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, ErrInvalidAddress
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
