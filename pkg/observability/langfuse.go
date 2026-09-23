package observability

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// langfuseStatsInterval 是导出统计日志的输出间隔。
const langfuseStatsInterval = time.Minute

// maxReportedSpanNames 限制「首次出现即打日志」的名字数量，防止异常情况下 map 无界增长。
const maxReportedSpanNames = 64

// AttachLangfuseSpanExporter 将 Langfuse OTLP 导出器注册到当前 Provider。
// 与内置 Postgres 导出器并行，互不影响；任一导出失败不影响另一条链路。
func AttachLangfuseSpanExporter(ctx context.Context, cfg config.LangfuseConfig) (func(context.Context) error, error) {
	provider, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil, fmt.Errorf("当前 OpenTelemetry Provider 不支持注册 Span Processor")
	}

	// 用 WithEndpointURL 接受完整 URL（scheme + host + port + path），内部自动拆分，
	// scheme 直接决定明文（http）还是 TLS（https），无需额外的 insecure 开关。
	// ⚠️ url.Parse 失败时它只静默保留默认值 localhost:4318、不返回 error，
	//    所以非法输入必须由 config.Validate() 提前拦下。
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.Host),
		// 顺序要紧：WithEndpointURL 会先把 host 里可能带的 path 写进 URLPath，
		// 这一行在其后执行，才能把路径定死为 Langfuse 的 OTLP 端点。
		otlptracehttp.WithURLPath("/api/public/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                basicAuth(cfg.PublicKey, cfg.SecretKey),
			"x-langfuse-ingestion-version": "4",
		}),
		otlptracehttp.WithTimeout(10 * time.Second),
		// 收敛重试窗口：默认 MaxElapsedTime 是 1 分钟，会拖慢 provider.Shutdown()；
		// WithTimeout 只管单次请求，管不住这个总时长。
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: time.Second,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  5 * time.Second,
		}),
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("创建 Langfuse OTLP 导出器失败: %w", err)
	}

	// 过滤器恒开 —— 见 langfuseSpanFilter 的注释说明为什么不留开关。
	filter := newLangfuseSpanFilter(sdktrace.NewBatchSpanProcessor(exporter,
		sdktrace.WithBatchTimeout(2*time.Second), // 对 Cloud 比 1s 更稳（速率限制）
		sdktrace.WithMaxExportBatchSize(128),
		sdktrace.WithMaxQueueSize(2048),
	))
	provider.RegisterSpanProcessor(filter)
	filter.startStatsReporter()

	logger.Info("[Langfuse] Span 导出已挂载：仅导出 Agent 执行链路，其余 Span 在进程内丢弃",
		zap.String("host", cfg.Host),
		zap.Bool("capture_content", cfg.CaptureContent),
		zap.Strings("allow_prefixes", langfuseAllowPrefixes),
	)

	return filter.Shutdown, nil
}

func basicAuth(publicKey, secretKey string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(publicKey+":"+secretKey))
}

// langfuseSpanFilter 只放行 Agent 执行链路的 Span，其余一律在进程内丢弃，不产生任何 OTLP 流量。
//
// 为什么必须过滤：Langfuse 是用来「看 Agent 工作流」的，而 Memora 的 Span 体系里有海量内部噪声——
// GORM tracing 给每次 DB 操作都建一个 db.* Span（pkg/database/gorm_tracing.go），
// 加上前端轮询的 GET /api/v1/*、探活请求。实测不过滤时 Langfuse 里积累了 1w+ 条 db.query
// 独立 trace，真正的 agent run 被彻底淹没，点开也看不出流程。
//
// 过滤后一条 trace = 一次 Agent run，名字为 run_id，结构如下：
//
//	queue.wait
//	└─ agent.run
//	   ├─ context.build
//	   ├─ agent.route
//	   └─ agent.react
//	      ├─ chat <model>     ← generation：prompt / 补全 / token 用量
//	      └─ tool.<name>      ← 含入参 + 返回结果
//
// ⚠️ 该过滤**恒开，不受任何配置控制**（此前它挂在 filter_probe_requests 开关下，
// 一旦那个开关被关掉或配置没读到，Langfuse 就退化成 Span 垃圾场，而页面照样有数据、
// 只是全是废的 —— 这种失效是静默的，极难排查，所以直接不留开关）。
// 运行态是否生效由日志自证：见 startStatsReporter 与 reportOnce。
type langfuseSpanFilter struct {
	next sdktrace.SpanProcessor

	kept    atomic.Int64 // 当前统计窗口内放行数
	dropped atomic.Int64 // 当前统计窗口内丢弃数

	seenMu      sync.Mutex
	seenKept    map[string]struct{} // 已打过日志的放行 Span 名
	seenDropped map[string]struct{} // 已打过日志的丢弃 Span 名

	stopOnce sync.Once
	stop     chan struct{}
}

// langfuseAllowPrefixes 是放行白名单（按 Span 名前缀匹配）。
// 白名单对应全项目 Span 名字清单，新增埋点时如果不在其中，需要同步补进来 ——
// 运行日志里出现「丢弃非 Agent 链路 Span」会立刻暴露漏配（见 reportOnce）。
var langfuseAllowPrefixes = []string{
	"agent.",        // agent.run / agent.route / agent.react / agent.plan_execute
	"tool.",         // tool.<name> / tool.stream.open.<name>
	"chat ",         // generation（"chat <model>"）
	"queue.wait",    // 队列等待（run 执行链起点）
	"context.build", // 上下文构建
}

func keepSpanForLangfuse(name string) bool {
	for _, prefix := range langfuseAllowPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func newLangfuseSpanFilter(next sdktrace.SpanProcessor) *langfuseSpanFilter {
	return &langfuseSpanFilter{
		next:        next,
		seenKept:    make(map[string]struct{}),
		seenDropped: make(map[string]struct{}),
		stop:        make(chan struct{}),
	}
}

func (p *langfuseSpanFilter) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	p.next.OnStart(parent, s)
}

func (p *langfuseSpanFilter) OnEnd(s sdktrace.ReadOnlySpan) {
	name := s.Name()
	if !keepSpanForLangfuse(name) {
		p.dropped.Add(1)
		p.reportOnce(p.seenDropped, name, "丢弃非 Agent 链路 Span")
		return
	}
	p.kept.Add(1)
	p.reportOnce(p.seenKept, name, "导出 Agent 链路 Span")
	p.next.OnEnd(s)
}

func (p *langfuseSpanFilter) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() { close(p.stop) })
	return p.next.Shutdown(ctx)
}

func (p *langfuseSpanFilter) ForceFlush(ctx context.Context) error { return p.next.ForceFlush(ctx) }

// reportOnce 对每个 Span 名字只打一次日志。
// 目的是让「过滤器到底在不在工作」在运行日志里一眼可见，而不是只能靠 Langfuse 页面反推。
func (p *langfuseSpanFilter) reportOnce(seen map[string]struct{}, name, msg string) {
	p.seenMu.Lock()
	if _, ok := seen[name]; ok || len(seen) >= maxReportedSpanNames {
		p.seenMu.Unlock()
		return
	}
	seen[name] = struct{}{}
	p.seenMu.Unlock()

	logger.Info("[Langfuse] "+msg, zap.String("span", name))
}

// startStatsReporter 定期把「放行 / 丢弃」计数打进日志。
//
// 这是运行态唯一可靠的自证手段：如果日志里长期没有任何「丢弃」记录，
// 而 Langfuse 页面又存在 db.* 之类的噪声，那结论只有一个 ——
// 跑的不是当前这份代码（典型原因：进程没重启、或跑的是旧二进制）。
func (p *langfuseSpanFilter) startStatsReporter() {
	go func() {
		ticker := time.NewTicker(langfuseStatsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				kept := p.kept.Swap(0)
				dropped := p.dropped.Swap(0)
				if kept == 0 && dropped == 0 {
					continue // 静默期不刷屏
				}
				logger.Info("[Langfuse] 导出统计",
					zap.Int64("kept", kept),
					zap.Int64("dropped_noise", dropped),
					zap.Duration("window", langfuseStatsInterval),
				)
			}
		}
	}()
}
