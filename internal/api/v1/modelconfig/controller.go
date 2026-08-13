package modelconfig

import (
	"net/http"

	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Controller 处理模型配置相关的 HTTP 请求。
type Controller struct {
	repo   repository.AIModelConfigRepository
	crypto encryption.Service
}

// NewController 创建一个新的 Controller 实例。
func NewController(repo repository.AIModelConfigRepository, crypto encryption.Service) *Controller {
	return &Controller{repo: repo, crypto: crypto}
}

// ListModelConfigs 返回当前用户的所有模型配置。
func (ctrl *Controller) ListModelConfigs(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	modelType := c.Query("model_type")

	configs, err := ctrl.repo.ListByUser(c.Request.Context(), user.ID, modelType)
	if err != nil {
		response.Failure(c, err)
		return
	}

	// 脱敏处理 - 使用已有的 APIKeyMasked 字段
	type maskedConfig struct {
		ID              string   `json:"id"`
		Name            string   `json:"name"`
		Provider        string   `json:"provider"`
		ModelType       string   `json:"model_type"`
		BaseURL         string   `json:"base_url"`
		APIKeyMasked    string   `json:"api_key_masked"`
		IsDefault       bool     `json:"is_default"`
		MaxTokens       *int     `json:"max_tokens,omitempty"`
		Temperature     *float64 `json:"temperature,omitempty"`
		VectorDimension *int     `json:"vector_dimension,omitempty"`
	}

	masked := make([]maskedConfig, len(configs))
	for i, cfg := range configs {
		masked[i] = maskedConfig{
			ID:              cfg.ID,
			Name:            cfg.Name,
			Provider:        cfg.Provider,
			ModelType:       cfg.ModelType,
			BaseURL:         cfg.BaseURL,
			APIKeyMasked:    cfg.APIKeyMasked,
			IsDefault:       cfg.IsDefault,
			MaxTokens:       cfg.MaxTokens,
			Temperature:     cfg.Temperature,
			VectorDimension: cfg.VectorDimension,
		}
	}

	response.Success(c, http.StatusOK, gin.H{
		"items": masked,
		"total": len(masked),
	})
}

// CreateModelConfig 创建新的模型配置。
func (ctrl *Controller) CreateModelConfig(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	var input struct {
		Name                string   `json:"name" binding:"required"`
		Provider            string   `json:"provider" binding:"required"`
		ModelType           string   `json:"model_type" binding:"required,oneof=chat embedding reranker"`
		BaseURL             string   `json:"base_url" binding:"required"`
		APIKey              string   `json:"api_key" binding:"required"`
		TimeoutSeconds      int      `json:"timeout_seconds"`
		RetryTimes          int      `json:"retry_times"`
		IsDefault           bool     `json:"is_default"`
		MaxTokens           *int     `json:"max_tokens,omitempty"`
		Temperature         *float64 `json:"temperature,omitempty"`
		VectorDimension     *int     `json:"vector_dimension,omitempty"`
		SupportsToolCalling bool     `json:"supports_tool_calling"`
		SupportsStreaming   bool     `json:"supports_streaming"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.Failure(c, err)
		return
	}

	// 生成脱敏后的 API Key
	apiKeyMasked := ""
	if len(input.APIKey) > 8 {
		apiKeyMasked = input.APIKey[:4] + "****" + input.APIKey[len(input.APIKey)-4:]
	} else if len(input.APIKey) > 0 {
		apiKeyMasked = "****"
	}
	if ctrl.crypto == nil {
		response.Failure(c, apperrors.New(contracts.ErrServiceUnavailable, nil))
		return
	}
	ciphertext, err := ctrl.crypto.Encrypt(input.APIKey)
	if err != nil {
		response.Failure(c, apperrors.New(contracts.ErrInternal, err))
		return
	}

	// 设置默认值
	timeoutSeconds := input.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	retryTimes := input.RetryTimes
	if retryTimes < 0 {
		retryTimes = 2
	}

	config := &entity.AIModelConfig{
		ID:                  uuid.New().String(),
		Name:                input.Name,
		UserID:              user.ID,
		Provider:            input.Provider,
		ModelType:           input.ModelType,
		BaseURL:             input.BaseURL,
		APIKeyMasked:        apiKeyMasked,
		APIKeyCiphertext:    ciphertext,
		TimeoutSeconds:      timeoutSeconds,
		RetryTimes:          retryTimes,
		IsDefault:           input.IsDefault,
		MaxTokens:           input.MaxTokens,
		Temperature:         input.Temperature,
		VectorDimension:     input.VectorDimension,
		SupportsToolCalling: input.SupportsToolCalling,
		SupportsStreaming:   input.SupportsStreaming,
		Enabled:             true,
	}

	if err := ctrl.repo.Create(c.Request.Context(), config); err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusCreated, config)
}

// UpdateModelConfig 更新模型配置。
func (ctrl *Controller) UpdateModelConfig(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	configID := c.Param("id")
	if configID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	// 先查询现有配置
	existing, err := ctrl.repo.FindByID(c.Request.Context(), configID)
	if err != nil {
		response.Failure(c, err)
		return
	}
	if existing.UserID != user.ID {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	var input struct {
		Name                *string  `json:"name"`
		Provider            *string  `json:"provider"`
		ModelType           *string  `json:"model_type" binding:"omitempty,oneof=chat embedding reranker"`
		BaseURL             *string  `json:"base_url"`
		APIKey              *string  `json:"api_key"`
		TimeoutSeconds      *int     `json:"timeout_seconds"`
		RetryTimes          *int     `json:"retry_times"`
		IsDefault           *bool    `json:"is_default"`
		Enabled             *bool    `json:"enabled"`
		MaxTokens           *int     `json:"max_tokens,omitempty"`
		Temperature         *float64 `json:"temperature,omitempty"`
		VectorDimension     *int     `json:"vector_dimension,omitempty"`
		SupportsToolCalling *bool    `json:"supports_tool_calling"`
		SupportsStreaming   *bool    `json:"supports_streaming"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.Failure(c, err)
		return
	}

	// 更新字段
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Provider != nil {
		existing.Provider = *input.Provider
	}
	if input.ModelType != nil {
		existing.ModelType = *input.ModelType
	}
	if input.BaseURL != nil {
		existing.BaseURL = *input.BaseURL
	}
	if input.APIKey != nil && *input.APIKey != "" {
		// 重新加密 API Key
		if ctrl.crypto != nil {
			ciphertext, err := ctrl.crypto.Encrypt(*input.APIKey)
			if err != nil {
				response.Failure(c, apperrors.New(contracts.ErrInternal, err))
				return
			}
			existing.APIKeyCiphertext = ciphertext
			// 更新脱敏显示
			if len(*input.APIKey) > 8 {
				existing.APIKeyMasked = (*input.APIKey)[:4] + "****" + (*input.APIKey)[len(*input.APIKey)-4:]
			} else {
				existing.APIKeyMasked = "****"
			}
		}
	}
	if input.TimeoutSeconds != nil {
		existing.TimeoutSeconds = *input.TimeoutSeconds
	}
	if input.RetryTimes != nil {
		existing.RetryTimes = *input.RetryTimes
	}
	if input.IsDefault != nil {
		existing.IsDefault = *input.IsDefault
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.MaxTokens != nil {
		existing.MaxTokens = input.MaxTokens
	}
	if input.Temperature != nil {
		existing.Temperature = input.Temperature
	}
	if input.VectorDimension != nil {
		existing.VectorDimension = input.VectorDimension
	}
	if input.SupportsToolCalling != nil {
		existing.SupportsToolCalling = *input.SupportsToolCalling
	}
	if input.SupportsStreaming != nil {
		existing.SupportsStreaming = *input.SupportsStreaming
	}

	if err := ctrl.repo.Update(c.Request.Context(), existing); err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, existing)
}

// DeleteModelConfig 删除模型配置。
func (ctrl *Controller) DeleteModelConfig(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	configID := c.Param("id")
	if configID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	if err := ctrl.repo.Delete(c.Request.Context(), configID, user.ID); err != nil {
		response.Failure(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}
