package adkcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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

// scriptedStreamModel 是按轮次脚本吐内容的假模型。
// 每调用一次 Stream 消费一轮脚本，用于只跑 ADK 的 ReAct 循环而不碰真实模型。
type scriptedStreamModel struct {
	mu     sync.Mutex
	rounds [][]*schema.Message
	idx    int
}

func (m *scriptedStreamModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("非流式兜底", nil), nil
}

func (m *scriptedStreamModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.rounds) {
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

	runner := NewADKReactRunner(
		func(context.Context, contracts.ID) (model.BaseModel[*schema.Message], error) { return scripted, nil },
		func(context.Context, contracts.AgentRunRequest) (string, error) { return "测试系统提示词", nil },
		contracts.DefaultAgentConfig(),
		nil,
		nil,
	)

	request := contracts.AgentRunRequest{
		RunID: contracts.ID("11111111-1111-1111-1111-111111111111"),
		Context: contracts.AgentContext{
			UserID:      contracts.ID("22222222-2222-2222-2222-222222222222"),
			Query:       "测试问题",
			ChatModelID: "fake-model",
			ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{&streamableEchoTool{result: toolOutput}},
			}},
		},
		Config: contracts.DefaultAgentConfig(),
	}

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
