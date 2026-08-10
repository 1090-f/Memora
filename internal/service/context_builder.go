package service

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
)

// contextBuilder 是 contracts.ContextBuilder 接口的实现。
// 使用固定插槽 + 结构化标签策略组装 AgentContext。
type contextBuilder struct {
	agentConfigRepo repository.AgentConfigRepository
	convCtxService  contracts.ConversationContextService
	memoryRetriever contracts.MemoryRetriever
}

// NewContextBuilder 创建新的上下文构建器实例。
func NewContextBuilder(
	agentConfigRepo repository.AgentConfigRepository,
	convCtxService contracts.ConversationContextService,
	memoryRetriever contracts.MemoryRetriever,
) contracts.ContextBuilder {
	return &contextBuilder{
		agentConfigRepo: agentConfigRepo,
		convCtxService:  convCtxService,
		memoryRetriever: memoryRetriever,
	}
}

// Build 根据请求构建 AgentContext。
// 采用固定优先级插槽策略组装上下文。
func (b *contextBuilder) Build(ctx context.Context, req contracts.AgentContextRequest) (contracts.AgentContext, error) {
	// 1. 加载 Agent 配置（必须存在）
	agentConfig, err := b.agentConfigRepo.FindByKnowledgeBase(ctx, string(req.UserID), string(req.KnowledgeBaseID))
	if err != nil {
		return contracts.AgentContext{}, fmt.Errorf("加载 Agent 配置失败: %w", err)
	}

	// 2. 构建基础上下文（固定插槽顺序）
	agentCtx := contracts.AgentContext{
		// 基础信息
		UserID:          req.UserID,
		KnowledgeBaseID: req.KnowledgeBaseID,
		ConversationID:  req.ConversationID,
		RunID:           req.RunID,
		Query:           req.Query,

		// 系统配置（来自 AgentConfig）
		SystemPrompt:   derefString(agentConfig.SystemPrompt),
		NetworkEnabled: agentConfig.NetworkEnabled,
		MemoryEnabled:  agentConfig.MemoryEnabled,
		MaxReactRounds: agentConfig.MaxReactRounds,
		MaxPlanSteps:   agentConfig.MaxPlanSteps,
		AllowedTools:   []string{}, // TODO: 从 AgentConfig 或工具表加载

		// 对话上下文（固定插槽 - 可选）
		Conversation: contracts.ConversationContext{},

		// 记忆上下文（固定插槽 - 可选）
		Memories: []contracts.MemoryQueryResult{},
	}

	// 3. 并行获取对话上下文和记忆（如果启用）
	type convResult struct {
		ctx contracts.ConversationContext
		err error
	}
	type memResult struct {
		memories []contracts.MemoryQueryResult
		err      error
	}

	convCh := make(chan convResult, 1)
	memCh := make(chan memResult, 1)

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
		fmt.Printf("警告: 记忆检索失败，降级处理: %v\n", memRes.err)
		agentCtx.Memories = nil
	} else {
		agentCtx.Memories = memRes.memories
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
