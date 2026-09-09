package database

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type gormSpanContextKey struct{}

type gormSpanState struct {
	parent context.Context
	span   trace.Span
}

// registerGormTracing 在统一数据库入口记录低敏 Span。它只采集操作类型、表名、行数与错误，
// 不记录 SQL 文本、绑定参数或查询结果。
func registerGormTracing(db *gorm.DB) error {
	return errors.Join(
		db.Callback().Create().Before("gorm:create").Register("memora:otel_before_create", beginGormSpan("create")),
		db.Callback().Create().After("gorm:create").Register("memora:otel_after_create", endGormSpan),
		db.Callback().Query().Before("gorm:query").Register("memora:otel_before_query", beginGormSpan("query")),
		db.Callback().Query().After("gorm:query").Register("memora:otel_after_query", endGormSpan),
		db.Callback().Update().Before("gorm:update").Register("memora:otel_before_update", beginGormSpan("update")),
		db.Callback().Update().After("gorm:update").Register("memora:otel_after_update", endGormSpan),
		db.Callback().Delete().Before("gorm:delete").Register("memora:otel_before_delete", beginGormSpan("delete")),
		db.Callback().Delete().After("gorm:delete").Register("memora:otel_after_delete", endGormSpan),
		db.Callback().Row().Before("gorm:row").Register("memora:otel_before_row", beginGormSpan("row")),
		db.Callback().Row().After("gorm:row").Register("memora:otel_after_row", endGormSpan),
		db.Callback().Raw().Before("gorm:raw").Register("memora:otel_before_raw", beginGormSpan("raw")),
		db.Callback().Raw().After("gorm:raw").Register("memora:otel_after_raw", endGormSpan),
	)
}

func beginGormSpan(operation string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Context == nil {
			return
		}
		parent := tx.Statement.Context
		ctx, span := otel.Tracer("github.com/1090-f/Memora/database").Start(
			parent,
			"db."+operation,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system.name", "postgresql"),
				attribute.String("db.operation.name", operation),
				attribute.String("db.collection.name", tx.Statement.Table),
			),
		)
		tx.Statement.Context = context.WithValue(ctx, gormSpanContextKey{}, gormSpanState{parent: parent, span: span})
	}
}

func endGormSpan(tx *gorm.DB) {
	if tx == nil || tx.Statement == nil || tx.Statement.Context == nil {
		return
	}
	state, ok := tx.Statement.Context.Value(gormSpanContextKey{}).(gormSpanState)
	if !ok {
		return
	}
	state.span.SetAttributes(attribute.Int64("db.response.returned_rows", tx.RowsAffected))
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		state.span.RecordError(tx.Error)
		state.span.SetStatus(codes.Error, "database operation failed")
	}
	state.span.End()
	tx.Statement.Context = state.parent
}
