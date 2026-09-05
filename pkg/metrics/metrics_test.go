package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueueHealthExportsStalledGaugeWithLowCardinalityJobType(t *testing.T) {
	QueueHealth("document_process_test", 42, 3, 2)
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))

	body := recorder.Body.String()
	for _, want := range []string{
		`memora_worker_oldest_pending_age_seconds{job_type="document_process_test"} 42`,
		`memora_worker_retried_tasks{job_type="document_process_test"} 3`,
		`memora_worker_stalled_tasks{job_type="document_process_test"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q:\n%s", want, body)
		}
	}
}
