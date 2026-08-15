package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// StepEventCallback 是步骤事件回调函数的类型。
// 通过回调发布步骤事件，保持 service 层不依赖 core 包。
// runID: 运行 ID, stepNo: 步骤序号, title: 步骤标题
type StepEventCallback func(ctx context.Context, runID contracts.ID, stepNo int, title string) error

// PlanExecutorService 实现 contracts.PlanExecutor 接口。
type PlanExecutorService struct {
	toolExecutor   contracts.ToolExecutor
	stateStore     PlanStateStore
	modelFactory   contracts.ModelFactory
	maxToolCalls   int
	maxResultBytes int
	// 步骤事件回调，由 PlanRunner 在调用 Execute 时设置，用于实时发布步骤生命周期事件。
	onStepStarted   StepEventCallback
	onStepCompleted StepEventCallback
	onUsage         PlanUsageCallback // 可选：token 消耗回调
}

// SetUsageCallback 设置 token 消耗回调。
func (s *PlanExecutorService) SetUsageCallback(cb PlanUsageCallback) {
	s.onUsage = cb
}

// NewPlanExecutorService 创建 PlanExecutorService 实例。
func NewPlanExecutorService(
	toolExecutor contracts.ToolExecutor,
	stateStore PlanStateStore,
	modelFactory contracts.ModelFactory,
	maxToolCalls int,
	maxResultBytes int,
) *PlanExecutorService {
	return &PlanExecutorService{
		toolExecutor:   toolExecutor,
		stateStore:     stateStore,
		modelFactory:   modelFactory,
		maxToolCalls:   maxToolCalls,
		maxResultBytes: maxResultBytes,
	}
}

// SetStepEventCallbacks 设置步骤事件回调，用于发布步骤生命周期事件。
// 由 PlanRunner 在 Run 时调用，参考 ReAct 的回调模式设计。
func (s *PlanExecutorService) SetStepEventCallbacks(onStarted, onCompleted StepEventCallback) {
	s.onStepStarted = onStarted
	s.onStepCompleted = onCompleted
}

// Execute 实现 contracts.PlanExecutor 接口。
func (s *PlanExecutorService) Execute(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan) (contracts.Plan, error) {
	start := time.Now()

	logger.Info("开始执行计划",
		zap.String("plan_id", string(plan.ID)),
		zap.Int("steps", len(plan.Steps)),
	)

	// 更新计划状态为执行中
	if err := s.stateStore.UpdateStatus(ctx, plan.ID, contracts.PlanExecuting); err != nil {
		return plan, fmt.Errorf("update plan status: %w", err)
	}

	// 构建依赖图
	dependencyGraph := s.buildDependencyGraph(plan.Steps)

	// 拓扑排序，获取执行层级
	levels := s.topologicalSort(dependencyGraph)

	// 逐层执行
	for _, level := range levels {
		if err := ctx.Err(); err != nil {
			return plan, fmt.Errorf("context cancelled: %w", err)
		}

		// 同层可并行执行
		if err := s.executeLevel(ctx, agentContext, plan, level); err != nil {
			return plan, fmt.Errorf("execute level: %w", err)
		}
	}

	// 更新计划状态为完成
	if err := s.stateStore.UpdateStatus(ctx, plan.ID, contracts.PlanCompleted); err != nil {
		return plan, fmt.Errorf("update plan status: %w", err)
	}

	logger.Info("计划执行完成",
		zap.String("plan_id", string(plan.ID)),
		zap.Duration("elapsed", time.Since(start)),
	)

	return plan, nil
}

// executeLevel 执行同一层级的步骤。
func (s *PlanExecutorService) executeLevel(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan, steps []contracts.PlanStep) error {
	if len(steps) == 1 {
		// 单个步骤直接执行
		return s.executeStep(ctx, agentContext, plan, steps[0])
	}

	// 多个步骤并行执行
	g, ctx := errgroup.WithContext(ctx)

	for _, step := range steps {
		step := step // 捕获变量
		g.Go(func() error {
			return s.executeStep(ctx, agentContext, plan, step)
		})
	}

	return g.Wait()
}

// executeStep 执行单个步骤。
func (s *PlanExecutorService) executeStep(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan, step contracts.PlanStep) error {
	start := time.Now()

	logger.Info("开始执行步骤",
		zap.String("plan_id", string(plan.ID)),
		zap.Int("step_no", step.StepNo),
		zap.String("title", step.Title),
	)

	// 更新步骤状态为运行中
	if err := s.stateStore.UpdateStepStatus(ctx, plan.ID, step.StepNo, contracts.StepRunning); err != nil {
		return fmt.Errorf("update step status: %w", err)
	}

	// 发布步骤开始事件（前端据此实时更新步骤状态为 running）
	if s.onStepStarted != nil {
		if err := s.onStepStarted(ctx, plan.RunID, step.StepNo, step.Title); err != nil {
			logger.Warn("发布步骤开始事件失败",
				zap.String("plan_id", string(plan.ID)),
				zap.Int("step_no", step.StepNo),
				zap.Error(err),
			)
		}
	}

	// 使用 LLM 决定是否需要调用工具
	toolCall, err := s.decideToolCall(ctx, agentContext, plan, step)
	if err != nil {
		// 记录错误
		s.stateStore.RecordStepError(ctx, plan.ID, step.StepNo, err)
		return fmt.Errorf("decide tool call: %w", err)
	}

	// 如果需要调用工具
	if toolCall != nil {
		// 构建工具上下文
		toolContext := contracts.ToolContext{
			UserID:           agentContext.UserID,
			KnowledgeBaseID:  agentContext.KnowledgeBaseID,
			AgentRunID:       agentContext.RunID,
			PlanStepID:       step.ID,
			AllowedToolNames: agentContext.AllowedTools,
			NetworkEnabled:   agentContext.NetworkEnabled,
			MaxResultBytes:   s.maxResultBytes,
		}

		// 执行工具调用
		result, err := s.toolExecutor.Execute(ctx, toolContext, *toolCall)
		if err != nil {
			// 记录错误
			s.stateStore.RecordStepError(ctx, plan.ID, step.StepNo, err)
			return fmt.Errorf("execute tool: %w", err)
		}

		// 记录结果
		if err := s.stateStore.RecordStepResult(ctx, plan.ID, step.StepNo, toolCall.ToolName, result.Text, "", ""); err != nil {
			return fmt.Errorf("record step result: %w", err)
		}

		// 更新步骤状态为完成
		if err := s.stateStore.UpdateStepStatus(ctx, plan.ID, step.StepNo, contracts.StepCompleted); err != nil {
			return fmt.Errorf("update step status: %w", err)
		}
	} else {
		// 不需要调用工具，直接完成
		if err := s.stateStore.UpdateStepStatus(ctx, plan.ID, step.StepNo, contracts.StepCompleted); err != nil {
			return fmt.Errorf("update step status: %w", err)
		}
	}

	// 发布步骤完成事件（前端据此实时更新步骤状态为 completed）
	if s.onStepCompleted != nil {
		if err := s.onStepCompleted(ctx, plan.RunID, step.StepNo, step.Title); err != nil {
			logger.Warn("发布步骤完成事件失败",
				zap.String("plan_id", string(plan.ID)),
				zap.Int("step_no", step.StepNo),
				zap.Error(err),
			)
		}
	}

	logger.Info("步骤执行完成",
		zap.String("plan_id", string(plan.ID)),
		zap.Int("step_no", step.StepNo),
		zap.Duration("elapsed", time.Since(start)),
	)

	return nil
}

// decideToolCall 使用 LLM 决定是否需要调用工具。
func (s *PlanExecutorService) decideToolCall(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan, step contracts.PlanStep) (*contracts.ToolCall, error) {
	// 如果步骤没有工具提示，不调用工具
	if step.ToolHint == "" {
		return nil, nil
	}

	// 构建 Prompt
	prompt := s.buildToolDecisionPrompt(plan, step)

	// 调用模型
	model, err := s.modelFactory.GetChatModel(ctx, contracts.ID(agentContext.ChatModelID))
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: "你是一个工具调用决策助手。根据步骤描述决定是否需要调用工具，以及如何调用。"},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("chat with model: %w", err)
	}

	// 记录 token 消耗
	if s.onUsage != nil {
		s.onUsage(response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens)
	}

	// 解析响应
	toolCall, err := s.parseToolCallResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("parse tool call response: %w", err)
	}

	return toolCall, nil
}

// buildToolDecisionPrompt 构建工具决策 Prompt。
func (s *PlanExecutorService) buildToolDecisionPrompt(plan contracts.Plan, step contracts.PlanStep) string {
	return fmt.Sprintf(`请根据以下步骤描述决定是否需要调用工具：

[计划目标]
%s

[步骤信息]
序号: %d
标题: %s
描述: %s
工具提示: %s
完成判据: %s

[可用工具]
%s

请输出JSON格式的决策：
{
  "need_tool": true/false,
  "tool_name": "工具名称（如果需要）",
  "arguments": {"参数名": "参数值"}（如果需要）
}

如果不需要调用工具，请设置 need_tool 为 false。`,
		plan.Goal,
		step.StepNo,
		step.Title,
		step.Description,
		step.ToolHint,
		step.CompletionCriteria,
		s.getAvailableTools(),
	)
}

// parseToolCallResponse 解析工具调用响应。
func (s *PlanExecutorService) parseToolCallResponse(response string) (*contracts.ToolCall, error) {
	// 提取 JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, nil
	}

	// 解析 JSON
	var result struct {
		NeedTool  bool                   `json:"need_tool"`
		ToolName  string                 `json:"tool_name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("unmarshal tool call: %w", err)
	}

	if !result.NeedTool || result.ToolName == "" {
		return nil, nil
	}

	// 转换参数
	arguments, err := json.Marshal(result.Arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal arguments: %w", err)
	}

	return &contracts.ToolCall{
		CallID:    contracts.ID(generateUUID()),
		ToolName:  result.ToolName,
		Arguments: arguments,
	}, nil
}

// getAvailableTools 获取可用工具列表。
func (s *PlanExecutorService) getAvailableTools() string {
	// 这里应该从 ToolRegistry 获取，暂时返回空
	return "暂无可用工具信息"
}

// buildDependencyGraph 构建依赖图。
func (s *PlanExecutorService) buildDependencyGraph(steps []contracts.PlanStep) map[int][]int {
	graph := make(map[int][]int)
	for _, step := range steps {
		graph[step.StepNo] = step.DependsOn
	}
	return graph
}

// topologicalSort 拓扑排序。
func (s *PlanExecutorService) topologicalSort(graph map[int][]int) [][]contracts.PlanStep {
	// 计算入度
	inDegree := make(map[int]int)
	for node := range graph {
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep]++
		}
	}

	// 找出入度为0的节点
	var levels [][]contracts.PlanStep
	visited := make(map[int]bool)

	for {
		var currentLevel []int
		for node, degree := range inDegree {
			if degree == 0 && !visited[node] {
				currentLevel = append(currentLevel, node)
			}
		}

		if len(currentLevel) == 0 {
			break
		}

		// 标记已访问
		for _, node := range currentLevel {
			visited[node] = true
			delete(inDegree, node)
		}

		// 更新入度
		for _, node := range currentLevel {
			for _, dep := range graph[node] {
				inDegree[dep]--
			}
		}

		// 转换为 PlanStep（这里简化处理，实际应该从 plan.Steps 中获取）
		level := make([]contracts.PlanStep, len(currentLevel))
		for i, node := range currentLevel {
			level[i] = contracts.PlanStep{StepNo: node}
		}
		levels = append(levels, level)
	}

	return levels
}
