package contracts

import (
	"context"
	"time"
)

type ConversationMessage struct {
	ID        ID        `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ConversationContext struct {
	ConversationID ID                    `json:"conversation_id"`
	Messages       []ConversationMessage `json:"messages"`
	Summary        string                `json:"summary,omitempty"`
	TokenCount     int                   `json:"token_count"`
}

type ConversationContextService interface {
	Build(ctx context.Context, userID, knowledgeBaseID, conversationID ID) (ConversationContext, error)
}
