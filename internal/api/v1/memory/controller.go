package memory

import (
	"net/http"
	"strconv"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/gin-gonic/gin"
)

// Controller 处理长期记忆管理相关的 HTTP 请求。
type Controller struct {
	memoryRepo repository.MemoryRepository
}

// NewController 创建一个新的记忆控制器实例。
func NewController(memoryRepo repository.MemoryRepository) *Controller {
	return &Controller{memoryRepo: memoryRepo}
}

// List 查询用户的记忆分页列表。
func (ctrl *Controller) List(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	opts := repository.ListMemoryOpts{
		MemoryType: c.Query("memory_type"),
		ScopeType:  c.Query("scope_type"),
		Status:     c.Query("status"),
		Page:       page,
		PageSize:   pageSize,
	}

	// 处理 scope_id 过滤
	if scopeID := c.Query("scope_id"); scopeID != "" {
		opts.ScopeID = &scopeID
	}

	result, err := ctrl.memoryRepo.ListByUser(c.Request.Context(), user.ID, opts)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"items": result.Items,
		"total": result.Total,
		"page":  page,
		"size":  pageSize,
	})
}

// Get 查询单个记忆详情。
func (ctrl *Controller) Get(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	memoryID := c.Param("memory_id")
	if memoryID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	memory, err := ctrl.memoryRepo.FindByID(c.Request.Context(), memoryID, user.ID)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, memory)
}

// Delete 软删除记忆。
func (ctrl *Controller) Delete(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	memoryID := c.Param("memory_id")
	if memoryID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	err := ctrl.memoryRepo.Delete(c.Request.Context(), memoryID, user.ID)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// UpdateStatus 更新记忆状态（active/inactive）。
func (ctrl *Controller) UpdateStatus(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	memoryID := c.Param("memory_id")
	if memoryID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=active inactive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	err := ctrl.memoryRepo.UpdateStatus(c.Request.Context(), memoryID, user.ID, req.Status)
	if err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"status": req.Status})
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
