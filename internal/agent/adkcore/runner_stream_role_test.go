package adkcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/config"
)

// loadExampleConfig 用示例配置初始化全局配置。
// 中间件会读 config.Get()（Langfuse 正文开关），未初始化会 panic；
// 示例文件扩展名是 .example，viper 认不出来，先复制成临时的 .yaml。
func loadExampleConfig(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("../../../configs/config.yaml.example")
	if err != nil {
		t.Fatalf("读取示例配置失败: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("加载示例配置失败: %v", err)
	}
}

// newTestRunner 组装一个只依赖假模型的 Runner，其余依赖（工具记录 / 工具规格）都留空。
func newTestRunner(m model.BaseModel[*schema.Message]) *ADKReactRunner {
	return NewADKReactRunner(
		func(context.Context, contracts.ID) (model.BaseModel[*schema.Message], error) { return m, nil },
		func(context.Context, contracts.AgentRunRequest) (string, error) { return "测试系统提示词", nil },
		contracts.DefaultAgentConfig(),
		nil,
		nil,
	)
}

// newTestRequest 构造一次最小可运行的 ReAct 请求。
func newTestRequest(tools ...tool.BaseTool) contracts.AgentRunRequest {
	return contracts.AgentRunRequest{
		RunID: contracts.ID("11111111-1111-1111-1111-111111111111"),
		Context: contracts.AgentContext{
			UserID:      contracts.ID("22222222-2222-2222-2222-222222222222"),
			Query:       "测试问题",
			ChatModelID: "fake-model",
			ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		},
		Config: contracts.DefaultAgentConfig(),
	}
}

// scriptedStreamModel 是按轮次脚本吐内容的假模型。
// 每调用一次 Stream 消费一轮脚本，用于只跑 ADK 的 ReAct 循环而不碰真实模型。
type scriptedStreamModel struct {
	mu     sync.Mutex
	rounds [][]*schema.Message
	idx    int
	// onExhausted 在脚本耗尽后接管，用于返回一条可控时序的流（验证逐字推送）。
	onExhausted func() *schema.StreamReader[*schema.Message]
}

func (m *scriptedStreamModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("非流式兜底", nil), nil
}

func (m *scriptedStreamModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.rounds) {
		if m.onExhausted != nil {
			return m.onExhausted(), nil
		}
		return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", nil)}), nil
	}
	chunks := m.rounds[m.idx]
	m.idx++
	return schema.StreamReaderFromArray(chunks), nil
}

// streamableEchoTool 同时实现 InvokableRun / StreamableRun。
// 必须实现流式接口，否则 ADK 只会走非流式工具路径，复现不出「工具结果也是流式事件」这个问题。
type streamableEchoTool struct {
	result string
}

func (t *streamableEchoTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "fake_search",
		Desc: "测试用检索工具",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"q": {Type: schema.String, Desc: "查询词", Required: true},
		}),
	}, nil
}

func (t *streamableEchoTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	return t.result, nil
}

func (t *streamableEchoTool) StreamableRun(context.Context, string, ...tool.Option) (*schema.StreamReader[string], error) {
	return schema.StreamReaderFromArray([]string{t.result}), nil
}

// deltaRecorder 只关心 answer.delta，其余事件透传 Noop。
type deltaRecorder struct {
	core.NoopEventPublisher
	mu     sync.Mutex
	deltas []string
}

func (r *deltaRecorder) PublishAnswerDelta(_ context.Context, _ contracts.ID, delta string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deltas = append(r.deltas, delta)
	return nil
}

func (r *deltaRecorder) joined() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.deltas, "")
}

// TestReactRunnerStreamingOnlyPublishesAssistantOutput 是「中间内容泄漏」的回归测试。
//
// 桩数据刻意构造成最容易出错的样子：第 1 轮先吐中间正文、再吐工具调用，工具返回内容里带唯一标记。
// 期望：只有第 2 轮（纯回答轮）的正文会变成 agent.answer.delta——工具返回内容与中间正文都不许出现。
func TestReactRunnerStreamingOnlyPublishesAssistantOutput(t *testing.T) {
	loadExampleConfig(t)

	const (
		intermediate = "先查一下资料吧。"
		toolOutput   = `{"call_id":"call_1","tool_name":"fake_search","success":true,"text":"TOOL-OUTPUT-MARKER"}`
		finalAnswer  = "这是最终答案。"
	)

	scripted := &scriptedStreamModel{rounds: [][]*schema.Message{
		{
			schema.AssistantMessage(intermediate, nil),
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:       "call_1",
					Type:     "function",
					Function: schema.FunctionCall{Name: "fake_search", Arguments: `{"q":"x"}`},
				}},
			},
		},
		{
			schema.AssistantMessage("这是", nil),
			schema.AssistantMessage("最终答案。", nil),
		},
	}}

	runner := newTestRunner(scripted)
	request := newTestRequest(&streamableEchoTool{result: toolOutput})

	publisher := &deltaRecorder{}
	result, err := runner.Run(context.Background(), request, publisher, core.NewCitationCollector())
	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	got := publisher.joined()
	if strings.Contains(got, "TOOL-OUTPUT-MARKER") {
		t.Fatalf("工具返回内容被当成回答增量发布（中间内容泄漏）: %q", got)
	}
	if strings.Contains(got, intermediate) {
		t.Fatalf("工具轮次的中间正文被当成回答增量发布: %q", got)
	}
	if got != finalAnswer {
		t.Fatalf("回答增量拼接结果应为 %q，实际 %q", finalAnswer, got)
	}
	if result.FinalResult != finalAnswer {
		t.Fatalf("FinalResult 应为 %q，实际 %q", finalAnswer, result.FinalResult)
	}
}

// TestReactRunnerPublishesFinalAnswerBeforeStreamEnds 锁住「最终回答逐块实时推送」这条性质。
//
// 用一条可控流：先给一块超过观察窗口的正文并停住不结束，此刻就应该已经有 delta 到达前端；
// 若退化成「整轮缓冲、流结束才发」（f99db4a 的行为），本用例会因为等不到增量而失败。
func TestReactRunnerPublishesFinalAnswerBeforeStreamEnds(t *testing.T) {
	loadExampleConfig(t)

	firstChunk := strings.Repeat("甲", answerHoldbackRunes+20)
	release := make(chan struct{})

	scripted := &scriptedStreamModel{
		rounds: [][]*schema.Message{
			{{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:       "call_1",
					Type:     "function",
					Function: schema.FunctionCall{Name: "fake_search", Arguments: `{"q":"x"}`},
				}},
			}},
		},
		onExhausted: func() *schema.StreamReader[*schema.Message] {
			reader, writer := schema.Pipe[*schema.Message](1)
			go func() {
				defer writer.Close()
				if writer.Send(schema.AssistantMessage(firstChunk, nil), nil) {
					return
				}
				<-release // 流仍未结束，用来证明增量不是等到整轮结束才发
				writer.Send(schema.AssistantMessage("乙", nil), nil)
			}()
			return reader
		},
	}

	runner := newTestRunner(scripted)
	request := newTestRequest(&streamableEchoTool{result: "tool-result"})
	publisher := &deltaRecorder{}

	var (
		result contracts.AgentRunResult
		runErr error
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, runErr = runner.Run(context.Background(), request, publisher, core.NewCitationCollector())
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && publisher.joined() == "" {
		time.Sleep(10 * time.Millisecond)
	}
	if publisher.joined() == "" {
		close(release)
		<-done
		t.Fatal("流未结束时没有任何回答增量（最终回答退化为整段输出）")
	}

	close(release)
	<-done
	if runErr != nil {
		t.Fatalf("运行失败: %v", runErr)
	}

	want := firstChunk + "乙"
	if got := publisher.joined(); got != want {
		t.Fatalf("回答增量拼接结果应为 %q，实际 %q", want, got)
	}
	if result.FinalResult != want {
		t.Fatalf("FinalResult 应为 %q，实际 %q", want, result.FinalResult)
	}
}
