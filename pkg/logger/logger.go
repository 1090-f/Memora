package logger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/1090-f/Memora/pkg/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log          = zap.NewNop()
	sugar        = log.Sugar()
	outputCloser io.Closer
)

// Init 初始化日志系统
func Init(cfg *config.LogConfig) error {
	if cfg == nil {
		return errors.New("日志配置不能为空")
	}
	if err := ensureLogDir(cfg.Filename); err != nil {
		return fmt.Errorf("创建日志目录: %w", err)
	}
	if err := closeOutput(); err != nil {
		return fmt.Errorf("关闭旧日志输出: %w", err)
	}

	level := parseLevel(cfg.Level)
	cores := []zapcore.Core{
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(newConsoleEncoderConfig()),
			zapcore.AddSync(os.Stdout),
			level,
		),
	}

	if cfg.Filename != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		outputCloser = fileWriter
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(newJSONEncoderConfig()),
			zapcore.AddSync(fileWriter),
			level,
		))
	}

	log = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))
	sugar = log.Sugar()
	return nil
}

func newBaseEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func newConsoleEncoderConfig() zapcore.EncoderConfig {
	cfg := newBaseEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.ConsoleSeparator = "  "
	return cfg
}

func newJSONEncoderConfig() zapcore.EncoderConfig {
	cfg := newBaseEncoderConfig()
	cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	return cfg
}

func parseLevel(raw string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func ensureLogDir(filename string) error {
	if filename == "" {
		return nil
	}
	dir := filepath.Dir(filename)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func closeOutput() error {
	if outputCloser == nil {
		return nil
	}
	err := outputCloser.Close()
	outputCloser = nil
	return err
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	log.Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	log.Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	log.Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

// Fatal 致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	log.Fatal(msg, fields...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	sugar.Debugf(format, args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	sugar.Infof(format, args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	sugar.Warnf(format, args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	sugar.Fatalf(format, args...)
}

// With 创建带字段的 logger
func With(fields ...zap.Field) *zap.Logger {
	return log.With(fields...)
}

// Sync 同步日志缓冲区
func Sync() error {
	if log == nil {
		return nil
	}
	_ = log.Sync()
	return nil
}

// GetLogger 获取原始 logger
func GetLogger() *zap.Logger {
	return log
}

// GetSugaredLogger 获取 sugared logger
func GetSugaredLogger() *zap.SugaredLogger {
	return sugar
}
