package adkcore

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
)

// ADKReactRunner 是基于 Eino ADK ChatModelAgent 的 ReAct 运行器。
// 它与现有的 core.ReactRunner 接口兼容，但使用 ADK 的内部 ReAct 循环。
type ADKReactRunner struct {
	// ToolSet 是注册到 ADK 的工具集合。
	ToolSet *ToolSet
	// ModelFactory 是创建 ChatModel 的工厂函数。
	ModelFactory func(ctx context.Context, modelConfigID contracts.ID) (model.BaseModel[*schema.Message], error)
	// SystemPromptBuilder 构建系统提示词。
	SystemPromptBuilder func(ctx context.Context, agentRun contracts.AgentRunRequest) (string, error)
	// Config 默认配置。
	Config contracts.AgentConfig
}

// NewADKReactRunner 创建 ADK 驱动的 React 运行器。
func NewADKReactRunner(
	toolSet *ToolSet,
	modelFactory func(ctx context.Context, modelConfigID contracts.ID) (model.BaseModel[*schema.Message], error),
	systemPromptBuilder func(ctx context.Context, request contracts.AgentRunRequest) (string, error),
	config contracts.AgentConfig,
) *ADKReactRunner {
	return &ADKReactRunner{
		ToolSet:             toolSet,
		ModelFactory:        modelFactory,
		SystemPromptBuilder: systemPromptBuilder,
		Config:              config,
	}
}

// Run 执行一次 ADK 驱动的 ReAct 运行。
func (r *ADKReactRunner) Run(ctx context.Context, request contracts.AgentRunRequest, eventPublisher core.EventPublisher, citationCollector core.CitationCollector) (result contracts.AgentRunResult, err error) {
	startedAt := time.Now().UTC()

	// 1. 构建系统提示词
	systemPrompt, err := r.SystemPromptBuilder(ctx, request)
	if err != nil {
		return result, fmt.Errorf("build system prompt: %w", err)
	}

	// 2. 构建 ChatModel
	chatModel, err := r.ModelFactory(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return result, fmt.Errorf("create chat model: %w", err)
	}

	// 3. 将请求消息转换为 Eino schema.Message
	convertedMessages, err := convertToSchemaMessages(ctx, request, systemPrompt)
	if err != nil {
		return result, fmt.Errorf("convert messages: %w", err)
	}

	// 4. 配置中间件
	middleware := &AgentMiddleware{
		EventPublisher:    eventPublisher,
		CitationCollector: citationCollector,
		RunID:             request.RunID,
		ToolContext: contracts.ToolContext{
			UserID:           request.Context.UserID,
			KnowledgeBaseID:  request.Context.KnowledgeBaseID,
			AgentRunID:       request.RunID,
			AllowedToolNames: request.Context.AllowedTools,
			NetworkEnabled:   request.Context.NetworkEnabled,
			MaxResultBytes:   request.Config.MaxToolResultBytes,
		},
		Config:    request.Config,
		StartedAt: startedAt,
	}

	// 5. 构建 ADK ToolsConfig
	toolsConfig := r.buildToolsConfig()

	maxIterations := request.Config.MaxReactRounds
	if maxIterations <= 0 {
		maxIterations = r.Config.MaxReactRounds
	}
	if maxIterations <= 0 {
		maxIterations = 20 // ADK 默认值
	}

	// 6. 创建 ADK ChatModelAgent
	chatModelAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "assistant",
		Description:   "A helpful assistant that can use tools to help users.",
		Instruction:   systemPrompt,
		Model:         chatModel,
		ToolsConfig:   toolsConfig,
		MaxIterations: maxIterations,
		Handlers: []adk.ChatModelAgentMiddleware{
			middleware,
		},
	})
	if err != nil {
		return result, fmt.Errorf("create chat model agent: %w", err)
	}

	// 7. 注入 ToolContext 到 context
	execCtx := WithToolContext(ctx, contracts.ToolContext{
		UserID:           request.Context.UserID,
		KnowledgeBaseID:  request.Context.KnowledgeBaseID,
		AgentRunID:       request.RunID,
		AllowedToolNames: request.Context.AllowedTools,
		NetworkEnabled:   request.Context.NetworkEnabled,
		MaxResultBytes:   request.Config.MaxToolResultBytes,
	})

	// 8. 构建 AgentInput
	input := &adk.AgentInput{
		Messages: convertedMessages,
	}

	// 9. 构建 AgentRunOption
	opts := []adk.AgentRunOption{
		adk.WithSessionValues(map[string]any{
			"user_id":         request.Context.UserID,
			"run_id":          request.RunID,
			"started_at":      startedAt,
			"conversation_id": request.Context.ConversationID,
		}),
		adk.WithChatModelOptions(nil),
		adk.WithToolOptions(nil),
	}

	// 10. 通过 agent.Run 直接运行（返回异步迭代器）
	iter := chatModelAgent.Run(execCtx, input, opts...)

	// 11. 迭代事件流并收集最终结果
	var finalContent string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		// 检查事件错误
		if event.Err != nil {
			err = event.Err
		}
		// 从消息输出中提取最终内容
		// GetMessage 返回 (msg, wrappedEvent, error)
		if msg, _, getErr := adk.GetMessage(event); getErr == nil && msg != nil {
			if msg.Role == schema.Assistant && msg.Content != "" {
				finalContent = msg.Content
			}
		}
	}

	if err != nil {
		return result, fmt.Errorf("adk agent run: %w", err)
	}

	return contracts.AgentRunResult{
		RunID:         request.RunID,
		ExecutionMode: contracts.ExecutionReact,
		FinalResult:   finalContent,
		Citations:     citationCollector.Get(),
		Usage:         contracts.TokenUsage{}, // 通过中间件收集
		StartedAt:     startedAt,
		EndedAt:       time.Now().UTC(),
	}, nil
}

// buildToolsConfig 从 ToolSet 构建 ADK 的 ToolsConfig。
func (r *ADKReactRunner) buildToolsConfig() adk.ToolsConfig {
	if r.ToolSet == nil || len(r.ToolSet.Tools) == 0 {
		return adk.ToolsConfig{}
	}

	return adk.ToolsConfig{
		ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: r.ToolSet.Tools,
		},
	}
}

// convertToSchemaMessages 将现有的聊天消息转换为 Eino schema.Message。
func convertToSchemaMessages(ctx context.Context, request contracts.AgentRunRequest, systemPrompt string) ([]*schema.Message, error) {
	messages := request.Context.Conversation.Messages
	msgs := make([]*schema.Message, 0, len(messages)+1)

	// 将现有的消息历史转换为 schema.Message
	for _, chatMsg := range messages {
		switch chatMsg.Role {
		case "user":
			msgs = append(msgs, schema.UserMessage(chatMsg.Content))
		case "assistant":
			msg := schema.AssistantMessage(chatMsg.Content, nil)
			msgs = append(msgs, msg)
		case "system":
			msgs = append(msgs, schema.SystemMessage(chatMsg.Content))
		case "tool":
			// ConversationMessage 没有 ToolCallID 字段，使用内容作为工具消息
			toolMsg := schema.ToolMessage(chatMsg.Content, "historical")
			msgs = append(msgs, toolMsg)
		}
	}

	if request.Context.Query != "" {
		msgs = append(msgs, schema.UserMessage(request.Context.Query))
	}
	return msgs, nil
}

// ToolSet 是准备注册到 ADK 的工具集合。
type ToolSet struct {
	Tools    []tool.BaseTool
	Executor *tools.Executor
}

// BuildToolSet 从 tool.BaseTool 列表构建 ADK 工具集。
func BuildToolSet(innerTools []tool.BaseTool, executor *tools.Executor) *ToolSet {
	if innerTools == nil {
		innerTools = make([]tool.BaseTool, 0)
	}
	return &ToolSet{
		Tools:    innerTools,
		Executor: executor,
	}
}
