package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestEnvExampleContainsCompleteRedisConfiguration(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	examplePath := filepath.Join(filepath.Dir(currentFile), "..", "..", ".env.example")
	values, err := godotenv.Read(examplePath)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"MEMORA_REDIS_ADDRESS":   "localhost:6379",
		"MEMORA_REDIS_PASSWORD":  "",
		"MEMORA_REDIS_DB":        "0",
		"MEMORA_REDIS_POOL_SIZE": "10",
	}
	for key, expected := range want {
		if actual, exists := values[key]; !exists || actual != expected {
			t.Errorf("%s = %q, exists = %v; want %q", key, actual, exists, expected)
		}
	}
}

func TestLoadDotEnvMissingFileIsOptional(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("missing .env should be optional: %v", err)
	}
}

func TestLoadDotEnvRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("BROKEN='unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err == nil || !strings.Contains(err.Error(), "load environment file") {
		t.Fatalf("expected a contextual parse error, got %v", err)
	}
}

func TestLoadDatabaseUsesDotEnvOverYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestConfig(t, dir, "postgres://yaml-value")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MEMORA_DATABASE_URL=postgres://env-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsetForTest(t, "MEMORA_DATABASE_URL")

	cfg, err := LoadDatabase(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.URL != "postgres://env-value" {
		t.Fatalf("database URL = %q, want .env value", cfg.Database.URL)
	}
}

func TestLoadDatabaseKeepsSystemEnvironmentOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestConfig(t, dir, "postgres://yaml-value")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MEMORA_DATABASE_URL=postgres://env-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEMORA_DATABASE_URL", "postgres://system-value")

	cfg, err := LoadDatabase(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.URL != "postgres://system-value" {
		t.Fatalf("database URL = %q, want system environment value", cfg.Database.URL)
	}
}

func writeTestConfig(t *testing.T, dir, databaseURL string) {
	t.Helper()
	content := "database:\n  url: \"" + databaseURL + "\"\n  max_idle_conns: 1\n  max_open_conns: 2\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func unsetForTest(t *testing.T, key string) {
	t.Helper()
	value, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
