package adkcore

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
)

// ADKReactRunner 是基于 Eino ADK ChatModelAgent 的 ReAct 运行器。
// 它与现有的 core.ReactRunner 接口兼容，但使用 ADK 的内部 ReAct 循环。
type ADKReactRunner struct {
	// ModelFactory 是创建 ChatModel 的工厂函数。
	ModelFactory func(ctx context.Context, modelConfigID contracts.ID) (model.BaseModel[*schema.Message], error)
	// SystemPromptBuilder 构建系统提示词。
	SystemPromptBuilder func(ctx context.Context, agentRun contracts.AgentRunRequest) (string, error)
	// Config 默认配置。
	Config contracts.AgentConfig
	// ToolCallRepo 用于持久化工具调用记录。
	ToolCallRepo repository.ToolCallRepository
}

// NewADKReactRunner 创建 ADK 驱动的 React 运行器。
func NewADKReactRunner(
	modelFactory func(ctx context.Context, modelConfigID contracts.ID) (model.BaseModel[*schema.Message], error),
	systemPromptBuilder func(ctx context.Context, request contracts.AgentRunRequest) (string, error),
	config contracts.AgentConfig,
) *ADKReactRunner {
	return &ADKReactRunner{
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

	// 追加工具调用失败处理规则到系统提示词。
	// 当 SafeToolMiddleware 将工具错误转为字符串返回给 LLM 时，
	// LLM 需要知道不应该无限重试同一个失败的工具。
	systemPrompt += "\n\n[Tool Call Rules]\n" +
		"When a tool call returns an error (Success=false), do NOT retry the same tool with the same parameters.\n" +
		"If a tool fails, try at most once more with different parameters if appropriate.\n" +
		"After a tool has failed twice, do not retry it again. Instead:\n" +
		"- Try a different tool that might achieve the same goal\n" +
		"- Use the information you already have to answer the user\n" +
		"- If you cannot proceed, inform the user about the limitation clearly\n" +
		"Do not waste iterations by repeatedly calling the same failing tool.\n"

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

	// 5. 使用上下文阶段构建好的 ADK ToolsConfig
	toolsConfig := request.Context.ToolsConfig

	maxIterations := request.Config.MaxReactRounds
	if maxIterations <= 0 {
		maxIterations = r.Config.MaxReactRounds
	}
	if maxIterations <= 0 {
		maxIterations = 20 // ADK 默认值
	}

	// 6. 创建 ADK ChatModelAgent
	// 中间件顺序：
	//   SafeToolMiddleware（最外层）：捕获所有工具调用的错误并发布失败事件
	//   ToolCallRecorderMiddleware（中间层）：持久化每次工具调用的详细信息到 tool_calls 表
	//   AgentMiddleware（最内层）：事件发布、引用收集、预算控制
	chatModelAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "assistant",
		Description:   "A helpful assistant that can use tools to help users.",
		Instruction:   systemPrompt,
		Model:         chatModel,
		ToolsConfig:   toolsConfig,
		MaxIterations: maxIterations,
		Handlers: []adk.ChatModelAgentMiddleware{
			&SafeToolMiddleware{
				EventPublisher: eventPublisher,
				RunID:          request.RunID,
			},
			&ToolCallRecorderMiddleware{
				ToolCallRepo: r.ToolCallRepo,
				RunID:        request.RunID,
			},
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

	// 11. 迭代事件流并收集最终结果和 token 用量
	// 注意：ADK 的 AfterAgent 中间件钩子在错误路径上不会被调用，
	// 因此必须在事件循环中主动累积 token 用量，确保失败时也有记录。
	var finalContent string
	var accumulatedUsage contracts.TokenUsage
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
		// 从消息输出中提取最终内容和 token 用量
		// GetMessage 返回 (msg, wrappedEvent, error)
		if msg, _, getErr := adk.GetMessage(event); getErr == nil && msg != nil {
			if msg.Role == schema.Assistant && msg.Content != "" {
				finalContent = msg.Content
			}
			// 从每条消息中提取 token 用量并累积
			if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
				accumulatedUsage.InputTokens += msg.ResponseMeta.Usage.PromptTokens
				accumulatedUsage.OutputTokens += msg.ResponseMeta.Usage.CompletionTokens
				accumulatedUsage.TotalTokens += msg.ResponseMeta.Usage.TotalTokens
			}
		}
	}

	if err != nil {
		// AfterAgent 不会被调用，使用事件循环中累积的用量
		result.Usage = accumulatedUsage
		result.RunID = request.RunID
		return result, fmt.Errorf("adk agent run: %w", err)
	}

	return contracts.AgentRunResult{
		RunID:         request.RunID,
		ExecutionMode: contracts.ExecutionReact,
		FinalResult:   finalContent,
		Citations:     citationCollector.Get(),
		Usage:         accumulatedUsage, // AfterAgent 只在成功路径被调用，此处等效
		StartedAt:     startedAt,
		EndedAt:       time.Now().UTC(),
	}, nil
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
