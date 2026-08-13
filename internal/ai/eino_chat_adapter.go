package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	"go.uber.org/zap"
)

// einoChatModelAdapter 将 Eino ToolCallingChatModel 适配为 contracts.ChatModel。
type einoChatModelAdapter struct {
	model model.ToolCallingChatModel
}

// Generate 实现 contracts.ChatModel.Generate。
func (a *einoChatModelAdapter) Generate(ctx context.Context, request contracts.ChatRequest) (contracts.ChatResponse, error) {
	// 转换消息格式（包含 ToolCallID 和 ToolCalls 映射）
	messages := make([]*schema.Message, len(request.Messages))
	for i, msg := range request.Messages {
		einoMsg := &schema.Message{
			Role:       schema.RoleType(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			einoMsg.ToolCalls = make([]schema.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				einoMsg.ToolCalls[j] = schema.ToolCall{
					ID:   string(tc.CallID),
					Type: "function",
					Function: schema.FunctionCall{
						Name:      tc.ToolName,
						Arguments: string(tc.Arguments),
					},
				}
			}
		}
		messages[i] = einoMsg
	}

	// 构建选项（携带工具定义，使 LLM 知道可调用哪些工具）
	var opts []model.Option
	if len(request.Tools) > 0 {
		toolInfos, err := parseToolInfos(request.Tools)
		if err != nil {
			return contracts.ChatResponse{}, fmt.Errorf("解析工具定义: %w", err)
		}
		opts = append(opts, model.WithTools(toolInfos))
	}

	// 调用 Eino ChatModel（传入工具选项）
	resp, err := a.model.Generate(ctx, messages, opts...)
	if err != nil {
		return contracts.ChatResponse{}, fmt.Errorf("eino generate: %w", err)
	}

	// 转换响应格式
	usage := contracts.TokenUsage{}
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage = contracts.TokenUsage{
			InputTokens:  resp.ResponseMeta.Usage.PromptTokens,
			OutputTokens: resp.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:  resp.ResponseMeta.Usage.TotalTokens,
		}
	}

	return contracts.ChatResponse{
		Content: resp.Content,
		Usage:   usage,
	}, nil
}

// Stream 实现 contracts.ChatModel.Stream。
func (a *einoChatModelAdapter) Stream(ctx context.Context, request contracts.ChatRequest) (<-chan contracts.ChatStreamEvent, error) {
	// 转换消息格式（包含 ToolCallID 和 ToolCalls 映射）
	messages := make([]*schema.Message, len(request.Messages))
	for i, msg := range request.Messages {
		einoMsg := &schema.Message{
			Role:       schema.RoleType(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			einoMsg.ToolCalls = make([]schema.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				einoMsg.ToolCalls[j] = schema.ToolCall{
					ID:   string(tc.CallID),
					Type: "function",
					Function: schema.FunctionCall{
						Name:      tc.ToolName,
						Arguments: string(tc.Arguments),
					},
				}
			}
		}
		messages[i] = einoMsg
	}

	// 构建选项（携带工具定义，使 LLM 知道可调用哪些工具）
	var opts []model.Option
	if len(request.Tools) > 0 {
		toolInfos, err := parseToolInfos(request.Tools)
		if err != nil {
			return nil, fmt.Errorf("解析工具定义: %w", err)
		}
		logger.Debug("einoChatModelAdapter.Stream: 传递工具定义给模型",
			zap.Int("tool_count", len(toolInfos)),
		)
		opts = append(opts, model.WithTools(toolInfos))
	} else {
		logger.Debug("einoChatModelAdapter.Stream: 没有工具定义传递给模型")
	}

	// 调用 Eino Stream（传入工具选项）
	stream, err := a.model.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("eino stream: %w", err)
	}

	// 转换为 contracts.ChatStreamEvent 通道
	eventChan := make(chan contracts.ChatStreamEvent, 100)
	go func() {
		defer close(eventChan)
		defer stream.Close()

		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				// 检查是否为真正的错误（非正常结束）
				if !errors.Is(recvErr, io.EOF) {
					logger.Warn("Eino 模型流异常结束",
						zap.Error(recvErr),
					)
				}
				// 流正常结束或异常终止，发送 Done 事件
				usage := &contracts.TokenUsage{}
				// 尝试从 event 中获取最后一条消息的用法
				if event != nil && event.ResponseMeta != nil && event.ResponseMeta.Usage != nil {
					usage.InputTokens = event.ResponseMeta.Usage.PromptTokens
					usage.OutputTokens = event.ResponseMeta.Usage.CompletionTokens
					usage.TotalTokens = event.ResponseMeta.Usage.TotalTokens
				}
				eventChan <- contracts.ChatStreamEvent{
					Done:  true,
					Usage: usage,
				}
				return
			}

			out := contracts.ChatStreamEvent{Done: false}
			if event.Content != "" {
				out.Delta = event.Content
			}
			if len(event.ToolCalls) > 0 {
				tcs := make([]contracts.ToolCall, len(event.ToolCalls))
				for i, tc := range event.ToolCalls {
					tcs[i] = contracts.ToolCall{
						CallID:    contracts.ID(tc.ID),
						ToolName:  tc.Function.Name,
						Arguments: []byte(tc.Function.Arguments),
					}
				}
				out.ToolCalls = tcs
			}
			eventChan <- out
		}
	}()

	return eventChan, nil
}

// parseToolInfos 将 JSON 格式的 []contracts.ToolSpec 转换为 Eino []*schema.ToolInfo。
// 这是适配层的桥接函数，使 contracts 层的工具定义能被 Eino 模型识别。
func parseToolInfos(toolsJSON json.RawMessage) ([]*schema.ToolInfo, error) {
	if len(toolsJSON) == 0 {
		logger.Debug("parseToolInfos: 工具定义 JSON 为空")
		return nil, nil
	}

	var specs []contracts.ToolSpec
	if err := json.Unmarshal(toolsJSON, &specs); err != nil {
		return nil, fmt.Errorf("反序列化工具规格: %w", err)
	}

	logger.Debug("parseToolInfos: 开始解析工具定义",
		zap.Int("spec_count", len(specs)),
	)

	infos := make([]*schema.ToolInfo, 0, len(specs))
	for _, spec := range specs {
		ti := &schema.ToolInfo{
			Name: spec.Name,
			Desc: spec.Description,
		}
		// 如果工具定义了 InputSchema（JSON Schema），转换为 Eino 的 ParamsOneOf
		if len(spec.InputSchema) > 0 {
			var inputSchema jsonschema.Schema
			if err := json.Unmarshal(spec.InputSchema, &inputSchema); err != nil {
				return nil, fmt.Errorf("工具 %q 的 InputSchema 无效: %w", spec.Name, err)
			}
			ti.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&inputSchema)
		}
		infos = append(infos, ti)
	}

	logger.Debug("parseToolInfos: 工具定义解析完成",
		zap.Int("tool_info_count", len(infos)),
		zap.Strings("tool_names", extractToolInfoNames(infos)),
	)

	return infos, nil
}

// extractToolInfoNames 从 ToolInfo 切片中提取名称列表。
func extractToolInfoNames(infos []*schema.ToolInfo) []string {
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name
	}
	return names
}
