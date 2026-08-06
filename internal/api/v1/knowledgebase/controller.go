package knowledgebase

import (
	"net/http"
	"strconv"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/audit"
	"github.com/gin-gonic/gin"
)

// Controller 处理知识库管理相关的 HTTP 请求。
type Controller struct{ kbs service.KnowledgeBaseService }

// NewController 创建一个新的知识库控制器实例。
func NewController(kbs service.KnowledgeBaseService) *Controller { return &Controller{kbs: kbs} }

// Create 创建知识库。
func (ctrl *Controller) Create(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.kbs.Create(c.Request.Context(), user.ID, &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("knowledge_base.create", user.ID, "knowledge_base:"+result.ID, middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusCreated, result)
}

// List 查询知识库分页列表。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	result, err := ctrl.kbs.List(c.Request.Context(), user.ID, page, pageSize, c.Query("keyword"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Get 查询知识库详情。
func (ctrl *Controller) Get(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.kbs.Get(c.Request.Context(), user.ID, c.Param("kb_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Update 修改知识库基础信息。
func (ctrl *Controller) Update(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.kbs.Update(c.Request.Context(), user.ID, c.Param("kb_id"), &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("knowledge_base.update", user.ID, "knowledge_base:"+result.ID, middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, result)
}

// Delete 软删除知识库。
func (ctrl *Controller) Delete(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	if err := ctrl.kbs.Delete(c.Request.Context(), user.ID, c.Param("kb_id")); err != nil {
		response.Failure(c, err)
		return
	}
	audit.Record("knowledge_base.delete", user.ID, "knowledge_base:"+c.Param("kb_id"), middleware.GetRequestID(c), middleware.GetTraceID(c), "succeeded")
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// GetSearchConfig 查询知识库搜索配置。
func (ctrl *Controller) GetSearchConfig(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.kbs.GetSearchConfig(c.Request.Context(), user.ID, c.Param("kb_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// UpdateSearchConfig 更新知识库搜索配置。
func (ctrl *Controller) UpdateSearchConfig(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.UpdateSearchConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.kbs.UpdateSearchConfig(c.Request.Context(), user.ID, c.Param("kb_id"), &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

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
