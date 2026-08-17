package logger

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestConsoleEncoderFormat(t *testing.T) {
	entry := testEntry()
	buffer, err := zapcore.NewConsoleEncoder(newConsoleEncoderConfig()).EncodeEntry(entry, []zap.Field{
		zap.String("request_id", "request-1"),
	})
	require.NoError(t, err)
	defer buffer.Free()

	line := buffer.String()
	require.Contains(t, line, "2026-08-15 10:20:30")
	require.Contains(t, line, "\x1b[34mINFO\x1b[0m")
	require.Contains(t, line, "  HTTP Request  ")
	require.Contains(t, line, `"request_id": "request-1"`)
	require.NotContains(t, line, "2026-08-15T10:20:30")
}

func TestJSONEncoderFormat(t *testing.T) {
	entry := testEntry()
	buffer, err := zapcore.NewJSONEncoder(newJSONEncoderConfig()).EncodeEntry(entry, []zap.Field{
		zap.String("request_id", "request-1"),
	})
	require.NoError(t, err)
	defer buffer.Free()

	var record map[string]any
	require.NoError(t, json.Unmarshal(buffer.Bytes(), &record))
	require.Equal(t, "2026-08-15 10:20:30", record["time"])
	require.Equal(t, "info", record["level"])
	require.Equal(t, "HTTP Request", record["msg"])
	require.Equal(t, "request-1", record["request_id"])
}

func TestParseLevel(t *testing.T) {
	require.Equal(t, zapcore.DebugLevel, parseLevel(" DEBUG "))
	require.Equal(t, zapcore.WarnLevel, parseLevel("warn"))
	require.Equal(t, zapcore.ErrorLevel, parseLevel("error"))
	require.Equal(t, zapcore.InfoLevel, parseLevel("unknown"))
}

func testEntry() zapcore.Entry {
	return zapcore.Entry{
		Time:       time.Date(2026, time.August, 15, 10, 20, 30, 0, time.FixedZone("CST", 8*60*60)),
		Level:      zapcore.InfoLevel,
		Message:    "HTTP Request",
		LoggerName: "test",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "internal/middleware/logger.go",
			Line:    16,
		},
	}
}
