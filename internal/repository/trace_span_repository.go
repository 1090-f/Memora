package repository

import (
	"context"
	"encoding/json"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// TraceSpanRepository 提供应用内 Trace Explorer 的只读查询。
type TraceSpanRepository interface {
	ListByRunTrace(ctx context.Context, traceID, runID string) ([]entity.TraceSpan, error)
}

type traceSpanRepository struct{ db *gorm.DB }

func NewTraceSpanRepository(db *gorm.DB) TraceSpanRepository {
	return &traceSpanRepository{db: db}
}

func (r *traceSpanRepository) ListByRunTrace(ctx context.Context, traceID, runID string) ([]entity.TraceSpan, error) {
	var spans []entity.TraceSpan
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("started_at ASC, span_id ASC").
		Find(&spans).Error
	if err != nil {
		return nil, err
	}
	return spansForRun(spans, runID), nil
}

// spansForRun 只保留带目标 run_id 的锚点及其后代，避免客户端复用 traceparent
// 后读取同一 Trace ID 下属于其他运行的 Span。
func spansForRun(spans []entity.TraceSpan, runID string) []entity.TraceSpan {
	allowed := make(map[string]bool)
	for i := range spans {
		var attributes map[string]any
		if json.Unmarshal(spans[i].Attributes, &attributes) == nil && attributes["memora.run_id"] == runID {
			allowed[spans[i].SpanID] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for i := range spans {
			if allowed[spans[i].SpanID] || spans[i].ParentSpanID == nil || !allowed[*spans[i].ParentSpanID] {
				continue
			}
			allowed[spans[i].SpanID] = true
			changed = true
		}
	}
	result := make([]entity.TraceSpan, 0, len(allowed))
	for i := range spans {
		if allowed[spans[i].SpanID] {
			result = append(result, spans[i])
		}
	}
	return result
}

var _ TraceSpanRepository = (*traceSpanRepository)(nil)
