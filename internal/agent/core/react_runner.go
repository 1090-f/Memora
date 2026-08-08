package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/compose"
)

// ReactRunner 执行有限轮次的 ReAct 循环。
type ReactRunner interface {
	Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error)
}

type reactState struct {
	Request  contracts.ChatRequest
	Response contracts.ChatResponse
}

// reactRunner 使用 Eino Graph 封装每次模型生成节点，工具仍统一通过 ToolExecutor 执行。
type reactRunner struct {
	model     contracts.ChatModel
	executor  contracts.ToolExecutor
	registry  *tools.Registry
	budget    BudgetController
	collector CitationCollector
}

// NewReactRunner 创建 ReAct 执行器，所有外部依赖必须由调用方注入。
func NewReactRunner(model contracts.ChatModel, executor contracts.ToolExecutor, registry *tools.Registry) ReactRunner {
	return &reactRunner{model: model, executor: executor, registry: registry, collector: NewCitationCollector()}
}

func (r *reactRunner) Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error) {
	if r == nil || r.model == nil || r.executor == nil || r.registry == nil {
		return RunOutput{}, newCoreError(contracts.ErrInternal, ErrExecutionDependency)
	}
	cfg = withDefaults(cfg)
	budget := r.budget
	if budget == nil {
		budget = DefaultBudgetController{Config: cfg}
	}
	r.collector.Reset()
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.MaxRunSeconds)*time.Second)
	defer cancel()

	messages := initialMessages(agentCtx)
	var usage contracts.TokenUsage
	var final string
	calls := 0
	for round := 1; round <= cfg.MaxReactRounds; round++ {
		if err := ctx.Err(); err != nil {
			return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage}, err
		}
		if err := budget.CheckRunDuration(startedAt); err != nil {
			return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage}, err
		}
		if err := budget.CheckReactRounds(round); err != nil {
			return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage}, err
		}
		definitions, err := r.registry.EinoTools(ctx, agentCtx.AllowedTools)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrResourceNotFound, err)
		}
		toolJSON, err := json.Marshal(definitions)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
		state := reactState{Request: contracts.ChatRequest{Messages: messages, Tools: toolJSON}}
		graph := compose.NewGraph[reactState, reactState]()
		if err := graph.AddLambdaNode("chat_model", compose.InvokableLambda(func(nodeCtx context.Context, input reactState) (reactState, error) {
			response, callErr := r.model.Generate(nodeCtx, input.Request)
			input.Response = response
			return input, callErr
		})); err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
		if err := graph.AddEdge(compose.START, "chat_model"); err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
		if err := graph.AddEdge("chat_model", compose.END); err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
		runnable, err := graph.Compile(ctx)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrInternal, err)
		}
		state, err = runnable.Invoke(ctx, state)
		if err != nil {
			return RunOutput{}, newCoreError(contracts.ErrModelCallFailed, err)
		}
		usage = addUsage(usage, state.Response.Usage)
		if err := budget.CheckTokenUsage(usage); err != nil {
			return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage}, err
		}
		messages = append(messages, contracts.ChatMessage{Role: "assistant", Content: state.Response.Content})
		if len(state.Response.ToolCalls) == 0 {
			final = strings.TrimSpace(state.Response.Content)
			break
		}
		for _, call := range state.Response.ToolCalls {
			calls++
			if err := budget.CheckToolCalls(calls); err != nil {
				return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage}, err
			}
			result, callErr := r.executor.Execute(ctx, contracts.ToolContext{UserID: agentCtx.UserID, KnowledgeBaseID: agentCtx.KnowledgeBaseID, AgentRunID: agentCtx.RunID, ReactRound: round, AllowedToolNames: agentCtx.AllowedTools, MaxResultBytes: cfg.MaxToolResultBytes}, call)
			r.collector.Add(result.Citations)
			messages = append(messages, contracts.ChatMessage{Role: "tool", Content: toolContent(result)})
			if callErr != nil {
				continue
			}
		}
	}
	if final == "" {
		return RunOutput{Citations: r.collector.Get(), Usage: usage}, fmt.Errorf("%w: no final answer", ErrBudgetExceeded)
	}
	return RunOutput{FinalResult: final, Citations: r.collector.Get(), Usage: usage, Summary: final}, nil
}

func initialMessages(agentCtx contracts.AgentContext) []contracts.ChatMessage {
	messages := make([]contracts.ChatMessage, 0, len(agentCtx.Conversation.Messages)+1)
	for _, message := range agentCtx.Conversation.Messages {
		messages = append(messages, contracts.ChatMessage{Role: message.Role, Content: message.Content})
	}
	messages = append(messages, contracts.ChatMessage{Role: "user", Content: agentCtx.Query})
	return messages
}

func toolContent(result contracts.ToolResult) string {
	data, err := json.Marshal(result)
	if err != nil {
		return result.ErrorMessage
	}
	return string(data)
}

func addUsage(total, current contracts.TokenUsage) contracts.TokenUsage {
	total.InputTokens += current.InputTokens
	total.OutputTokens += current.OutputTokens
	total.TotalTokens += current.TotalTokens
	return total
}

func withDefaults(cfg contracts.AgentConfig) contracts.AgentConfig {
	defaults := contracts.DefaultAgentConfig()
	if cfg.MaxReactRounds <= 0 {
		cfg.MaxReactRounds = defaults.MaxReactRounds
	}
	if cfg.MaxPlanSteps <= 0 {
		cfg.MaxPlanSteps = defaults.MaxPlanSteps
	}
	if cfg.MaxToolCalls <= 0 {
		cfg.MaxToolCalls = defaults.MaxToolCalls
	}
	if cfg.MaxToolResultBytes <= 0 {
		cfg.MaxToolResultBytes = defaults.MaxToolResultBytes
	}
	if cfg.MaxRunSeconds <= 0 {
		cfg.MaxRunSeconds = defaults.MaxRunSeconds
	}
	return cfg
}
