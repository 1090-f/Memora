package service

import (
	"context"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// 混合路由配置常量
const (
	// RuleConfidenceThreshold 规则置信度阈值，超过此值直接使用规则结果
	RuleConfidenceThreshold = 0.75
	// LLMConfidenceThreshold LLM 置信度阈值，超过此值使用 LLM 结果
	LLMConfidenceThreshold = 0.7
	// DefaultConfidence 默认置信度（兜底策略）
	DefaultConfidence = 0.5
	// MultiQuestionThreshold 多问题阈值：包含多个问号视为复杂问题
	MultiQuestionThreshold = 2
	// MultiConjunctionThreshold 多连接词阈值：包含多个连接词视为复合问题
	MultiConjunctionThreshold = 2
)

// HybridRouter 混合路由器
type HybridRouter struct {
	llmRouter *LLMRouter
}

// NewHybridRouter 创建混合路由器
func NewHybridRouter(llmRouter *LLMRouter) *HybridRouter {
	return &HybridRouter{
		llmRouter: llmRouter,
	}
}

// Route 执行混合路由决策
func (r *HybridRouter) Route(ctx context.Context, agentCtx contracts.AgentContext) (contracts.RouterDecision, error) {
	start := time.Now()

	logger.Info("开始路由决策",
		zap.String("query", agentCtx.Query),
		zap.String("user_id", string(agentCtx.UserID)),
	)

	// 第一层: 规则快速判断
	ruleMode, ruleConfidence, ruleMatched := r.routeByRules(agentCtx)
	if ruleMatched && ruleConfidence >= RuleConfidenceThreshold {
		logger.Info("规则路由命中",
			zap.String("mode", string(ruleMode)),
			zap.Float64("confidence", ruleConfidence),
			zap.Duration("elapsed", time.Since(start)),
		)
		return contracts.RouterDecision{
			ExecutionMode: ruleMode,
			ReasonSummary: "规则匹配",
			Confidence:    ruleConfidence,
			FallbackUsed:  false,
			CreatedAt:     time.Now(),
		}, nil
	}

	// 第二层: LLM 判断（仅在规则不确定时调用）
	if r.llmRouter != nil {
		llmResult := r.llmRouter.Route(ctx, agentCtx)
		if llmResult.Error == nil && llmResult.Confidence >= LLMConfidenceThreshold {
			logger.Info("LLM 路由命中",
				zap.String("mode", string(llmResult.Mode)),
				zap.Float64("confidence", llmResult.Confidence),
				zap.String("reason", llmResult.Reason),
				zap.Duration("elapsed", time.Since(start)),
			)
			return contracts.RouterDecision{
				ExecutionMode: llmResult.Mode,
				ReasonSummary: llmResult.Reason,
				Confidence:    llmResult.Confidence,
				FallbackUsed:  false,
				CreatedAt:     time.Now(),
			}, nil
		}
		logger.Warn("LLM 路由置信度不足或失败",
			zap.Float64("confidence", llmResult.Confidence),
			zap.Error(llmResult.Error),
		)
	}

	// 第三层: 默认降级策略
	// 如果规则有匹配但置信度不够高，使用规则结果
	if ruleMatched {
		logger.Info("使用规则路由（置信度不足）",
			zap.String("mode", string(ruleMode)),
			zap.Float64("confidence", ruleConfidence),
			zap.Duration("elapsed", time.Since(start)),
		)
		return contracts.RouterDecision{
			ExecutionMode: ruleMode,
			ReasonSummary: "规则匹配（置信度不足，降级使用）",
			Confidence:    ruleConfidence,
			FallbackUsed:  true,
			CreatedAt:     time.Now(),
		}, nil
	}

	// 完全兜底: 默认使用 React
	logger.Info("使用默认路由（React）",
		zap.Duration("elapsed", time.Since(start)),
	)
	return contracts.RouterDecision{
		ExecutionMode: contracts.ExecutionReact,
		ReasonSummary: "默认路由",
		Confidence:    DefaultConfidence,
		FallbackUsed:  true,
		CreatedAt:     time.Now(),
	}, nil
}

// routeByRules 基于规则进行路由
func (r *HybridRouter) routeByRules(agentCtx contracts.AgentContext) (contracts.ExecutionMode, float64, bool) {
	query := strings.ToLower(agentCtx.Query)

	// 规则 1: 包含明确的计划类关键词 → Plan-Execute
	if containsPlanKeywords(query) {
		return contracts.ExecutionPlanExecute, 0.9, true
	}

	// 规则 2: 包含多个问号 → Plan-Execute（复合问题）
	if hasMultipleQuestions(query) {
		return contracts.ExecutionPlanExecute, 0.8, true
	}

	// 规则 3: 包含多个连接词 → Plan-Execute（复合任务）
	if hasMultipleConjunctions(query) {
		return contracts.ExecutionPlanExecute, 0.75, true
	}

	// 规则 4: 包含明确的简单查询关键词 → React
	if containsSimpleKeywords(query) {
		return contracts.ExecutionReact, 0.85, true
	}

	// 规则 5: 包含工具调用关键词 → React（可能需要工具）
	if containsToolKeywords(query) {
		return contracts.ExecutionReact, 0.7, true
	}

	// 未匹配任何规则
	return contracts.ExecutionReact, 0.5, false
}

// containsPlanKeywords 检查是否包含计划类关键词
func containsPlanKeywords(query string) bool {
	planKeywords := []string{
		"步骤", "首先", "然后", "接着", "最后",
		"计划", "规划", "方案", "流程",
		"分步", "逐步", "依次",
		"制定", "安排", "组织",
		"第一步", "第二步", "第三步",
	}
	return containsAny(query, planKeywords)
}

// containsSimpleKeywords 检查是否包含简单查询关键词
func containsSimpleKeywords(query string) bool {
	simpleKeywords := []string{
		"是什么", "什么是", "谁是", "谁",
		"哪里", "在哪", "什么时候", "几点",
		"多少", "几个", "几",
		"解释", "说明", "定义",
	}
	return containsAny(query, simpleKeywords)
}

// containsToolKeywords 检查是否包含工具调用关键词
func containsToolKeywords(query string) bool {
	toolKeywords := []string{
		"搜索", "查找", "查询", "检索",
		"计算", "统计", "分析",
		"对比", "比较", "区别",
		"总结", "概括", "归纳",
	}
	return containsAny(query, toolKeywords)
}

// hasMultipleQuestions 检查是否包含多个问号
func hasMultipleQuestions(query string) bool {
	count := strings.Count(query, "?") + strings.Count(query, "？")
	return count >= MultiQuestionThreshold
}

// hasMultipleConjunctions 检查是否包含多个连接词
func hasMultipleConjunctions(query string) bool {
	conjunctions := []string{"和", "与", "及", "或者", "还是", "同时", "并且"}
	count := 0
	for _, conj := range conjunctions {
		count += strings.Count(query, conj)
	}
	return count >= MultiConjunctionThreshold
}

// containsAny 检查是否包含任一关键词
func containsAny(query string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(query, kw) {
			return true
		}
	}
	return false
}

// routerService 是 contracts.Router 接口的实现
type routerService struct {
	hybridRouter *HybridRouter
}

// NewRouterService 创建路由器服务
func NewRouterService(llmRouter *LLMRouter) contracts.Router {
	return &routerService{
		hybridRouter: NewHybridRouter(llmRouter),
	}
}

// Route 实现 contracts.Router 接口
func (s *routerService) Route(ctx context.Context, agentCtx contracts.AgentContext) (contracts.RouterDecision, error) {
	return s.hybridRouter.Route(ctx, agentCtx)
}
