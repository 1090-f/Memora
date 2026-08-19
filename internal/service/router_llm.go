package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/1090-f/Memora/internal/ai/prompts"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// LLMRouter 基于 LLM 的路由器
type LLMRouter struct {
	modelFactory contracts.ModelFactory
}

// NewLLMRouter 创建 LLM 路由器
func NewLLMRouter(modelFactory contracts.ModelFactory) *LLMRouter {
	return &LLMRouter{
		modelFactory: modelFactory,
	}
}

// LLMRouteResult LLM 路由结果
type LLMRouteResult struct {
	Mode       contracts.ExecutionMode
	Confidence float64
	Reason     string
	Error      error
}

// Route 使用 LLM 进行路由判断
func (r *LLMRouter) Route(ctx context.Context, agentCtx contracts.AgentContext) LLMRouteResult {
	// 构建路由提示词
	prompt := r.buildRouterPrompt(agentCtx)

	// 从 AgentContext 获取 ChatModelID（使用用户配置的模型）
	chatModelID := contracts.ID(agentCtx.ChatModelID)
	if chatModelID == "" {
		logger.Warn("未配置 ChatModelID，使用默认路由")
		return LLMRouteResult{
			Mode:       contracts.ExecutionReact,
			Confidence: 0.3,
			Reason:     "未配置 ChatModelID，使用默认路由",
		}
	}

	// 调用 LLM
	model, err := r.modelFactory.GetChatModel(ctx, chatModelID)
	if err != nil {
		logger.Error("获取路由模型失败", zap.String("model_id", string(chatModelID)), zap.Error(err))
		return LLMRouteResult{
			Mode:       contracts.ExecutionReact,
			Confidence: 0.3,
			Reason:     "模型获取失败，使用默认路由",
			Error:      err,
		}
	}

	// 构建请求
	request := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3, // 低温度，更确定性的路由决策
	}

	// 生成响应
	response, err := model.Generate(ctx, request)
	if err != nil {
		logger.Error("LLM 路由生成失败", zap.Error(err))
		return LLMRouteResult{
			Mode:       contracts.ExecutionReact,
			Confidence: 0.3,
			Reason:     "LLM 生成失败，使用默认路由",
			Error:      err,
		}
	}

	// 解析响应
	return r.parseResponse(response.Content)
}

// buildRouterPrompt 构建路由提示词（从 yaml 模板渲染）
func (r *LLMRouter) buildRouterPrompt(agentCtx contracts.AgentContext) string {
	// 构建对话历史（只取最近 3 条）
	var history []prompts.ConversationMessage
	if len(agentCtx.Conversation.Messages) > 0 {
		start := len(agentCtx.Conversation.Messages) - 3
		if start < 0 {
			start = 0
		}
		for _, msg := range agentCtx.Conversation.Messages[start:] {
			history = append(history, prompts.ConversationMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// 构建记忆（只取前 2 条）
	var memories []prompts.MemoryResult
	limit := 2
	if len(agentCtx.Memories) < limit {
		limit = len(agentCtx.Memories)
	}
	for _, mem := range agentCtx.Memories[:limit] {
		memories = append(memories, prompts.MemoryResult{
			Content: mem.Content,
		})
	}

	// 渲染模板
	data := prompts.RouterPromptData{
		Query:               agentCtx.Query,
		ConversationHistory: history,
		Memories:            memories,
	}

	userPrompt, err := prompts.RouterPrompt.Render(data)
	if err != nil {
		logger.Error("渲染路由提示词失败", zap.Error(err))
		// 降级到简单提示词
		return "请判断以下问题应该用 react 模式处理，并返回 JSON: {\"mode\": \"react\", \"confidence\": ..., \"reason\": \"...\"}\n\n问题: " + agentCtx.Query
	}

	return userPrompt
}

// parseResponse 解析 LLM 响应
func (r *LLMRouter) parseResponse(response string) LLMRouteResult {
	// 尝试提取 JSON
	jsonStr := r.extractJSON(response)
	if jsonStr == "" {
		return LLMRouteResult{
			Mode:       contracts.ExecutionReact,
			Confidence: 0.4,
			Reason:     "无法解析 LLM 响应，使用默认路由",
		}
	}

	var result struct {
		Mode       string  `json:"mode"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		logger.Error("解析路由响应失败", zap.Error(err), zap.String("response", response))
		return LLMRouteResult{
			Mode:       contracts.ExecutionReact,
			Confidence: 0.4,
			Reason:     "JSON 解析失败，使用默认路由",
		}
	}

	// 规范化模式
	mode := r.normalizeMode(result.Mode)

	// 限制置信度范围
	confidence := result.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return LLMRouteResult{
		Mode:       mode,
		Confidence: confidence,
		Reason:     result.Reason,
	}
}

// extractJSON 从响应中提取 JSON
func (r *LLMRouter) extractJSON(response string) string {
	// 尝试找 ```json ... ```
	if start := strings.Index(response, "```json"); start != -1 {
		start += 7
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// 尝试找 { ... }
	if start := strings.Index(response, "{"); start != -1 {
		if end := strings.LastIndex(response, "}"); end != -1 {
			return response[start : end+1]
		}
	}

	return ""
}

// normalizeMode 规范化模式名称。
// 合法模式：react（及其别名）、plan_execute（及其别名）。
// 只有空值或未知值才降级为 React。
func (r *LLMRouter) normalizeMode(mode string) contracts.ExecutionMode {
	mode = strings.ToLower(strings.TrimSpace(mode))

	switch mode {
	// React 模式及其别名
	case "react", "re_act", "reactive":
		return contracts.ExecutionReact

	// Plan-Execute 模式及其别名
	case "plan_execute", "plan-execute", "planexecute", "plan":
		return contracts.ExecutionPlanExecute

	// 空值或未知值降级为 React
	default:
		logger.Warn("未知路由模式，降级为 React",
			zap.String("original_mode", mode),
		)
		return contracts.ExecutionReact
	}
}
