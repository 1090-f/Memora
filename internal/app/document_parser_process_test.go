package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/1090-f/Memora/pkg/config"
)

func TestDocumentParserProcessStartsAndStopsManagedService(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	baseURL := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release local port: %v", err)
	}

	process := newDocumentParserProcess(config.DocumentParserConfig{
		BaseURL:                   baseURL,
		AutoStart:                 true,
		AutoStartCommand:          os.Args[0],
		AutoStartArgs:             []string{"-test.run=TestDocumentParserHelperProcess"},
		AutoStartWorkingDirectory: t.TempDir(),
		AutoStartTimeout:          5 * time.Second,
		AutoStartEnvironment: map[string]string{
			"MEMORA_TEST_PARSER_URL": baseURL,
		},
		MaxFileBytes:  1024,
		MaxAssetBytes: 1024,
	})
	if err := process.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !process.started {
		t.Fatal("Ensure() did not start the unavailable parser")
	}
	if err := process.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if err := process.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDocumentParserHelperProcess(t *testing.T) {
	rawURL := os.Getenv("MEMORA_TEST_PARSER_URL")
	if rawURL == "" {
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse helper URL: %v", err)
	}
	server := &http.Server{
		Addr: parsed.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		t.Fatalf("serve helper parser: %v", err)
	}
}

func TestDocumentParserProcessReusesReadyService(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	process := newDocumentParserProcess(config.DocumentParserConfig{
		BaseURL:                   server.URL,
		AutoStart:                 true,
		AutoStartCommand:          "command-that-must-not-run",
		AutoStartWorkingDirectory: ".",
		AutoStartTimeout:          time.Second,
	})
	if err := process.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if process.started {
		t.Fatal("Ensure() started a child process even though the parser was ready")
	}
}

func TestDocumentParserProcessRejectsRemoteManagedService(t *testing.T) {
	t.Parallel()
	process := newDocumentParserProcess(config.DocumentParserConfig{
		BaseURL:                   "https://example.com:5001",
		AutoStart:                 true,
		AutoStartCommand:          "uv",
		AutoStartWorkingDirectory: ".",
		AutoStartTimeout:          time.Second,
	})
	if err := process.Ensure(context.Background()); err == nil {
		t.Fatal("Ensure() error = nil, want remote-address validation error")
	}
}

func TestDocumentParserProcessReportsModelInitializationFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/ready" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"error","detail":"model load failed"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	process := newDocumentParserProcess(config.DocumentParserConfig{
		BaseURL:                   server.URL,
		AutoStart:                 true,
		AutoStartCommand:          "command-that-must-not-run",
		AutoStartWorkingDirectory: ".",
		AutoStartTimeout:          time.Second,
	})
	if err := process.Ensure(context.Background()); err == nil {
		t.Fatal("Ensure() error = nil, want model initialization error")
	}
	if process.started {
		t.Fatal("Ensure() started a second parser after a model initialization failure")
	}
}
