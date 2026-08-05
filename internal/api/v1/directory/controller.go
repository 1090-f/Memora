package directory

import (
	"net/http"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/service"
	"github.com/gin-gonic/gin"
)

// Controller 处理文档目录管理相关的 HTTP 请求。
type Controller struct{ dirs service.DirectoryService }

// NewController 创建一个新的文档目录控制器实例。
func NewController(dirs service.DirectoryService) *Controller { return &Controller{dirs: dirs} }

// Tree 查询知识库目录树。
func (ctrl *Controller) Tree(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	result, err := ctrl.dirs.GetTree(c.Request.Context(), user.ID, c.Param("kb_id"))
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

// Create 创建目录。
func (ctrl *Controller) Create(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var req request.CreateDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	result, err := ctrl.dirs.Create(c.Request.Context(), user.ID, c.Param("kb_id"), &req)
	if err != nil {
		response.Failure(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}
