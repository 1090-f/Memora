package react

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/compose"
)

type graphState struct {
	Request  contracts.ChatRequest
	Response contracts.ChatResponse
}

func (a *agent) Run(ctx context.Context, agentContext contracts.AgentContext, config contracts.AgentConfig) (contracts.AgentRunResult, error) {
	startedAt := time.Now()
	config = defaults(config)
	if err := validate(agentContext, a.dependencies); err != nil {
		return contracts.AgentRunResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.MaxRunSeconds)*time.Second)
	defer cancel()
	a.dependencies.CitationCollector.Reset()
	state := State{AgentContext: agentContext, Config: config, Messages: buildMessages(agentContext)}
	for state.ReactRound = 1; state.ReactRound <= config.MaxReactRounds; state.ReactRound++ {
		if err := checkContext(ctx); err != nil {
			return result(state, startedAt), err
		}
		response, err := a.generate(ctx, state.Messages, agentContext.AllowedTools)
		if err != nil {
			return result(state, startedAt), wrap(contracts.ErrModelCallFailed, err)
		}
		state.Usage = mergeUsage(state.Usage, response.Usage)
		state.Messages = append(state.Messages, contracts.ChatMessage{Role: "assistant", Content: response.Content})
		if len(response.ToolCalls) == 0 {
			state.FinalResult = normalizeAnswer(response.Content)
			state.Citations = a.dependencies.CitationCollector.Get()
			if state.FinalResult == "" {
				return result(state, startedAt), wrap(contracts.ErrModelCallFailed, ErrInvalidResponse)
			}
			state.Completed = true
			return result(state, startedAt), nil
		}
		for _, call := range response.ToolCalls {
			state.ToolCalls++
			if state.ToolCalls > config.MaxToolCalls {
				return result(state, startedAt), wrap(contracts.ErrServiceUnavailable, ErrBudgetExceeded)
			}
			if err := validateCall(call, agentContext.AllowedTools); err != nil {
				return result(state, startedAt), err
			}
			toolContext := contracts.ToolContext{UserID: agentContext.UserID, KnowledgeBaseID: agentContext.KnowledgeBaseID, AgentRunID: agentContext.RunID, ReactRound: state.ReactRound, AllowedToolNames: agentContext.AllowedTools, MaxResultBytes: config.MaxToolResultBytes, NetworkEnabled: true}
			result, callErr := a.dependencies.ToolExecutor.Execute(ctx, toolContext, call)
			a.dependencies.CitationCollector.Add(result.Citations)
			state.Messages = append(state.Messages, contracts.ChatMessage{Role: "tool", Content: observation(result)})
			if callErr != nil {
				continue
			}
		}
	}
	return result(state, startedAt), wrap(contracts.ErrServiceUnavailable, ErrBudgetExceeded)
}

func (a *agent) generate(ctx context.Context, messages []contracts.ChatMessage, allowed []string) (contracts.ChatResponse, error) {
	definitions := make([]contracts.ToolSpec, 0, len(allowed))
	for _, spec := range a.dependencies.ToolRegistry.Specs() {
		if contains(allowed, spec.Name) && spec.Enabled && spec.ReadOnly {
			definitions = append(definitions, spec)
		}
	}
	toolsJSON, err := json.Marshal(definitions)
	if err != nil {
		return contracts.ChatResponse{}, err
	}
	graph := compose.NewGraph[graphState, graphState]()
	if err := graph.AddLambdaNode("chat_model", compose.InvokableLambda(func(nodeCtx context.Context, input graphState) (graphState, error) {
		response, callErr := a.dependencies.ChatModel.Generate(nodeCtx, input.Request)
		input.Response = response
		return input, callErr
	})); err != nil {
		return contracts.ChatResponse{}, err
	}
	if err := graph.AddEdge(compose.START, "chat_model"); err != nil {
		return contracts.ChatResponse{}, err
	}
	if err := graph.AddEdge("chat_model", compose.END); err != nil {
		return contracts.ChatResponse{}, err
	}
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return contracts.ChatResponse{}, err
	}
	value, err := runnable.Invoke(ctx, graphState{Request: contracts.ChatRequest{Messages: messages, Tools: toolsJSON}})
	if err != nil {
		return contracts.ChatResponse{}, err
	}
	return value.Response, nil
}

func result(state State, startedAt time.Time) contracts.AgentRunResult {
	return resultValue(state, startedAt)
}
func resultValue(state State, startedAt time.Time) contracts.AgentRunResult {
	return contracts.AgentRunResult{RunID: state.AgentContext.RunID, ExecutionMode: "react", KnowledgeStatus: knowledgeStatus(state), FinalResult: state.FinalResult, Citations: stateCitations(state), Usage: state.Usage, StartedAt: startedAt, EndedAt: time.Now()}
}
func (a *agent) citations() []contracts.Citation { return a.dependencies.CitationCollector.Get() }
func stateCitations(state State) []contracts.Citation {
	return append([]contracts.Citation(nil), state.Citations...)
}
func knowledgeStatus(state State) string {
	if len(state.Messages) > 0 {
		return "available"
	}
	return "insufficient"
}
func observation(value contracts.ToolResult) string {
	if value.Success {
		return value.Text
	}
	return fmt.Sprintf("工具调用失败（错误码：%s）：%s", value.ErrorCode, value.ErrorMessage)
}
func validate(ctx contracts.AgentContext, dependencies Dependencies) error {
	if ctx.RunID == "" || ctx.UserID == "" || ctx.Query == "" || dependencies.ChatModel == nil || dependencies.ToolRegistry == nil || dependencies.ToolExecutor == nil || dependencies.CitationCollector == nil {
		return wrap(contracts.ErrInvalidArgument, ErrDependencyUnavailable)
	}
	return nil
}
func validateCall(call contracts.ToolCall, allowed []string) error {
	if call.ToolName == "" || !contains(allowed, call.ToolName) || !json.Valid(call.Arguments) {
		return wrap(contracts.ErrInvalidArgument, ErrInvalidResponse)
	}
	return nil
}
func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
