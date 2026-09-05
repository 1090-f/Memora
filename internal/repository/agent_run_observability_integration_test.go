package repository

import (
	"testing"

	"github.com/1090-f/Memora/internal/testutil"
)

func TestAgentRunObservabilityMigrationColumns(t *testing.T) {
	db := testutil.OpenRAGTestDB(t)
	columns := []string{
		"first_token_at",
		"first_token_latency_ms",
		"model_generate_duration_ms",
		"failure_stage",
		"retryable",
		"recovery_advice",
		"trace_parent_span_id",
		"trace_sampled",
	}
	for _, column := range columns {
		var count int64
		if err := db.Raw(`
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'agent_runs' AND column_name = ?
		`, column).Scan(&count).Error; err != nil {
			t.Fatalf("query column %s: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("agent_runs.%s missing after migrations", column)
		}
	}
}
