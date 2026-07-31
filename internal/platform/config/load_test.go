package config_test

import (
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	t.Setenv("MEMORA_DATABASE_URL", "")
	t.Setenv("MEMORA_JWT_SECRET", "")

	_, err := config.Load()

	require.ErrorContains(t, err, "MEMORA_DATABASE_URL")
}

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("MEMORA_DATABASE_URL", "postgres://memora:password@localhost:5432/memora")
	t.Setenv("MEMORA_JWT_SECRET", "test-jwt-secret")

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.HTTP.Address)
	require.Equal(t, 2*time.Hour, cfg.Auth.AccessTTL)
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("MEMORA_DATABASE_URL", "postgres://memora:password@localhost:5432/memora")
	t.Setenv("MEMORA_JWT_SECRET", "test-jwt-secret")
	t.Setenv("MEMORA_ACCESS_TTL", "not-a-duration")

	_, err := config.Load()

	require.ErrorContains(t, err, "MEMORA_ACCESS_TTL")
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("MEMORA_DATABASE_URL", "")
	t.Setenv("MEMORA_JWT_SECRET", "test-jwt-secret")

	_, err := config.Load()

	require.ErrorContains(t, err, "MEMORA_DATABASE_URL")
}

func TestLoadRejectsMissingJWTSecret(t *testing.T) {
	t.Setenv("MEMORA_DATABASE_URL", "postgres://memora:password@localhost:5432/memora")
	t.Setenv("MEMORA_JWT_SECRET", "")

	_, err := config.Load()

	require.ErrorContains(t, err, "MEMORA_JWT_SECRET")
}
