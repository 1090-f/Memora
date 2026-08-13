package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ReactPromptConfig 定义 ReAct Prompt 配置。
type ReactPromptConfig struct {
	System string `yaml:"system"` // 系统提示词模板
	User   string `yaml:"user"`   // 用户提示词模板
}

// ReactService 管理 ReAct 循环的执行逻辑。
// 参考 Plan-Execute 模式的 Service 分层设计，将 ReAct 循环的核心业务逻辑
// 从 Runner 中抽离为独立的 Service，便于测试和复用。
type ReactService struct {
	modelFactory contracts.ModelFactory // 模型工厂，用于创建 ChatModel 实例
	executor     contracts.ToolExecutor // 工具执行器，统一工具调用入口
	registry     contracts.ToolRegistry // 工具注册表，获取工具规格
	promptConfig ReactPromptConfig      // Prompt 配置模板
}

// NewReactService 创建 ReactService 实例。
// 参数:
//   - modelFactory: 模型工厂
//   - executor: 工具执行器
//   - registry: 工具注册表
//   - promptConfigPath: Prompt 配置文件路径
func NewReactService(
	modelFactory contracts.ModelFactory,
	executor contracts.ToolExecutor,
	registry contracts.ToolRegistry,
	promptConfigPath string,
) (*ReactService, error) {
	config, err := loadReactPromptConfig(promptConfigPath)
	if err != nil {
		return nil, fmt.Errorf("加载 React Prompt 配置: %w", err)
	}

	return &ReactService{
		modelFactory: modelFactory,
		executor:     executor,
		registry:     registry,
		promptConfig: *config,
	}, nil
}

// loadReactPromptConfig 从 YAML 文件加载 Prompt 配置。
func loadReactPromptConfig(path string) (*ReactPromptConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 React Prompt 配置文件: %w", err)
	}

	var config ReactPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 React Prompt 配置: %w", err)
	}

	return &config, nil
}

// ReActResult 包含 ReAct 循环的执行结果。
type ReActResult struct {
	FinalResult   string               // 最终回答文本
	Citations     []contracts.Citation // 引用列表
	Usage         contracts.TokenUsage // Token 消耗
	RoundCount    int                  // 实际执行的轮数
	ToolCallCount int                  // 实际工具调用次数
}

// RunReActLoop 执行完整的 ReAct 循环。
// 这是 ReAct 的核心方法，包含循环调用模型、执行工具、收集结果的完整链路。
// 参考 Plan-Execute 中 PlannerService.Plan 和 PlanExecutorService.Execute 的设计模式。
func (s *ReactService) RunReActLoop(
	ctx context.Context,
	agentCtx contracts.AgentContext,
	cfg contracts.AgentConfig,
	onRoundStarted func(ctx context.Context, round int) error,
	onToolStarted func(ctx context.Context, toolName string, callID contracts.ID) error,
	onToolCompleted func(ctx context.Context, toolName string, callID contracts.ID, success bool, summary string) error,
	onAnswerDelta func(ctx context.Context, delta string) error,
) (ReActResult, error) {
	startedAt := time.Now()

	logger.Info("开始 ReAct 循环",
		zap.String("query", agentCtx.Query),
		zap.Int("max_rounds", cfg.MaxReactRounds),
		zap.Int("max_tool_calls", cfg.MaxToolCalls),
	)

	// 获取 ChatModel 实例
	model, err := s.modelFactory.GetChatModel(ctx, contracts.ID(agentCtx.ChatModelID))
	if err != nil {
		return ReActResult{}, fmt.Errorf("获取 ChatModel: %w", err)
	}

	// 初始化消息列表
	messages := s.buildInitialMessages(agentCtx, cfg)

	var accumulatedUsage contracts.TokenUsage
	var allCitations []contracts.Citation
	toolCallCount := 0
	var finalResult string

	// ReAct 主循环
	for round := 1; round <= cfg.MaxReactRounds; round++ {
		// 检查上下文是否已取消
		if err := ctx.Err(); err != nil {
			logger.Warn("ReAct 循环被取消",
				zap.Int("round", round),
				zap.Error(err),
			)
			return ReActResult{
				FinalResult:   finalResult,
				Citations:     allCitations,
				Usage:         accumulatedUsage,
				RoundCount:    round - 1,
				ToolCallCount: toolCallCount,
			}, err
		}

		// 检查运行时长
		if time.Since(startedAt) > time.Duration(cfg.MaxRunSeconds)*time.Second {
			logger.Warn("ReAct 循环超时",
				zap.Int("round", round),
				zap.Duration("elapsed", time.Since(startedAt)),
			)
			if finalResult == "" {
				return ReActResult{
					Citations:     allCitations,
					Usage:         accumulatedUsage,
					RoundCount:    round - 1,
					ToolCallCount: toolCallCount,
				}, fmt.Errorf("ReAct 循环超时: 已运行 %v, 超过限制 %d 秒", time.Since(startedAt), cfg.MaxRunSeconds)
			}
			break
		}

		// 发布轮次开始事件
		if onRoundStarted != nil {
			if err := onRoundStarted(ctx, round); err != nil {
				logger.Warn("发布轮次开始事件失败", zap.Error(err))
			}
		}

		logger.Info("ReAct 轮次",
			zap.Int("round", round),
			zap.Int("message_count", len(messages)),
			zap.Int("tool_calls_so_far", toolCallCount),
		)

		// 获取当前轮次的工具定义（重新获取以反映工具启用状态的动态变化）
		toolDefs, err := s.buildToolDefinitions(ctx, agentCtx.AllowedTools)
		if err != nil {
			return ReActResult{}, fmt.Errorf("构建工具定义: %w", err)
		}

		// 调用模型（流式）
		// 使用 Eino ChatModel 流式生成响应，实时推送文本增量
		streamCh, err := model.Stream(ctx, contracts.ChatRequest{
			Messages: messages,
			Tools:    toolDefs,
		})
		if err != nil {
			return ReActResult{}, fmt.Errorf("模型流式调用失败(轮次 %d): %w", round, err)
		}

		var fullContent strings.Builder
		var roundToolCalls []contracts.ToolCall
		var roundUsage contracts.TokenUsage

		for chunk := range streamCh {
			if chunk.Delta != "" {
				fullContent.WriteString(chunk.Delta)
				if onAnswerDelta != nil {
					if err := onAnswerDelta(ctx, chunk.Delta); err != nil {
						logger.Warn("发布回答增量事件失败", zap.Error(err))
					}
				}
			}
			if len(chunk.ToolCalls) > 0 {
				roundToolCalls = append(roundToolCalls, chunk.ToolCalls...)
			}
			if chunk.Usage != nil {
				roundUsage.InputTokens = chunk.Usage.InputTokens
				roundUsage.OutputTokens = chunk.Usage.OutputTokens
				roundUsage.TotalTokens = chunk.Usage.TotalTokens
			}
			if chunk.Done {
				break
			}
		}

		content := fullContent.String()

		// 累加 Token 用量
		accumulatedUsage.InputTokens += roundUsage.InputTokens
		accumulatedUsage.OutputTokens += roundUsage.OutputTokens
		accumulatedUsage.TotalTokens += roundUsage.TotalTokens

		// 添加模型回复到消息列表
		assistantMsg := contracts.ChatMessage{
			Role:    "assistant",
			Content: content,
		}
		if len(roundToolCalls) > 0 {
			assistantMsg.ToolCalls = roundToolCalls
		}
		messages = append(messages, assistantMsg)

		// 检查是否有工具调用
		if len(roundToolCalls) == 0 {
			// 没有工具调用，模型给出最终答案
			finalResult = strings.TrimSpace(content)
			logger.Info("ReAct 循环完成",
				zap.Int("round", round),
				zap.Int("tool_calls", toolCallCount),
			)
			break
		}

		// 处理工具调用（使用流式收集的工具调用列表）
		for _, call := range roundToolCalls {
			toolCallCount++
			if toolCallCount > cfg.MaxToolCalls {
				logger.Warn("工具调用次数超限",
					zap.Int("tool_calls", toolCallCount),
					zap.Int("max_tool_calls", cfg.MaxToolCalls),
				)
				return ReActResult{
					FinalResult:   finalResult,
					Citations:     allCitations,
					Usage:         accumulatedUsage,
					RoundCount:    round,
					ToolCallCount: toolCallCount - 1,
				}, fmt.Errorf("工具调用次数 %d 超过上限 %d", toolCallCount, cfg.MaxToolCalls)
			}

			// 发布工具调用开始事件
			if onToolStarted != nil {
				if err := onToolStarted(ctx, call.ToolName, call.CallID); err != nil {
					logger.Warn("发布工具调用开始事件失败", zap.Error(err))
				}
			}

			// 记录工具调用
			logger.Info("工具调用",
				zap.String("tool_name", call.ToolName),
				zap.String("call_id", string(call.CallID)),
				zap.Int("round", round),
				zap.Int("tool_call_index", toolCallCount),
			)

			// 构建工具执行上下文
			toolCtx := contracts.ToolContext{
				UserID:           agentCtx.UserID,
				KnowledgeBaseID:  agentCtx.KnowledgeBaseID,
				AgentRunID:       agentCtx.RunID,
				ReactRound:       round,
				AllowedToolNames: agentCtx.AllowedTools,
				NetworkEnabled:   agentCtx.NetworkEnabled,
				MaxResultBytes:   cfg.MaxToolResultBytes,
			}

			// 通过 ToolExecutor 执行工具调用
			// 这是唯一的工具执行入口，所有工具（内置 + MCP）都必须经过此入口
			toolResult, execErr := s.executor.Execute(ctx, toolCtx, call)
			if execErr != nil {
				logger.Error("工具调用失败",
					zap.String("tool_name", call.ToolName),
					zap.String("call_id", string(call.CallID)),
					zap.Error(execErr),
				)
			}

			// 收集引用
			if len(toolResult.Citations) > 0 {
				allCitations = append(allCitations, toolResult.Citations...)
			}

			// 构建工具结果消息（必须携带 ToolCallID 以关联对应的工具调用）
			resultContent := s.formatToolResult(toolResult, execErr)
			messages = append(messages, contracts.ChatMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: string(call.CallID),
			})

			// 发布工具调用完成事件
			if onToolCompleted != nil {
				summary := s.buildToolCallSummary(toolResult, execErr)
				if err := onToolCompleted(ctx, call.ToolName, call.CallID, execErr == nil, summary); err != nil {
					logger.Warn("发布工具调用完成事件失败", zap.Error(err))
				}
			}
		}
	}

	// 如果循环结束仍未得到最终答案
	if finalResult == "" {
		logger.Warn("ReAct 循环未产生最终答案",
			zap.Int("rounds", cfg.MaxReactRounds),
			zap.Int("tool_calls", toolCallCount),
		)
		return ReActResult{
			Citations:     allCitations,
			Usage:         accumulatedUsage,
			RoundCount:    cfg.MaxReactRounds,
			ToolCallCount: toolCallCount,
		}, fmt.Errorf("ReAct 循环 %d 轮后未生成最终答案", cfg.MaxReactRounds)
	}

	logger.Info("ReAct 循环执行完成",
		zap.Int("rounds_used", len(messages)),
		zap.Int("tool_calls", toolCallCount),
		zap.Duration("elapsed", time.Since(startedAt)),
	)

	return ReActResult{
		FinalResult:   finalResult,
		Citations:     allCitations,
		Usage:         accumulatedUsage,
		RoundCount:    len(messages),
		ToolCallCount: toolCallCount,
	}, nil
}

// buildInitialMessages 构建初始消息列表。
// 包含系统提示词（含工具定义）、会话历史、记忆和用户查询。
func (s *ReactService) buildInitialMessages(agentCtx contracts.AgentContext, cfg contracts.AgentConfig) []contracts.ChatMessage {
	var messages []contracts.ChatMessage

	// 1. 构建系统提示词（含工具定义）
	systemPrompt := s.buildSystemPrompt(agentCtx, cfg)
	messages = append(messages, contracts.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// 2. 添加会话历史
	for _, msg := range agentCtx.Conversation.Messages {
		messages = append(messages, contracts.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 3. 添加用户查询（包含上下文标签）
	userPrompt := s.buildUserPrompt(agentCtx)
	messages = append(messages, contracts.ChatMessage{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

// buildSystemPrompt 构建系统提示词。
// 使用模板引擎渲染工具定义到系统提示词中。
func (s *ReactService) buildSystemPrompt(agentCtx contracts.AgentContext, cfg contracts.AgentConfig) string {
	// 获取工具规格
	toolSpecs := s.registry.Specs()

	// 构建工具描述
	var toolDescs []string
	for _, spec := range toolSpecs {
		if !spec.Enabled {
			continue
		}
		desc := fmt.Sprintf("- %s: %s", spec.Name, spec.Description)
		if spec.NetworkRequired {
			desc += " [需要联网]"
		}
		toolDescs = append(toolDescs, desc)
	}
	toolsText := strings.Join(toolDescs, "\n")
	if toolsText == "" {
		toolsText = "当前没有可用工具。"
	}

	// 渲染系统提示词模板
	tmpl, err := template.New("react_system").Parse(s.promptConfig.System)
	if err != nil {
		logger.Warn("解析系统提示词模板失败，使用默认提示词", zap.Error(err))
		return fmt.Sprintf(`你是一个智能助手，通过逐步推理和调用工具来帮助用户解决问题。

可用工具:
%s

请逐步思考并解决问题。如果需要信息，直接调用对应的工具函数。
如果已经获得足够信息，直接给出最终答案。`, toolsText)
	}

	data := struct {
		Tools string
	}{
		Tools: toolsText,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.Warn("执行系统提示词模板失败，使用默认提示词", zap.Error(err))
		return fmt.Sprintf(`你是一个智能助手，通过逐步推理和调用工具来帮助用户解决问题。

可用工具:
%s

请逐步思考并解决问题。如果需要信息，直接调用对应的工具函数。
如果已经获得足够信息，直接给出最终答案。`, toolsText)
	}

	return buf.String()
}

// buildUserPrompt 构建用户提示词。
// 包含带标签的上下文信息和用户问题。
func (s *ReactService) buildUserPrompt(agentCtx contracts.AgentContext) string {
	// 使用带有紧凑标签的上下文格式
	return agentCtx.ToPromptWithTagsCompact()
}

// buildToolDefinitions 获取工具定义（JSON 格式的 ToolInfo 列表）。
func (s *ReactService) buildToolDefinitions(ctx context.Context, allowedTools []string) (json.RawMessage, error) {
	// 如果 ToolRegistry 可以返回 Eino 格式的工具信息，使用它
	// 否则直接从 Specs 构建
	specs := s.registry.Specs()

	// 日志：记录注册表中有多少工具
	logger.Debug("buildToolDefinitions: 工具注册表状态",
		zap.Int("total_specs", len(specs)),
	)

	var filtered []contracts.ToolSpec
	for _, spec := range specs {
		if !spec.Enabled {
			logger.Debug("buildToolDefinitions: 跳过未启用的工具",
				zap.String("tool_name", spec.Name),
			)
			continue
		}
		// 检查是否在白名单中
		if len(allowedTools) > 0 {
			inAllowed := false
			for _, name := range allowedTools {
				if spec.Name == name {
					inAllowed = true
					break
				}
			}
			if !inAllowed {
				logger.Debug("buildToolDefinitions: 工具不在白名单中",
					zap.String("tool_name", spec.Name),
					zap.Any("allowed_tools", allowedTools),
				)
				continue
			}
		}
		filtered = append(filtered, spec)
	}

	if len(filtered) == 0 {
		logger.Warn("buildToolDefinitions: 没有可用的工具，将不会传递工具定义给模型",
			zap.Int("total_specs", len(specs)),
			zap.Any("allowed_tools", allowedTools),
		)
		return nil, nil
	}

	logger.Debug("buildToolDefinitions: 工具筛选完成",
		zap.Int("total_specs", len(specs)),
		zap.Int("filtered_count", len(filtered)),
		zap.Strings("tool_names", extractToolNames(filtered)),
	)

	data, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("序列化工具定义: %w", err)
	}

	return data, nil
}

// extractToolNames 从 ToolSpec 切片中提取工具名称列表。
func extractToolNames(specs []contracts.ToolSpec) []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}

// formatToolResult 格式化工具执行结果为模型可读的消息内容。
func (s *ReactService) formatToolResult(result contracts.ToolResult, execErr error) string {
	if execErr != nil {
		// 工具调用失败，返回错误信息供模型决策
		errInfo := map[string]any{
			"call_id":       string(result.CallID),
			"tool_name":     result.ToolName,
			"success":       false,
			"error_code":    result.ErrorCode,
			"error_message": result.ErrorMessage,
		}
		if result.ErrorMessage == "" {
			errInfo["error_message"] = execErr.Error()
		}
		data, _ := json.Marshal(errInfo)
		return string(data)
	}

	// 工具调用成功，返回结果
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"call_id":"%s","tool_name":"%s","success":false,"error_message":"序列化结果失败"}`, result.CallID, result.ToolName)
	}
	return string(data)
}

// buildToolCallSummary 构建工具调用的摘要信息（用于事件发布）。
func (s *ReactService) buildToolCallSummary(result contracts.ToolResult, execErr error) string {
	if execErr != nil || !result.Success {
		return fmt.Sprintf("工具 %s 调用失败: %s", result.ToolName, result.ErrorMessage)
	}

	summary := fmt.Sprintf("工具 %s 调用成功", result.ToolName)
	if result.Truncated {
		summary += "（结果已截断）"
	}
	if len(result.Citations) > 0 {
		summary += fmt.Sprintf("，产生 %d 条引用", len(result.Citations))
	}
	if result.Text != "" {
		textLen := len(result.Text)
		if textLen > 100 {
			textLen = 100
		}
		summary += fmt.Sprintf("，结果长度 %d 字符", textLen)
	}
	return summary
}
