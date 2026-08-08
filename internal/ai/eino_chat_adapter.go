package ai

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// einoChatModelAdapter 将 Eino ToolCallingChatModel 适配为 contracts.ChatModel。
type einoChatModelAdapter struct {
	model model.ToolCallingChatModel
}

// Generate 实现 contracts.ChatModel.Generate。
func (a *einoChatModelAdapter) Generate(ctx context.Context, request contracts.ChatRequest) (contracts.ChatResponse, error) {
	// 转换消息格式
	messages := make([]*schema.Message, len(request.Messages))
	for i, msg := range request.Messages {
		messages[i] = &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: msg.Content,
		}
	}

	// 调用 Eino ChatModel
	resp, err := a.model.Generate(ctx, messages)
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
	// 转换消息格式
	messages := make([]*schema.Message, len(request.Messages))
	for i, msg := range request.Messages {
		messages[i] = &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: msg.Content,
		}
	}

	// 调用 Eino Stream
	stream, err := a.model.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("eino stream: %w", err)
	}

	// 转换为 contracts.ChatStreamEvent 通道
	eventChan := make(chan contracts.ChatStreamEvent, 100)
	go func() {
		defer close(eventChan)
		for {
			event, err := stream.Recv()
			if err != nil {
				// 流结束或出错
				eventChan <- contracts.ChatStreamEvent{
					Done:  true,
					Usage: &contracts.TokenUsage{},
				}
				return
			}
			eventChan <- contracts.ChatStreamEvent{
				Delta: event.Content,
				Done:  false,
			}
		}
	}()

	return eventChan, nil
}
