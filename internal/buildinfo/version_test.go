package buildinfo_test

import (
	"testing"

	"github.com/1090-f/Memora/internal/buildinfo"
	"github.com/stretchr/testify/require"
)

func TestInfoHasServiceName(t *testing.T) {
	got := buildinfo.Info()

	require.Equal(t, "memora", got.Service)
	require.NotEmpty(t, got.Version)
}
