package service

import (
	"context"
	"errors"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/google/uuid"
)

// ConversationService 定义会话业务逻辑接口。
type ConversationService interface {
	// Create 创建新的会话。
	Create(ctx context.Context, userID, kbID, title, chatModelID string) (*entity.Conversation, error)
	// Get 获取会话详情。
	Get(ctx context.Context, userID, conversationID string) (*entity.Conversation, error)
	// ListByKnowledgeBase 列出知识库的会话列表。
	ListByKnowledgeBase(ctx context.Context, userID, kbID string, page, pageSize int) ([]entity.Conversation, int64, error)
	// Update 更新会话标题。
	Update(ctx context.Context, userID, conversationID, title string) error
	// UpdateChatModel 修改后续 Run 默认选择的 Chat 模型。
	UpdateChatModel(ctx context.Context, userID, conversationID, chatModelID string) (*entity.Conversation, error)
	// Delete 删除会话。
	Delete(ctx context.Context, userID, conversationID string) error
}

type conversationService struct {
	conversations  repository.ConversationRepository
	knowledgeBases repository.KnowledgeBaseRepository
	models         repository.AIModelConfigRepository
}

// NewConversationService 创建一个新的会话服务实例。
func NewConversationService(conversations repository.ConversationRepository, knowledgeBases repository.KnowledgeBaseRepository, models repository.AIModelConfigRepository) ConversationService {
	return &conversationService{conversations: conversations, knowledgeBases: knowledgeBases, models: models}
}

// Create 创建新的会话。
func (s *conversationService) Create(ctx context.Context, userID, kbID, title, chatModelID string) (*entity.Conversation, error) {
	if chatModelID == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	if _, err := s.knowledgeBases.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	model, err := s.models.FindByIDForUserAndType(ctx, userID, chatModelID, "chat")
	if err != nil {
		return nil, apperrors.New(contracts.ErrInvalidArgument, err)
	}
	if !chatModelConfigComplete(model) {
		return nil, apperrors.New(contracts.ErrInvalidArgument, errors.New("Chat 模型配置不完整"))
	}
	if title == "" {
		title = "新会话"
	}

	conversation := &entity.Conversation{
		ID:              uuid.New().String(),
		UserID:          userID,
		KnowledgeBaseID: kbID,
		ChatModelID:     chatModelID,
		Title:           title,
		Status:          "active",
	}

	if err := s.conversations.Create(ctx, conversation); err != nil {
		return nil, err
	}

	return conversation, nil
}

// UpdateChatModel 修改 Conversation 后续 Run 默认选择的 Chat 模型。
func (s *conversationService) UpdateChatModel(ctx context.Context, userID, conversationID, chatModelID string) (*entity.Conversation, error) {
	if chatModelID == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	conversation, err := s.conversations.FindByID(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	model, err := s.models.FindByIDForUserAndType(ctx, userID, chatModelID, "chat")
	if err != nil {
		return nil, apperrors.New(contracts.ErrInvalidArgument, err)
	}
	if !chatModelConfigComplete(model) {
		return nil, apperrors.New(contracts.ErrInvalidArgument, errors.New("Chat 模型配置不完整"))
	}
	conversation.ChatModelID = chatModelID
	if err := s.conversations.Update(ctx, conversation); err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return conversation, nil
}

func chatModelConfigComplete(model *entity.AIModelConfig) bool {
	return model != nil && strings.TrimSpace(model.Name) != "" && strings.TrimSpace(model.Provider) != "" && strings.TrimSpace(model.BaseURL) != ""
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
// 标题最多保留前15个字符（按UTF-8字符计数，支持中文）。
func (s *conversationService) Update(ctx context.Context, userID, conversationID, title string) error {
	conversation, err := s.conversations.FindByID(ctx, conversationID, userID)
	if err != nil {
		return err
	}

	// 截断标题为最多15个字符
	runes := []rune(title)
	if len(runes) > 15 {
		title = string(runes[:15])
	}

	conversation.Title = title
	return s.conversations.Update(ctx, conversation)
}

// Delete 删除会话。
func (s *conversationService) Delete(ctx context.Context, userID, conversationID string) error {
	return s.conversations.Delete(ctx, conversationID, userID)
}
