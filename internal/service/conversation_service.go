package service

import (
	"context"
	"errors"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/google/uuid"
)

// ConversationService 定义会话业务逻辑接口。
type ConversationService interface {
	// Create 创建新的会话。
	Create(ctx context.Context, userID, kbID, title string) (*entity.Conversation, error)
	// Get 获取会话详情。
	Get(ctx context.Context, userID, conversationID string) (*entity.Conversation, error)
	// ListByKnowledgeBase 列出知识库的会话列表。
	ListByKnowledgeBase(ctx context.Context, userID, kbID string, page, pageSize int) ([]entity.Conversation, int64, error)
	// Update 更新会话标题。
	Update(ctx context.Context, userID, conversationID, title string) error
	// Delete 删除会话。
	Delete(ctx context.Context, userID, conversationID string) error
}

type conversationService struct {
	conversations repository.ConversationRepository
}

// NewConversationService 创建一个新的会话服务实例。
func NewConversationService(conversations repository.ConversationRepository) ConversationService {
	return &conversationService{conversations: conversations}
}

// Create 创建新的会话。
func (s *conversationService) Create(ctx context.Context, userID, kbID, title string) (*entity.Conversation, error) {
	if title == "" {
		title = "新会话"
	}

	conversation := &entity.Conversation{
		ID:              uuid.New().String(),
		UserID:          userID,
		KnowledgeBaseID: kbID,
		Title:           title,
		Status:          "active",
	}

	if err := s.conversations.Create(ctx, conversation); err != nil {
		return nil, err
	}

	return conversation, nil
}

// Get 获取会话详情。
func (s *conversationService) Get(ctx context.Context, userID, conversationID string) (*entity.Conversation, error) {
	conversation, err := s.conversations.FindByID(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return nil, err
		}
		return nil, err
	}
	return conversation, nil
}

// ListByKnowledgeBase 列出知识库的会话列表。
func (s *conversationService) ListByKnowledgeBase(ctx context.Context, userID, kbID string, page, pageSize int) ([]entity.Conversation, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.conversations.ListByKnowledgeBase(ctx, userID, kbID, page, pageSize)
}

// Update 更新会话标题。
func (s *conversationService) Update(ctx context.Context, userID, conversationID, title string) error {
	conversation, err := s.conversations.FindByID(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	conversation.Title = title
	return s.conversations.Update(ctx, conversation)
}

// Delete 删除会话。
func (s *conversationService) Delete(ctx context.Context, userID, conversationID string) error {
	return s.conversations.Delete(ctx, conversationID, userID)
}
