package audit

import (
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

func Record(action, actorID, resource, requestID, traceID, outcome string) {
	logger.Info("audit event",
		zap.String("audit_action", action), zap.String("actor_id", actorID),
		zap.String("resource", resource), zap.String("outcome", outcome),
		zap.String("request_id", requestID), zap.String("trace_id", traceID),
	)
}
