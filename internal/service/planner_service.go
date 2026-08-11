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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// PlannerPromptConfig 定义 Planner Prompt 配置。
type PlannerPromptConfig struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// PlannerService 实现 contracts.Planner 接口。
type PlannerService struct {
	modelFactory contracts.ModelFactory
	promptConfig PlannerPromptConfig
	maxSteps     int
}

// NewPlannerService 创建 PlannerService 实例。
func NewPlannerService(modelFactory contracts.ModelFactory, promptConfigPath string, maxSteps int) (*PlannerService, error) {
	// 读取 Prompt 配置
	config, err := loadPlannerPromptConfig(promptConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load planner prompt config: %w", err)
	}

	return &PlannerService{
		modelFactory: modelFactory,
		promptConfig: *config,
		maxSteps:     maxSteps,
	}, nil
}

// loadPlannerPromptConfig 加载 Planner Prompt 配置。
func loadPlannerPromptConfig(path string) (*PlannerPromptConfig, error) {
	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read planner prompt config: %w", err)
	}

	var config PlannerPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal planner prompt config: %w", err)
	}

	return &config, nil
}

// Plan 实现 contracts.Planner 接口。
func (s *PlannerService) Plan(ctx context.Context, agentContext contracts.AgentContext, config contracts.AgentConfig) (contracts.Plan, error) {
	start := time.Now()

	logger.Info("开始生成计划",
		zap.String("query", agentContext.Query),
		zap.Int("max_steps", config.MaxPlanSteps),
	)

	// 构建 Prompt
	prompt, err := s.buildPrompt(agentContext, config)
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("build prompt: %w", err)
	}

	// 调用模型
	model, err := s.modelFactory.GetChatModel(ctx, contracts.ID(agentContext.ChatModelID))
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("get chat model: %w", err)
	}

	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: s.promptConfig.System},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("chat with model: %w", err)
	}

	// 解析响应
	plan, err := s.parseResponse(response.Content, agentContext.RunID)
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("parse response: %w", err)
	}

	// 校验计划
	if err := s.validatePlan(plan, config); err != nil {
		return contracts.Plan{}, fmt.Errorf("validate plan: %w", err)
	}

	logger.Info("计划生成完成",
		zap.String("plan_id", string(plan.ID)),
		zap.Int("steps", len(plan.Steps)),
		zap.Duration("elapsed", time.Since(start)),
	)

	return plan, nil
}

// buildPrompt 构建 Prompt。
func (s *PlannerService) buildPrompt(agentContext contracts.AgentContext, config contracts.AgentConfig) (string, error) {
	tmpl, err := template.New("planner").Parse(s.promptConfig.User)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// 构建上下文
	context := agentContext.ToPromptWithTagsCompact()

	data := struct {
		Query    string
		Context  string
		MaxSteps int
	}{
		Query:    agentContext.Query,
		Context:  context,
		MaxSteps: config.MaxPlanSteps,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// parseResponse 解析模型响应。
func (s *PlannerService) parseResponse(response string, runID contracts.ID) (contracts.Plan, error) {
	// 提取 JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return contracts.Plan{}, fmt.Errorf("no JSON found in response")
	}

	// 解析 JSON
	var result struct {
		Goal               string   `json:"goal"`
		CompletionCriteria []string `json:"completion_criteria"`
		Steps              []struct {
			StepNo             int    `json:"step_no"`
			Title              string `json:"title"`
			Description        string `json:"description"`
			DependsOn          []int  `json:"depends_on"`
			ToolHint           string `json:"tool_hint"`
			CompletionCriteria string `json:"completion_criteria"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return contracts.Plan{}, fmt.Errorf("unmarshal plan: %w", err)
	}

	// 转换为 contracts.Plan
	plan := contracts.Plan{
		ID:                 contracts.ID(generateUUID()),
		RunID:              runID,
		Version:            1,
		Goal:               result.Goal,
		CompletionCriteria: result.CompletionCriteria,
		Status:             contracts.PlanPending,
		Steps:              make([]contracts.PlanStep, len(result.Steps)),
	}

	for i, step := range result.Steps {
		plan.Steps[i] = contracts.PlanStep{
			ID:                 contracts.ID(generateUUID()),
			StepNo:             step.StepNo,
			Title:              step.Title,
			Description:        step.Description,
			DependsOn:          step.DependsOn,
			ToolHint:           step.ToolHint,
			CompletionCriteria: step.CompletionCriteria,
			Status:             contracts.StepPending,
		}
	}

	return plan, nil
}

// validatePlan 校验计划。
func (s *PlannerService) validatePlan(plan contracts.Plan, config contracts.AgentConfig) error {
	// 检查步骤数量
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan must have at least one step")
	}

	if len(plan.Steps) > config.MaxPlanSteps {
		return fmt.Errorf("plan has %d steps, max allowed is %d", len(plan.Steps), config.MaxPlanSteps)
	}

	// 检查步骤序号连续性
	for i, step := range plan.Steps {
		expectedNo := i + 1
		if step.StepNo != expectedNo {
			return fmt.Errorf("step %d has invalid step_no %d, expected %d", i, step.StepNo, expectedNo)
		}
	}

	// 检查依赖关系
	for _, step := range plan.Steps {
		for _, dep := range step.DependsOn {
			if dep < 1 || dep >= step.StepNo {
				return fmt.Errorf("step %d has invalid dependency %d", step.StepNo, dep)
			}
		}
	}

	// 检查标题
	for _, step := range plan.Steps {
		if step.Title == "" {
			return fmt.Errorf("step %d must have a title", step.StepNo)
		}
	}

	return nil
}

// extractJSON 从文本中提取 JSON。
func extractJSON(text string) string {
	// 尝试找到 JSON 块
	start := strings.Index(text, "```json")
	if start != -1 {
		start += len("```json")
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// 尝试找到 JSON 对象
	start = strings.Index(text, "{")
	if start != -1 {
		end := strings.LastIndex(text, "}")
		if end != -1 {
			return text[start : end+1]
		}
	}

	return text
}

// generateUUID 生成 UUID。
func generateUUID() string {
	return uuid.New().String()
}
