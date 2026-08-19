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
// 优先识别复杂任务，避免被简单关键词提前命中
func (r *HybridRouter) routeByRules(agentCtx contracts.AgentContext) (contracts.ExecutionMode, float64, bool) {
	query := strings.ToLower(agentCtx.Query)

	// 规则 1: 优先识别复杂任务特征 → Plan-Execute
	// 复杂特征包括：多个动作、依赖关系、多工具协作、明确要求制定计划
	if isComplexTask(query) {
		return contracts.ExecutionPlanExecute, 0.8, true
	}

	// 规则 2: 包含计划类关键词 → Plan-Execute（如果未被复杂特征检测覆盖）
	if containsPlanKeywords(query) {
		return contracts.ExecutionPlanExecute, 0.75, true
	}

	// 规则 3: 包含工具调用关键词 → React（可能需要工具，但不是多步骤复杂任务）
	if containsToolKeywords(query) {
		return contracts.ExecutionReact, 0.7, true
	}

	// 规则 4: 包含明确的简单查询关键词 → React
	// 这些关键词明确表示简单查询，但必须在复杂特征检测之后
	if containsSimpleKeywords(query) {
		return contracts.ExecutionReact, 0.85, true
	}

	// 未匹配任何规则
	return contracts.ExecutionReact, 0.5, false
}

// isComplexTask 检查是否为复杂任务
// 复杂任务特征：多个动作、依赖关系、多工具协作、明确要求制定计划
func isComplexTask(query string) bool {
	// 特征 1: 包含多个动作词（表示需要多个步骤）
	if hasMultipleActions(query) {
		return true
	}

	// 特征 2: 包含依赖关系词（表示步骤之间有依赖）
	if hasDependencyRelations(query) {
		return true
	}

	// 特征 3: 包含多个工具调用词（表示需要多个工具协作）
	if hasMultipleToolCalls(query) {
		return true
	}

	// 特征 4: 明确要求制定计划
	if explicitlyRequiresPlan(query) {
		return true
	}

	// 特征 5: 包含多个问号（表示多个问题）
	if hasMultipleQuestions(query) {
		return true
	}

	return false
}

// hasMultipleActions 检查是否包含多个动作词
func hasMultipleActions(query string) bool {
	actionWords := []string{
		"搜索", "查找", "查询", "检索",
		"分析", "统计", "计算",
		"总结", "概括", "归纳",
		"对比", "比较", "区别",
		"整理", "梳理", "分类",
		"生成", "创建", "制作",
		"编写", "撰写", "写",
		"翻译", "解释", "说明",
	}
	count := 0
	for _, word := range actionWords {
		if strings.Contains(query, word) {
			count++
		}
	}
	return count >= 2
}

// hasDependencyRelations 检查是否包含依赖关系词
func hasDependencyRelations(query string) bool {
	dependencyWords := []string{
		"然后", "接着", "之后", "最后",
		"并且", "同时", "而且",
		"之后再", "然后在", "接着再",
		"搜索后", "分析后", "总结后",
		"基于.*结果", "根据.*输出",
	}
	return containsAny(query, dependencyWords)
}

// hasMultipleToolCalls 检查是否包含多个工具调用词
func hasMultipleToolCalls(query string) bool {
	// 定义工具调用词组
	toolGroups := []string{
		"搜索", "查找", "查询", "检索",
		"分析", "统计", "计算",
		"总结", "概括", "归纳",
		"对比", "比较", "区别",
		"整理", "梳理", "分类",
	}

	// 检查是否包含多个不同的工具调用词
	count := 0
	for _, tool := range toolGroups {
		if strings.Contains(query, tool) {
			count++
		}
	}
	return count >= 2
}

// explicitlyRequiresPlan 检查是否明确要求制定计划
func explicitlyRequiresPlan(query string) bool {
	// 定义计划相关关键词
	planKeywords := []string{
		"制定", "规划", "安排", "组织",
		"计划", "方案", "流程", "步骤",
		"分步", "逐步", "依次",
	}

	// 定义计划类型词
	planTypes := []string{
		"计划", "方案", "流程", "步骤",
	}

	// 检查是否同时包含计划动词和计划类型词
	hasPlanVerb := false
	hasPlanType := false

	for _, verb := range planKeywords {
		if strings.Contains(query, verb) {
			hasPlanVerb = true
			break
		}
	}

	for _, planType := range planTypes {
		if strings.Contains(query, planType) {
			hasPlanType = true
			break
		}
	}

	// 如果同时包含计划动词和计划类型词，则认为是明确要求制定计划
	if hasPlanVerb && hasPlanType {
		return true
	}

	// 检查是否包含明确的计划请求词
	explicitPlanRequests := []string{
		"分步执行", "逐步执行", "依次执行",
		"第一步", "第二步", "第三步",
	}
	return containsAny(query, explicitPlanRequests)
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
