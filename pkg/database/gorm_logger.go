package database

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger 将 GORM 日志桥接到 zap
type GormLogger struct {
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger 创建 GORM 日志适配器
func NewGormLogger() *GormLogger {
	return &GormLogger{
		level:         gormlogger.Warn, // 默认只显示 Warn 及以上
		slowThreshold: 500 * time.Millisecond,
	}
}

// LogMode 实现 gormlogger.Interface
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

// Info 实现 gormlogger.Interface
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level < gormlogger.Info {
		return
	}
	logger.Debug("[GORM] "+msg, zap.Any("args", data))
}

// Warn 实现 gormlogger.Interface
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level < gormlogger.Warn {
		return
	}
	logger.Warn("[GORM] "+msg, zap.Any("args", data))
}

// Error 实现 gormlogger.Interface
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level < gormlogger.Error {
		return
	}
	logger.Error("[GORM] "+msg, zap.Any("args", data))
}

// Trace 实现 gormlogger.Interface，记录 SQL 执行
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	sql, rows := fc()
	// 移除 SQL 中的 embedding 向量，避免日志噪音
	sql = removeEmbeddingVectors(sql)

	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
	}

	// 查询未命中是 First/Take 的正常业务结果，只降低日志级别；
	// 不修改 err，Repository 仍可按 gorm.ErrRecordNotFound 处理。
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if l.level >= gormlogger.Info {
			fields = append(fields, zap.String("sql", sql))
			logger.Debug("[GORM] 查询未命中", fields...)
		}
		return
	}

	if err != nil {
		// SQL 错误始终记录
		fields = append(fields, zap.Error(err))
		logger.Error("[GORM] SQL 执行错误", fields...)
		return
	}

	// 慢查询告警
	if l.slowThreshold > 0 && elapsed > l.slowThreshold {
		fields = append(fields, zap.String("sql", sql))
		logger.Warn("[GORM] 慢查询", fields...)
		return
	}

	// 正常查询记录为 DEBUG
	if l.level >= gormlogger.Info {
		fields = append(fields, zap.String("sql", sql))
		logger.Debug("[GORM] SQL", fields...)
	}
}

// embeddingVectorPattern 匹配 SQL 中的 embedding 向量
// 例如: '[-0.005081,0.013549,...]' 或 '[-0.005081,0.013549,...,0.016560]'
var embeddingVectorPattern = regexp.MustCompile(`'(\[[\s\-0-9,\.eE\+]+\])'`)

// removeEmbeddingVectors 移除 SQL 中的 embedding 向量，避免日志噪音
func removeEmbeddingVectors(sql string) string {
	return embeddingVectorPattern.ReplaceAllString(sql, "'<vector>'")
}
