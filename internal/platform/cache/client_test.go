package cache_test

import (
	"testing"

	"github.com/1090-f/Memora/internal/platform/cache"
	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestOpenRejectsEmptyAddress(t *testing.T) {
	client, err := cache.Open(config.RedisConfig{})

	require.ErrorIs(t, err, cache.ErrInvalidAddress)
	require.Nil(t, client)
}
