package audit

import (
	"context"
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// AuditEntry 表示一条审计事件记录。
type AuditEntry struct {
	Action    string
	ActorID   string
	Resource  string
	Outcome   string
	RequestID string
	TraceID   string
}

// Store 定义审计记录的持久化存储接口。
type Store interface {
	Insert(ctx context.Context, entry *AuditEntry) error
}

var store Store

// SetStore 注入审计记录的持久化实现，未注入时仅输出日志。
func SetStore(s Store) { store = s }

// Record 记录一条审计事件：先输出结构化日志，再异步持久化。
func Record(action, actorID, resource, requestID, traceID, outcome string) {
	logger.Info("审计事件",
		zap.String("audit_action", action), zap.String("actor_id", actorID),
		zap.String("resource", resource), zap.String("outcome", outcome),
		zap.String("request_id", requestID), zap.String("trace_id", traceID),
	)
	if store == nil {
		return
	}
	entry := &AuditEntry{
		Action: action, ActorID: actorID, Resource: resource, Outcome: outcome,
		RequestID: requestID, TraceID: traceID,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := store.Insert(ctx, entry); err != nil {
			logger.Error("审计事件持久化失败",
				zap.String("audit_action", action), zap.Error(err))
		}
	}()
}
