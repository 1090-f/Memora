package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

type retrievalMock struct {
	last contracts.RetrievalRequest
	res  contracts.RetrievalResult
	err  error
}

func (m *retrievalMock) Retrieve(ctx context.Context, request contracts.RetrievalRequest) (contracts.RetrievalResult, error) {
	m.last = request
	return m.res, m.err
}

type documentMock struct {
	last contracts.DocumentReadRequest
	res  contracts.DocumentReadResult
	err  error
}

func (m *documentMock) Read(ctx context.Context, request contracts.DocumentReadRequest) (contracts.DocumentReadResult, error) {
	m.last = request
	return m.res, m.err
}

type dummyTool struct {
	spec contracts.ToolSpec
	run  func(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error)
}

func (t *dummyTool) Spec() contracts.ToolSpec { return t.spec }
func (t *dummyTool) Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
	return t.run(ctx, toolContext, arguments)
}

func TestExecutorRejectsNotAllowedTool(t *testing.T) {
	reg := NewRegistry()
	tool := &dummyTool{
		spec: contracts.ToolSpec{Name: "x", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true},
		run: func(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
			return contracts.ToolResult{Success: true}, nil
		},
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := NewExecutor(reg)

	res, err := exec.Execute(context.Background(), contracts.ToolContext{
		UserID:          "u1",
		KnowledgeBaseID: "kb1",
		AgentRunID:      "run1",
		AllowedToolNames: []string{
			"y",
		},
		MaxResultBytes: 1024,
	}, contracts.ToolCall{CallID: "c1", ToolName: "x", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Success {
		t.Fatalf("expected failure")
	}
	if res.ErrorCode != contracts.ErrForbidden {
		t.Fatalf("expected forbidden, got %s", res.ErrorCode)
	}
}

func TestExecutorRejectsNetworkToolWhenDisabled(t *testing.T) {
	reg := NewRegistry()
	tool := &dummyTool{
		spec: contracts.ToolSpec{Name: "net", Type: contracts.ToolTypeMCP, ReadOnly: true, Enabled: true, NetworkRequired: true},
		run: func(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
			return contracts.ToolResult{Success: true}, nil
		},
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := NewExecutor(reg)

	res, err := exec.Execute(context.Background(), contracts.ToolContext{
		UserID:           "u1",
		KnowledgeBaseID:  "kb1",
		AgentRunID:       "run1",
		AllowedToolNames: []string{"net"},
		NetworkEnabled:   false,
		MaxResultBytes:   1024,
	}, contracts.ToolCall{CallID: "c1", ToolName: "net", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.ErrorCode != contracts.ErrNetworkDisabled {
		t.Fatalf("expected network disabled, got %s", res.ErrorCode)
	}
}

func TestKnowledgeSearchToolInjectsIdentity(t *testing.T) {
	m := &retrievalMock{res: contracts.RetrievalResult{Items: nil, KnowledgeStatus: "ok"}}
	tool := NewKnowledgeSearchTool(m)
	reg := NewRegistry()
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := NewExecutor(reg)

	args := json.RawMessage(`{"query":"q","mode":"keyword","top_k":3}`)
	res, err := exec.Execute(context.Background(), contracts.ToolContext{
		UserID:           "u1",
		KnowledgeBaseID:  "kb1",
		AgentRunID:       "run1",
		AllowedToolNames: []string{KnowledgeSearchToolName},
		NetworkEnabled:   false,
		MaxResultBytes:   1024 * 1024,
	}, contracts.ToolCall{CallID: "c1", ToolName: KnowledgeSearchToolName, Arguments: args})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %s %s", res.ErrorCode, res.ErrorMessage)
	}
	if m.last.UserID != "u1" || m.last.KnowledgeBaseID != "kb1" || m.last.Query != "q" || m.last.TopK != 3 {
		t.Fatalf("unexpected request: %+v", m.last)
	}
}

func TestExecutorEnforcesTimeout(t *testing.T) {
	reg := NewRegistry()
	tool := &dummyTool{
		spec: contracts.ToolSpec{Name: "slow", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true, Timeout: 10 * time.Millisecond},
		run: func(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
			<-ctx.Done()
			return contracts.ToolResult{Success: false}, ctx.Err()
		},
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := NewExecutor(reg)

	res, err := exec.Execute(context.Background(), contracts.ToolContext{
		UserID:           "u1",
		KnowledgeBaseID:  "kb1",
		AgentRunID:       "run1",
		AllowedToolNames: []string{"slow"},
		MaxResultBytes:   1024,
	}, contracts.ToolCall{CallID: "c1", ToolName: "slow", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.ErrorCode != contracts.ErrUpstreamTimeout {
		t.Fatalf("expected timeout, got %s", res.ErrorCode)
	}
}

func TestExecutorEnforcesResultSize(t *testing.T) {
	reg := NewRegistry()
	tool := &dummyTool{
		spec: contracts.ToolSpec{Name: "big", Type: contracts.ToolTypeBuiltin, ReadOnly: true, Enabled: true},
		run: func(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error) {
			return contracts.ToolResult{
				Text:      strings.Repeat("a", 5000),
				Citations: []contracts.Citation{{SourceType: contracts.CitationKnowledge}},
				Success:   true,
			}, nil
		},
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := NewExecutor(reg)

	res, err := exec.Execute(context.Background(), contracts.ToolContext{
		UserID:           "u1",
		KnowledgeBaseID:  "kb1",
		AgentRunID:       "run1",
		AllowedToolNames: []string{"big"},
		MaxResultBytes:   200,
	}, contracts.ToolCall{CallID: "c1", ToolName: "big", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated")
	}
}
