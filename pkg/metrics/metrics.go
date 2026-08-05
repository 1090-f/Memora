package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type httpKey struct{ method, path, status string }
type workerKey struct{ jobType, result string }

var (
	httpRequests      sync.Map
	httpActive        atomic.Int64
	httpDurationNanos atomic.Int64
	httpDurationCount atomic.Uint64
	workerJobs        sync.Map
	workerDuration    atomic.Int64
	workerCount       atomic.Uint64
	workerHeartbeat   atomic.Int64
)

// HTTPStarted 记录一个新HTTP请求开始，增加活跃请求计数
func HTTPStarted() { httpActive.Add(1) }

// HTTPFinished 记录一个HTTP请求完成，更新请求数量和耗时统计
func HTTPFinished(method, path string, status int, duration time.Duration) {
	httpActive.Add(-1)
	httpDurationNanos.Add(duration.Nanoseconds())
	httpDurationCount.Add(1)
	value, _ := httpRequests.LoadOrStore(httpKey{method: method, path: path, status: strconv.Itoa(status)}, &atomic.Uint64{})
	value.(*atomic.Uint64).Add(1)
}

// WorkerFinished 记录一个Worker任务完成，更新任务数量和耗时统计
func WorkerFinished(jobType, result string, duration time.Duration) {
	workerDuration.Add(duration.Nanoseconds())
	workerCount.Add(1)
	value, _ := workerJobs.LoadOrStore(workerKey{jobType: jobType, result: result}, &atomic.Uint64{})
	value.(*atomic.Uint64).Add(1)
}

// WorkerHeartbeat 更新Worker最后一次心跳时间戳
func WorkerHeartbeat() { workerHeartbeat.Store(time.Now().UTC().Unix()) }

// Handler 返回Prometheus格式的指标导出HTTP处理器
func Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var lines []string
		httpRequests.Range(func(key, value any) bool {
			labels := key.(httpKey)
			lines = append(lines, fmt.Sprintf("memora_http_requests_total{method=%q,path=%q,status=%q} %d", labels.method, labels.path, labels.status, value.(*atomic.Uint64).Load()))
			return true
		})
		workerJobs.Range(func(key, value any) bool {
			labels := key.(workerKey)
			lines = append(lines, fmt.Sprintf("memora_worker_jobs_total{job_type=%q,result=%q} %d", labels.jobType, labels.result, value.(*atomic.Uint64).Load()))
			return true
		})
		sort.Strings(lines)
		lines = append(lines,
			fmt.Sprintf("memora_http_active_requests %d", httpActive.Load()),
			fmt.Sprintf("memora_http_request_duration_seconds_sum %.6f", float64(httpDurationNanos.Load())/float64(time.Second)),
			fmt.Sprintf("memora_http_request_duration_seconds_count %d", httpDurationCount.Load()),
			fmt.Sprintf("memora_worker_job_duration_seconds_sum %.6f", float64(workerDuration.Load())/float64(time.Second)),
			fmt.Sprintf("memora_worker_job_duration_seconds_count %d", workerCount.Load()),
			fmt.Sprintf("memora_worker_last_heartbeat_timestamp_seconds %d", workerHeartbeat.Load()),
		)
		_, _ = writer.Write([]byte(strings.Join(lines, "\n") + "\n"))
	})
}
