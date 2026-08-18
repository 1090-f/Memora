package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino/adk"
	"gopkg.in/yaml.v3"

	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// SystemPromptConfig 系统提示词配置。
type SystemPromptConfig struct {
	System string `yaml:"system"`
}

// contextBuilder 是 contracts.ContextBuilder 接口的实现。
// 使用固定插槽 + 结构化标签策略组装 AgentContext。
type contextBuilder struct {
	agentConfigRepo     repository.AgentConfigRepository
	convCtxService      contracts.ConversationContextService
	memoryRetriever     contracts.MemoryRetriever
	retrievalSvc        contracts.RetrievalService
	mcpToolRefresher    *tools.MCPToolRefresher // 可选：MCP 工具刷新器（第一层校验）
	toolRegistry        contracts.ToolRegistry  // 可选：工具注册表，用于检测已注册的 MCP 工具
	toolsConfigBuilder  func() adk.ToolsConfig  // 可选：构建本次运行的 ADK 工具配置
	defaultSystemPrompt string                  // 默认系统提示词（从文件加载）
}

// NewContextBuilder 创建新的上下文构建器实例。
func NewContextBuilder(
	agentConfigRepo repository.AgentConfigRepository,
	convCtxService contracts.ConversationContextService,
	memoryRetriever contracts.MemoryRetriever,
	retrievalSvc contracts.RetrievalService,
	toolRegistry contracts.ToolRegistry,
) contracts.ContextBuilder {
	// 加载默认系统提示词
	defaultPrompt := loadDefaultSystemPrompt()

	return &contextBuilder{
		agentConfigRepo:     agentConfigRepo,
		convCtxService:      convCtxService,
		memoryRetriever:     memoryRetriever,
		retrievalSvc:        retrievalSvc,
		toolRegistry:        toolRegistry,
		defaultSystemPrompt: defaultPrompt,
	}
}

// loadDefaultSystemPrompt 从文件加载默认系统提示词。
func loadDefaultSystemPrompt() string {
	data, err := os.ReadFile("internal/ai/prompts/system_prompt.yaml")
	if err != nil {
		logger.Warn("加载默认系统提示词失败，将使用数据库配置", zap.Error(err))
		return ""
	}

	var config SystemPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		logger.Warn("解析默认系统提示词失败，将使用数据库配置", zap.Error(err))
		return ""
	}

	logger.Info("成功加载默认系统提示词")
	return config.System
}

// SetMCPToolRefresher 注入 MCP 工具刷新器，用于 Agent 启动前的第一层校验。
func (b *contextBuilder) SetMCPToolRefresher(refresher *tools.MCPToolRefresher) {
	b.mcpToolRefresher = refresher
}

// SetToolsConfigBuilder 注入 ADK 工具配置构建器。
func (b *contextBuilder) SetToolsConfigBuilder(builder func() adk.ToolsConfig) {
	b.toolsConfigBuilder = builder
}

func (b *contextBuilder) Build(ctx context.Context, req contracts.AgentContextRequest) (contracts.AgentContext, error) {
	// ====== 双层校验机制的第一层：Agent 启动前刷新 MCP 工具列表 ======
	// 在构建上下文之前，先刷新该用户的 MCP 工具列表。
	// 从数据库查询用户已启用的 MCP Server 和 Tool，只把已启用的工具注册到工具表，
	// 这样模型只能看到已启用的工具，避免尝试调用被禁用的工具。
	// 注：这里不会阻塞整个请求，即使刷新失败也继续执行（降级处理）。
	if b.mcpToolRefresher != nil {
		// 使用带超时的刷新，防止 MCP 工具查询阻塞过久
		refreshErr := b.mcpToolRefresher.RefreshForUserWithTimeout(ctx, string(req.UserID), 3*time.Second)
		if refreshErr != nil {
			// 刷新失败不阻断核心流程，降级处理：Agent 将只使用内置工具
			logger.Warn("MCP 工具列表刷新失败，降级处理",
				zap.String("user_id", string(req.UserID)),
				zap.Error(refreshErr),
			)
		}
	}

	// 1. 加载 Agent 配置（必须存在）
	agentConfig, err := b.agentConfigRepo.FindByKnowledgeBase(ctx, string(req.UserID), string(req.KnowledgeBaseID))
	if err != nil {
		return contracts.AgentContext{}, fmt.Errorf("加载 Agent 配置失败: %w", err)
	}

	// 2. 构建基础上下文（固定插槽顺序）
	logger.Debug("Agent 配置从数据库加载",
		zap.Int("max_plan_steps", agentConfig.MaxPlanSteps),
		zap.Int("max_replans", agentConfig.MaxReplans),
		zap.Int("reviewer_runs", agentConfig.ReviewerRuns),
		zap.Bool("memory_enabled", agentConfig.MemoryEnabled),
		zap.Int("memory_top_k", agentConfig.MemoryTopK),
	)

	// 确定系统提示词：优先使用文件中的默认提示词，如果为空则从数据库加载
	systemPrompt := b.defaultSystemPrompt
	if systemPrompt == "" {
		systemPrompt = derefString(agentConfig.SystemPrompt)
	}

	agentCtx := contracts.AgentContext{
		// 基础信息
		UserID:          req.UserID,
		KnowledgeBaseID: req.KnowledgeBaseID,
		ConversationID:  req.ConversationID,
		RunID:           req.RunID,
		Query:           req.Query,

		// 系统配置（优先使用文件中的默认提示词）
		SystemPrompt:   systemPrompt,
		ChatModelID:    agentConfig.ChatModelID,
		NetworkEnabled: agentConfig.NetworkEnabled,
		MemoryEnabled:  agentConfig.MemoryEnabled,
		MaxReactRounds: agentConfig.MaxReactRounds,
		AllowedTools:   []string{}, // TODO: 从 AgentConfig 或工具表加载

		// Plan-Execute 模式配置（来自 AgentConfig）
		MaxPlanSteps: agentConfig.MaxPlanSteps,
		MaxReplans:   agentConfig.MaxReplans,
		ReviewerRuns: agentConfig.ReviewerRuns,

		// 对话上下文（固定插槽 - 可选）
		Conversation: contracts.ConversationContext{},

		// 记忆上下文（固定插槽 - 可选）
		Memories: []contracts.MemoryQueryResult{},
	}
	if b.toolsConfigBuilder != nil {
		agentCtx.ToolsConfig = b.toolsConfigBuilder()
	}

	// 3. 并行获取对话上下文、记忆和知识状态（如果启用）
	type convResult struct {
		ctx contracts.ConversationContext
		err error
	}
	type memResult struct {
		memories []contracts.MemoryQueryResult
		err      error
	}
	type retrievalResult struct {
		knowledgeStatus string
		err             error
	}

	convCh := make(chan convResult, 1)
	memCh := make(chan memResult, 1)
	retrievalCh := make(chan retrievalResult, 1)

	// 并行获取对话上下文
	go func() {
		convCtx, err := b.convCtxService.Build(ctx, req.UserID, req.KnowledgeBaseID, req.ConversationID)
		convCh <- convResult{ctx: convCtx, err: err}
	}()

	// 并行获取记忆（如果启用）
	go func() {
		if !agentConfig.MemoryEnabled {
			memCh <- memResult{memories: nil, err: nil}
			return
		}
		topK := agentConfig.MemoryTopK
		if topK <= 0 {
			topK = 5 // 默认值
		}
		memories, err := b.memoryRetriever.Retrieve(ctx, contracts.MemoryQuery{
			UserID:          req.UserID,
			KnowledgeBaseID: req.KnowledgeBaseID,
			Query:           req.Query,
			TopK:            topK,
		})
		memCh <- memResult{memories: memories, err: err}
	}()

	// 并行获取知识状态（通过检索服务）
	go func() {
		if b.retrievalSvc == nil {
			retrievalCh <- retrievalResult{knowledgeStatus: "", err: nil}
			return
		}
		result, err := b.retrievalSvc.Retrieve(ctx, contracts.RetrievalRequest{
			UserID:          req.UserID,
			KnowledgeBaseID: req.KnowledgeBaseID,
			Query:           req.Query,
			Mode:            contracts.RetrievalHybrid,
			TopK:            1, // 只需要知识状态，不需要具体内容
			Config:          contracts.DefaultSearchConfig(),
		})
		if err != nil {
			retrievalCh <- retrievalResult{knowledgeStatus: "", err: err}
			return
		}
		retrievalCh <- retrievalResult{knowledgeStatus: result.KnowledgeStatus, err: nil}
	}()

	// 等待对话上下文完成
	convRes := <-convCh
	if convRes.err != nil {
		return contracts.AgentContext{}, fmt.Errorf("构建对话上下文失败: %w", convRes.err)
	}
	agentCtx.Conversation = convRes.ctx

	// 等待记忆检索完成
	memRes := <-memCh
	if memRes.err != nil {
		// 记忆检索失败不影响核心功能，降级处理
		logger.Warn("记忆检索失败，降级处理",
			zap.Error(memRes.err),
		)
		agentCtx.Memories = nil
	} else {
		agentCtx.Memories = memRes.memories
	}

	// 等待知识状态检索完成
	retrievalRes := <-retrievalCh
	if retrievalRes.err != nil {
		// 检索失败不影响核心功能，降级处理
		logger.Warn("知识状态检索失败，降级处理",
			zap.Error(retrievalRes.err),
		)
		agentCtx.KnowledgeStatus = ""
	} else {
		agentCtx.KnowledgeStatus = retrievalRes.knowledgeStatus
	}

	// 自动启用网络：当注册表中存在 MCP 类型工具时，自动将 NetworkEnabled 设为 true。
	// 因为 MCP 工具（如 baidu_search）必然需要网络访问，且用户已在 MCP 管理页面显式启用了这些工具。
	if !agentCtx.NetworkEnabled && b.toolRegistry != nil {
		for _, spec := range b.toolRegistry.Specs() {
			if spec.Type == contracts.ToolTypeMCP && spec.Enabled {
				agentCtx.NetworkEnabled = true
				logger.Debug("自动启用网络：检测到已注册的 MCP 工具",
					zap.String("tool_name", spec.Name),
					zap.String("source_id", spec.SourceID),
				)
				break
			}
		}
	}

	// 日志：记录当前注册的工具数量（用于诊断工具注册问题）
	if b.mcpToolRefresher != nil {
		logger.Debug("AgentContext 构建完成，注册表工具状态",
			zap.Any("allowed_tools", agentCtx.AllowedTools),
			zap.Bool("network_enabled", agentCtx.NetworkEnabled),
			zap.String("chat_model_id", agentCtx.ChatModelID),
		)
	}

	return agentCtx, nil
}

// derefString 解引用指针字符串，如果为 nil 返回空字符串。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
