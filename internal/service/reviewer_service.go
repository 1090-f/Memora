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

// ReviewerPromptConfig 定义 Reviewer Prompt 配置。
type ReviewerPromptConfig struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// ReviewerService 实现 contracts.PlanReviewer 接口。
type ReviewerService struct {
	modelFactory contracts.ModelFactory
	promptConfig ReviewerPromptConfig
	stateStore   PlanStateStore
	onUsage      PlanUsageCallback // 可选：token 消耗回调
}

// SetUsageCallback 设置 token 消耗回调。
func (s *ReviewerService) SetUsageCallback(cb PlanUsageCallback) {
	s.onUsage = cb
}

// NewReviewerService 创建 ReviewerService 实例。
func NewReviewerService(modelFactory contracts.ModelFactory, promptConfigPath string, stateStore PlanStateStore) (*ReviewerService, error) {
	// 读取 Prompt 配置
	config, err := loadReviewerPromptConfig(promptConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load reviewer prompt config: %w", err)
	}

	return &ReviewerService{
		modelFactory: modelFactory,
		promptConfig: *config,
		stateStore:   stateStore,
	}, nil
}

// loadReviewerPromptConfig 加载 Reviewer Prompt 配置。
func loadReviewerPromptConfig(path string) (*ReviewerPromptConfig, error) {
	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read reviewer prompt config: %w", err)
	}

	var config ReviewerPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal reviewer prompt config: %w", err)
	}

	return &config, nil
}

// Review 实现 contracts.PlanReviewer 接口。
func (s *ReviewerService) Review(ctx context.Context, agentContext contracts.AgentContext, plan contracts.Plan) (contracts.ReviewerResult, error) {
	start := time.Now()

	logger.Info("开始审查计划",
		zap.String("plan_id", string(plan.ID)),
		zap.String("goal", plan.Goal),
	)

	// 构建 Prompt
	prompt, err := s.buildPrompt(plan)
	if err != nil {
		return contracts.ReviewerResult{}, fmt.Errorf("build prompt: %w", err)
	}

	// 调用模型
	model, err := s.modelFactory.GetChatModel(ctx, contracts.ID(agentContext.ChatModelID))
	if err != nil {
		return contracts.ReviewerResult{}, fmt.Errorf("get chat model: %w", err)
	}

	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: s.promptConfig.System},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return contracts.ReviewerResult{}, fmt.Errorf("chat with model: %w", err)
	}

	// 记录 token 消耗
	if s.onUsage != nil {
		s.onUsage(response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens)
	}

	// 解析响应
	result, err := s.parseResponse(response.Content)
	if err != nil {
		return contracts.ReviewerResult{}, fmt.Errorf("parse response: %w", err)
	}

	// 记录审查结果
	if err := s.stateStore.RecordReview(ctx, plan.ID, result); err != nil {
		logger.Error("Failed to record review result",
			zap.Error(err),
			zap.String("plan_id", string(plan.ID)),
		)
	}

	logger.Info("计划审查完成",
		zap.String("plan_id", string(plan.ID)),
		zap.String("result", result.Result),
		zap.Duration("elapsed", time.Since(start)),
	)

	return result, nil
}

// buildPrompt 构建 Prompt。
func (s *ReviewerService) buildPrompt(plan contracts.Plan) (string, error) {
	tmpl, err := template.New("reviewer").Parse(s.promptConfig.User)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// 构建步骤结果
	stepsResult := s.buildStepsResult(plan)

	// 构建完成判据
	completionCriteria := strings.Join(plan.CompletionCriteria, "\n- ")

	data := struct {
		Goal               string
		CompletionCriteria string
		StepsResult        string
	}{
		Goal:               plan.Goal,
		CompletionCriteria: completionCriteria,
		StepsResult:        stepsResult,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// buildStepsResult 构建步骤结果。
func (s *ReviewerService) buildStepsResult(plan contracts.Plan) string {
	var result strings.Builder

	for _, step := range plan.Steps {
		result.WriteString(fmt.Sprintf("步骤 %d: %s\n", step.StepNo, step.Title))
		result.WriteString(fmt.Sprintf("  状态: %s\n", step.Status))
		if step.Description != "" {
			result.WriteString(fmt.Sprintf("  描述: %s\n", step.Description))
		}
		if step.CompletionCriteria != "" {
			result.WriteString(fmt.Sprintf("  完成判据: %s\n", step.CompletionCriteria))
		}
		result.WriteString("\n")
	}

	return result.String()
}

// parseResponse 解析模型响应。
func (s *ReviewerService) parseResponse(response string) (contracts.ReviewerResult, error) {
	// 提取 JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return contracts.ReviewerResult{}, fmt.Errorf("no JSON found in response")
	}

	// 解析 JSON
	var result struct {
		Result  string `json:"result"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return contracts.ReviewerResult{}, fmt.Errorf("unmarshal review result: %w", err)
	}

	// 验证结果
	if result.Result == "" {
		result.Result = "completed"
	}

	if result.Summary == "" {
		result.Summary = "计划审查完成"
	}

	return contracts.ReviewerResult{
		Result:  result.Result,
		Summary: result.Summary,
	}, nil
}
