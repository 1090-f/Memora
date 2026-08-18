package contracts

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

// AgentContextRequest 表示构建 Agent 运行执行上下文的请求。
type AgentContextRequest struct {
	UserID          ID     `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id"` // 知识库标识
	ConversationID  ID     `json:"conversation_id"`   // 会话标识
	RunID           ID     `json:"run_id"`            // 运行标识
	Query           string `json:"query"`             // 用户查询
}

// AgentContext 包含 Agent 执行运行所需的全部信息。
type AgentContext struct {
	// 基础信息（固定插槽 - 必需）
	UserID          ID     `json:"user_id"`           // 用户标识
	KnowledgeBaseID ID     `json:"knowledge_base_id"` // 知识库标识
	ConversationID  ID     `json:"conversation_id"`   // 会话标识
	RunID           ID     `json:"run_id"`            // 运行标识
	Query           string `json:"query"`             // 用户查询

	// 系统配置（来自 AgentConfig）
	SystemPrompt     string            `json:"system_prompt"`     // 系统提示词
	ChatModelID      string            `json:"chat_model_id"`     // ChatModelID 关联的对话模型配置 ID
	NetworkEnabled   bool              `json:"network_enabled"`   // 是否允许联网搜索
	MemoryEnabled    bool              `json:"memory_enabled"`    // 是否启用记忆功能
	MaxReactRounds   int               `json:"max_react_rounds"`  // ReAct 模式最大轮数
	AllowedTools     []string          `json:"allowed_tools"`     // 允许使用的工具白名单
	ToolDescriptions map[string]string `json:"tool_descriptions"` // 工具名到描述的映射，供 Planner 生成计划时参考
	AvailableTools   []ToolSpec        `json:"-"`
	ToolsConfig      adk.ToolsConfig   `json:"-"` // 当前运行可用的 ADK 工具配置

	// Plan-Execute 模式配置（来自 AgentConfig）
	MaxPlanSteps int `json:"max_plan_steps"` // Plan-Execute 最大步骤数 (default: 5)
	MaxReplans   int `json:"max_replans"`    // Plan-Execute 最大重规划次数 (default: 1)
	ReviewerRuns int `json:"reviewer_runs"`  // Plan-Execute 审查次数 (default: 1)

	// 对话上下文（固定插槽 - 可选）
	Conversation ConversationContext `json:"conversation"` // 会话历史上下文

	// 记忆上下文（固定插槽 - 可选）
	Memories []MemoryQueryResult `json:"memories"` // 检索到的记忆

	// 知识状态（来自检索结果）
	KnowledgeStatus string `json:"knowledge_status"` // 知识充分性状态
}

// ContextBuilder 根据请求构建 AgentContext。
type ContextBuilder interface {
	// Build 根据给定的请求构建 AgentContext。
	Build(ctx context.Context, request AgentContextRequest) (AgentContext, error)
}

// ToPromptWithTags 将 AgentContext 转换为带有结构化标签的提示词格式。
// 这样下游消费者（Router, ReAct, Plan-Execute）可以按标签解析并选择性使用上下文。
func (c AgentContext) ToPromptWithTags() string {
	var prompt string

	// [System] 系统提示词
	if c.SystemPrompt != "" {
		prompt += "[System]\n" + c.SystemPrompt + "\n\n"
	}

	// [Query] 用户当前问题
	prompt += "[Query]\n" + c.Query + "\n\n"

	// [History] 会话历史（最近 N 轮）
	if len(c.Conversation.Messages) > 0 {
		prompt += "[History]\n"
		for _, msg := range c.Conversation.Messages {
			prompt += msg.Role + ": " + msg.Content + "\n"
		}
		prompt += "\n"
	}

	// [Memory] 相关长期记忆（Top K 条）
	if len(c.Memories) > 0 {
		prompt += "[Memory]\n"
		for i, mem := range c.Memories {
			prompt += string(mem.MemoryType) + " " + string(rune('0'+i+1)) + ": " + mem.Content + "\n"
		}
		prompt += "\n"
	}

	// [Config] 元数据
	prompt += "[Config]\n"
	prompt += "UserID: " + string(c.UserID) + "\n"
	prompt += "KnowledgeBaseID: " + string(c.KnowledgeBaseID) + "\n"
	if c.NetworkEnabled {
		prompt += "Network: enabled\n"
	}
	if c.MemoryEnabled {
		prompt += "Memory: enabled\n"
	}

	return prompt
}

// ToPromptWithTagsCompact 将 AgentContext 转换为紧凑的标签格式（用于 token 有限的场景）。
// 注意：SystemPrompt 不在此处注入 —— 它只属于 system role，不应作为上下文文本传递给 Planner。
func (c AgentContext) ToPromptWithTagsCompact() string {
	var prompt string

	// 会话历史（只取最后 3 条，节省 token）
	if len(c.Conversation.Messages) > 0 {
		start := len(c.Conversation.Messages) - 3
		if start < 0 {
			start = 0
		}
		prompt += "[对话历史]\n"
		for _, msg := range c.Conversation.Messages[start:] {
			prompt += msg.Role + ": " + msg.Content + "\n"
		}
		prompt += "\n"
	}

	// 记忆（只取前 2 条，节省 token）
	if len(c.Memories) > 0 {
		limit := 2
		if len(c.Memories) < limit {
			limit = len(c.Memories)
		}
		prompt += "[相关记忆]\n"
		for _, mem := range c.Memories[:limit] {
			prompt += "- " + mem.Content + "\n"
		}
		prompt += "\n"
	}

	// 用户问题
	prompt += "[问题]\n" + c.Query

	return prompt
}
