package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const insertTraceSpanSQL = `
INSERT INTO trace_spans (
    trace_id, span_id, parent_span_id, name, kind, status_code, status_message,
    started_at, ended_at, duration_ms, attributes, events, service_name,
    instrumentation_scope, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (trace_id, span_id) DO UPDATE SET
    parent_span_id = EXCLUDED.parent_span_id,
    name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    status_code = EXCLUDED.status_code,
    status_message = EXCLUDED.status_message,
    started_at = EXCLUDED.started_at,
    ended_at = EXCLUDED.ended_at,
    duration_ms = EXCLUDED.duration_ms,
    attributes = EXCLUDED.attributes,
    events = EXCLUDED.events,
    service_name = EXCLUDED.service_name,
    instrumentation_scope = EXCLUDED.instrumentation_scope`

// PostgresSpanExporter 将安全投影后的 Span 批量写入现有 PostgreSQL。
// 它直接使用 database/sql，避免 exporter 自身触发 GORM Trace 并产生递归 Span。
type PostgresSpanExporter struct{ db *sql.DB }

func NewPostgresSpanExporter(db *sql.DB) *PostgresSpanExporter {
	return &PostgresSpanExporter{db: db}
}

func (e *PostgresSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e == nil || e.db == nil || len(spans) == 0 {
		return nil
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始写入 Trace Span 失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, insertTraceSpanSQL)
	if err != nil {
		return fmt.Errorf("准备 Trace Span 写入失败: %w", err)
	}
	defer stmt.Close()
	for _, span := range spans {
		record, ok := projectSpan(span)
		if !ok {
			continue
		}
		if _, err := stmt.ExecContext(ctx, record.values()...); err != nil {
			return fmt.Errorf("写入 Trace Span 失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 Trace Span 写入失败: %w", err)
	}
	return nil
}

func (e *PostgresSpanExporter) Shutdown(context.Context) error { return nil }

type persistedSpan struct {
	traceID, spanID string
	parentSpanID    *string
	name, kind      string
	statusCode      string
	statusMessage   *string
	startedAt       time.Time
	endedAt         time.Time
	durationMS      int64
	attributes      []byte
	events          []byte
	serviceName     *string
	scope           *string
	createdAt       time.Time
}

func (s persistedSpan) values() []any {
	return []any{s.traceID, s.spanID, s.parentSpanID, s.name, s.kind, s.statusCode, s.statusMessage,
		s.startedAt, s.endedAt, s.durationMS, s.attributes, s.events, s.serviceName, s.scope, s.createdAt}
}

func projectSpan(span sdktrace.ReadOnlySpan) (persistedSpan, bool) {
	if span == nil || !span.SpanContext().IsValid() || span.EndTime().IsZero() {
		return persistedSpan{}, false
	}
	attributes := safeAttributes(span.Attributes())
	attributeJSON, _ := json.Marshal(attributes)
	events := make([]map[string]any, 0, len(span.Events()))
	for _, event := range span.Events() {
		events = append(events, map[string]any{
			"name":       truncateRunes(event.Name, 128),
			"timestamp":  event.Time.UTC(),
			"attributes": safeAttributes(event.Attributes),
		})
	}
	eventJSON, _ := json.Marshal(events)
	resourceAttributes := safeAttributes(span.Resource().Attributes())
	serviceName := optionalString(resourceAttributes["service.name"])
	scopeName := truncateRunes(span.InstrumentationScope().Name, 255)
	parentID := ""
	if span.Parent().SpanID().IsValid() {
		parentID = span.Parent().SpanID().String()
	}
	duration := span.EndTime().Sub(span.StartTime()).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	return persistedSpan{
		traceID: span.SpanContext().TraceID().String(), spanID: span.SpanContext().SpanID().String(),
		parentSpanID: optionalString(parentID), name: truncateRunes(span.Name(), 255), kind: span.SpanKind().String(),
		// Status Description 可能由第三方 instrumentation 直接填入底层错误文本，
		// 默认不持久化；用户可读错误继续由经过脱敏的业务事件提供。
		statusCode: span.Status().Code.String(), statusMessage: nil,
		startedAt: span.StartTime().UTC(), endedAt: span.EndTime().UTC(), durationMS: duration,
		attributes: attributeJSON, events: eventJSON, serviceName: serviceName, scope: optionalString(scopeName), createdAt: time.Now().UTC(),
	}, true
}

func safeAttributes(values []attribute.KeyValue) map[string]any {
	result := make(map[string]any)
	for _, item := range values {
		key := string(item.Key)
		if !safeAttributeKey(key) {
			continue
		}
		result[key] = item.Value.AsInterface()
	}
	return result
}

func safeAttributeKey(key string) bool {
	lower := strings.ToLower(key)
	for _, denied := range []string{"authorization", "cookie", "password", "secret", "credential", "api_key", "apikey", "prompt", "content", "statement"} {
		if strings.Contains(lower, denied) {
			return false
		}
	}
	for _, prefix := range []string{"service.", "http.", "url.", "server.", "network.", "db.", "rpc.", "messaging.", "gen_ai.", "error.", "memora."} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func optionalString(value any) *string {
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	return &text
}

func truncateRunes(value string, max int) string {
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

var _ sdktrace.SpanExporter = (*PostgresSpanExporter)(nil)
