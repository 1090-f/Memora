package conversation

import (
	"net/http"
	"strconv"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/gin-gonic/gin"
)

// Controller 处理会话管理相关的 HTTP 请求。
type Controller struct {
	conversations service.ConversationService
	messages      repository.MessageRepository
}

// NewController 创建一个新的会话控制器实例。
func NewController(conversations service.ConversationService, messages repository.MessageRepository) *Controller {
	return &Controller{conversations: conversations, messages: messages}
}

// Create 创建会话。
func (ctrl *Controller) Create(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	kbID := c.Param("kb_id")
	if kbID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req.Title = ""
	}

	conversation, err := ctrl.conversations.Create(c.Request.Context(), user.ID, kbID, req.Title)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusCreated, conversation)
}

// Get 获取会话详情。
func (ctrl *Controller) Get(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	conversation, err := ctrl.conversations.Get(c.Request.Context(), user.ID, conversationID)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, conversation)
}

// List 列出会话列表。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	kbID := c.Query("knowledge_base_id")
	if kbID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	conversations, total, err := ctrl.conversations.ListByKnowledgeBase(c.Request.Context(), user.ID, kbID, page, pageSize)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"items": conversations,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// Update 更新会话标题。
func (ctrl *Controller) Update(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	if req.Title == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	if err := ctrl.conversations.Update(c.Request.Context(), user.ID, conversationID, req.Title); err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// Delete 删除会话。
func (ctrl *Controller) Delete(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	if err := ctrl.conversations.Delete(c.Request.Context(), user.ID, conversationID); err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// ListMessages 列出会话的消息列表。
func (ctrl *Controller) ListMessages(c *gin.Context) {
	if _, ok := middleware.GetUser(c); !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	offset := (page - 1) * pageSize
	messages, err := ctrl.messages.ListByConversation(c.Request.Context(), conversationID, pageSize, offset)
	if err != nil {
		response.Failure(c, err)
		return
	}

	total, err := ctrl.messages.CountByConversation(c.Request.Context(), conversationID)
	if err != nil {
		response.Failure(c, err)
		return
	}

	// 如果消息为空，返回空数组而不是 null
	if messages == nil {
		messages = []entity.Message{}
	}

	response.Success(c, http.StatusOK, gin.H{
		"items": messages,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// parseIntDefault 解析查询参数为整数，缺失或非法时返回默认值。
func parseIntDefault(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
