package service

import (
	"context"
	"errors"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// 知识库创建时的默认值。
const (
	defaultDirectoryName = "默认目录"
	defaultLanguage      = "zh-CN"
)

// 搜索配置与 Agent 配置的上限约束（对齐 DB CHECK 与 API 6.x）。
const (
	maxKeywordTopK        = 200
	maxVectorTopK         = 200
	maxRRFK               = 200
	maxRRFTopK            = 100
	maxRerankerTopK       = 20
	maxRerankerThreshold  = 1.0
	maxReactRounds        = 8
	maxPlanSteps          = 5
	maxReplans            = 1
	maxReviewerRuns       = 1
	maxToolCalls          = 10
	maxDocumentReadTokens = 20000
	maxToolResultBytes    = 8 * 1024 * 1024
	maxRunSeconds         = 3600
	maxMemoryTopK         = 50
)

// knowledgeBaseService 是 KnowledgeBaseService 接口的实现。
type knowledgeBaseService struct {
	kbs           repository.KnowledgeBaseRepository
	dirs          repository.DocumentDirectoryRepository
	searchConfigs repository.SearchConfigRepository
	agentConfigs  repository.AgentConfigRepository
	modelConfigs  repository.ModelConfigRepository
	tx            repository.Transactor
}

// NewKnowledgeBaseService 创建一个新的知识库服务实例。
func NewKnowledgeBaseService(
	kbs repository.KnowledgeBaseRepository,
	dirs repository.DocumentDirectoryRepository,
	searchConfigs repository.SearchConfigRepository,
	agentConfigs repository.AgentConfigRepository,
	modelConfigs repository.ModelConfigRepository,
	tx repository.Transactor,
) KnowledgeBaseService {
	return &knowledgeBaseService{
		kbs: kbs, dirs: dirs, searchConfigs: searchConfigs,
		agentConfigs: agentConfigs, modelConfigs: modelConfigs, tx: tx,
	}
}

// Create 创建知识库，并在同一短事务中原子创建默认目录、搜索配置和 Agent 配置。
// 默认 Agent 配置需要的 Chat 模型按以下优先级获取：
//  1. 请求中的 default_chat_model_id（校验归属与类型）；
//  2. 当前用户 is_default=true 的 Chat 模型；
//  3. 都没有则拒绝创建，不落库任何关联表。
func (s *knowledgeBaseService) Create(ctx context.Context, userID string, req *request.CreateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	if req.DefaultLanguage == "" {
		req.DefaultLanguage = defaultLanguage
	}

	chatModelID, err := s.resolveChatModel(ctx, userID, req.DefaultChatModelID)
	if err != nil {
		return nil, err
	}
	if req.DefaultEmbeddingModelID != nil && *req.DefaultEmbeddingModelID != "" {
		if _, err := s.modelConfigs.FindEnabledByID(ctx, userID, *req.DefaultEmbeddingModelID); err != nil {
			if errors.Is(err, repository.ErrModelConfigNotFound) {
				return nil, apperrors.New(contracts.ErrInvalidArgument, err)
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
	}
	if req.DefaultRerankerModelID != nil && *req.DefaultRerankerModelID != "" {
		if _, err := s.modelConfigs.FindEnabledByID(ctx, userID, *req.DefaultRerankerModelID); err != nil {
			if errors.Is(err, repository.ErrModelConfigNotFound) {
				return nil, apperrors.New(contracts.ErrInvalidArgument, err)
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
	}

	var created *entity.KnowledgeBase
	// 用短事务原子写 4 张表（知识库 + 默认目录 + 搜索配置 + Agent 配置）：
	// 任一步失败整体回滚，避免产生“孤儿”知识库或缺失默认配置的知识库；
	// 事务内不做外部 I/O，持锁时间极短。
	err = s.tx.WithTx(ctx, func(txCtx context.Context) error {
		kb := &entity.KnowledgeBase{
			UserID:          userID,
			Name:            strings.TrimSpace(req.Name),
			Description:     req.Description,
			Icon:            req.Icon,
			DefaultLanguage: req.DefaultLanguage,
			QAEnabled:       boolValue(req.QAEnabled, true),
			AgentEnabled:    boolValue(req.AgentEnabled, true),
			NetworkEnabled:  boolValue(req.NetworkEnabled, false),
		}
		if req.DefaultChatModelID != nil {
			id := *req.DefaultChatModelID
			kb.DefaultChatModelID = &id
		}
		if req.DefaultEmbeddingModelID != nil {
			id := *req.DefaultEmbeddingModelID
			kb.DefaultEmbeddingModelID = &id
		}
		if req.DefaultRerankerModelID != nil {
			id := *req.DefaultRerankerModelID
			kb.DefaultRerankerModelID = &id
		}
		if err := s.kbs.Create(txCtx, kb); err != nil {
			return err
		}

		if err := s.dirs.Create(txCtx, &entity.DocumentDirectory{
			UserID: userID, KnowledgeBaseID: kb.ID, Name: defaultDirectoryName,
			Depth: 1, SortOrder: 0, IsDefault: true,
		}); err != nil {
			return err
		}

		searchConfig := defaultSearchConfigEntity(kb.ID)
		if err := s.searchConfigs.Create(txCtx, searchConfig); err != nil {
			return err
		}

		agentConfig := defaultAgentConfigEntity(userID, kb.ID, chatModelID)
		if err := s.agentConfigs.Create(txCtx, agentConfig); err != nil {
			return err
		}

		// 四张表全部成功后，把新知识库实体暴露给事务外层用于构造响应。
		created = kb
		return nil
	})
	if errors.Is(err, repository.ErrKnowledgeBaseConflict) {
		return nil, apperrors.ErrConflict
	}
	if errors.Is(err, repository.ErrDirectoryConflict) || errors.Is(err, repository.ErrSearchConfigConflict) || errors.Is(err, repository.ErrAgentConfigConflict) {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}

	logger.Info("知识库已创建", zap.String("user_id", userID), zap.String("kb_id", created.ID))
	return knowledgeBaseResponse(created), nil
}

// resolveChatModel 按优先级确定默认 Agent 配置的 Chat 模型。
func (s *knowledgeBaseService) resolveChatModel(ctx context.Context, userID string, requested *string) (string, error) {
	if requested != nil && *requested != "" {
		model, err := s.modelConfigs.FindChatByID(ctx, userID, *requested)
		if errors.Is(err, repository.ErrModelConfigNotFound) {
			return "", apperrors.New(contracts.ErrInvalidArgument, err)
		}
		if err != nil {
			return "", apperrors.New(contracts.ErrInternal, err)
		}
		return model.ID, nil
	}
	model, err := s.modelConfigs.FindDefaultChat(ctx, userID)
	if errors.Is(err, repository.ErrModelConfigNotFound) {
		return "", &apperrors.AppError{
			Code: contracts.ErrInvalidArgument,
			Details: map[string]string{
				"reason": "当前用户未配置默认 Chat 模型，无法为知识库创建默认 Agent 配置。请先配置一个启用的 Chat 模型并设为默认。",
			},
			Cause: errors.New("no default chat model configured"),
		}
	}
	if err != nil {
		return "", apperrors.New(contracts.ErrInternal, err)
	}
	return model.ID, nil
}

// List 分页查询用户知识库列表。
func (s *knowledgeBaseService) List(ctx context.Context, userID string, page, pageSize int, keyword string) (*dto.KnowledgeBaseList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := s.kbs.List(ctx, userID, page, pageSize, keyword)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	result := &dto.KnowledgeBaseList{Page: page, PageSize: pageSize, Total: total, Items: make([]*dto.KnowledgeBaseListItem, 0, len(items))}
	for _, kb := range items {
		item := &dto.KnowledgeBaseListItem{
			ID: kb.ID, Name: kb.Name, Icon: kb.Icon, Description: kb.Description,
			AgentEnabled: kb.AgentEnabled, NetworkEnabled: kb.NetworkEnabled,
			UpdatedAt: kb.UpdatedAt, CreatedAt: kb.CreatedAt,
		}
		// 逐个补充文档计数：分页量级小，直查更简单；计数失败静默降级为 0，不影响列表返回。
		if count, countErr := s.kbs.CountDocuments(ctx, userID, kb.ID); countErr == nil {
			item.DocumentCount = count
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// Get 查询知识库详情。
func (s *knowledgeBaseService) Get(ctx context.Context, userID, kbID string) (*dto.KnowledgeBaseResponse, error) {
	kb, err := s.kbs.FindByID(ctx, userID, kbID)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return knowledgeBaseResponse(kb), nil
}

// Update 修改知识库基础信息。
func (s *knowledgeBaseService) Update(ctx context.Context, userID, kbID string, req *request.UpdateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error) {
	if req == nil || (req.Name == nil && req.Description == nil && req.Icon == nil &&
		req.DefaultLanguage == nil && req.QAEnabled == nil && req.AgentEnabled == nil &&
		req.NetworkEnabled == nil && req.DefaultChatModelID == nil) {
		return nil, apperrors.ErrInvalidArgument
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.DefaultLanguage != nil && *req.DefaultLanguage != "" {
		updates["default_language"] = *req.DefaultLanguage
	}
	if req.QAEnabled != nil {
		updates["qa_enabled"] = *req.QAEnabled
	}
	if req.AgentEnabled != nil {
		updates["agent_enabled"] = *req.AgentEnabled
	}
	if req.NetworkEnabled != nil {
		updates["network_enabled"] = *req.NetworkEnabled
	}
	if req.DefaultChatModelID != nil && *req.DefaultChatModelID != "" {
		if _, err := s.modelConfigs.FindChatByID(ctx, userID, *req.DefaultChatModelID); err != nil {
			if errors.Is(err, repository.ErrModelConfigNotFound) {
				return nil, apperrors.New(contracts.ErrInvalidArgument, err)
			}
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
		updates["default_chat_model_id"] = *req.DefaultChatModelID
	}
	updated, err := s.kbs.Update(ctx, userID, kbID, updates)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if errors.Is(err, repository.ErrKnowledgeBaseConflict) {
		return nil, apperrors.ErrConflict
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return knowledgeBaseResponse(updated), nil
}

// Delete 软删除知识库。
func (s *knowledgeBaseService) Delete(ctx context.Context, userID, kbID string) error {
	err := s.kbs.SoftDelete(ctx, userID, kbID)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.New(contracts.ErrInternal, err)
	}
	logger.Info("知识库已删除", zap.String("user_id", userID), zap.String("kb_id", kbID))
	return nil
}

// GetSearchConfig 查询知识库搜索配置。
func (s *knowledgeBaseService) GetSearchConfig(ctx context.Context, userID, kbID string) (*dto.SearchConfigResponse, error) {
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	cfg, err := s.searchConfigs.FindByKnowledgeBase(ctx, kbID)
	if errors.Is(err, repository.ErrSearchConfigNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return searchConfigResponse(cfg), nil
}

// UpdateSearchConfig 更新知识库搜索配置并做范围校验。
func (s *knowledgeBaseService) UpdateSearchConfig(ctx context.Context, userID, kbID string, req *request.UpdateSearchConfigRequest) (*dto.SearchConfigResponse, error) {
	if req == nil {
		return nil, apperrors.ErrInvalidArgument
	}
	// 先校验归属与配置存在性，再逐字段做上限/下限校验，避免读到不存在的配置。
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	cfg, err := s.searchConfigs.FindByKnowledgeBase(ctx, kbID)
	if errors.Is(err, repository.ErrSearchConfigNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	if req.KeywordTopK != nil {
		if *req.KeywordTopK > maxKeywordTopK {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.KeywordTopK = *req.KeywordTopK
	}
	if req.VectorTopK != nil {
		if *req.VectorTopK > maxVectorTopK {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.VectorTopK = *req.VectorTopK
	}
	if req.RRFK != nil {
		if *req.RRFK > maxRRFK {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.RRFK = *req.RRFK
	}
	if req.RRFTopK != nil {
		if *req.RRFTopK > maxRRFTopK {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.RRFTopK = *req.RRFTopK
	}
	if req.RerankerTopK != nil {
		if *req.RerankerTopK > maxRerankerTopK {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.RerankerTopK = *req.RerankerTopK
	}
	if req.RerankerThreshold != nil {
		if *req.RerankerThreshold < 0 || *req.RerankerThreshold > maxRerankerThreshold {
			return nil, apperrors.ErrInvalidArgument
		}
		cfg.RerankerThreshold = req.RerankerThreshold
	}
	if req.MinimumEffectiveResult != nil {
		cfg.MinimumEffectiveRate = *req.MinimumEffectiveResult
	}
	if req.RerankerModelID != nil && *req.RerankerModelID != "" {
		if _, err := s.modelConfigs.FindEnabledByID(ctx, userID, *req.RerankerModelID); err != nil {
			return nil, apperrors.New(contracts.ErrInvalidArgument, err)
		}
		cfg.RerankerModelID = req.RerankerModelID
	}
	updated, err := s.searchConfigs.Update(ctx, cfg)
	if err != nil {
		if errors.Is(err, repository.ErrSearchConfigNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return searchConfigResponse(updated), nil
}

// defaultSearchConfigEntity 从 contracts 默认配置构造搜索配置实体（值与 API 6.x 对齐）。
func defaultSearchConfigEntity(kbID string) *entity.SearchConfig {
	config := contracts.DefaultSearchConfig()
	return &entity.SearchConfig{
		KnowledgeBaseID:      kbID,
		KeywordTopK:          config.KeywordTopK,
		VectorTopK:           config.VectorTopK,
		RRFK:                 config.RRFK,
		RRFTopK:              config.RRFTopK,
		RerankerTopK:         config.RerankerTopK,
		RerankerThreshold:    config.RerankerThreshold,
		MinimumEffectiveRate: config.MinimumEffectiveResult,
	}
}

// defaultAgentConfigEntity 从 contracts 默认配置构造 Agent 配置实体，网络功能默认关闭。
func defaultAgentConfigEntity(userID, kbID, chatModelID string) *entity.AgentConfig {
	config := contracts.DefaultAgentConfig()
	networkEnabled := false
	agentConfig := &entity.AgentConfig{
		UserID: userID, KnowledgeBaseID: kbID, Name: "Default Agent",
		ChatModelID:    chatModelID,
		MaxReactRounds: config.MaxReactRounds, MaxPlanSteps: config.MaxPlanSteps,
		MaxReplans: config.MaxReplans, ReviewerRuns: config.ReviewerRuns,
		MaxToolCalls: config.MaxToolCalls, MaxDocumentReadTokens: config.MaxDocumentReadTokens,
		MaxToolResultBytes: config.MaxToolResultBytes, MaxRunSeconds: config.MaxRunSeconds,
		NetworkEnabled: networkEnabled, MemoryEnabled: true, MemoryTopK: config.MemoryTopK,
		ShowExecutionStatus: true, Status: "active",
	}
	return agentConfig
}

// boolValue 安全解引用布尔指针，nil 时返回传入的默认值。
func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

// knowledgeBaseResponse 将知识库实体转换为响应 DTO。
func knowledgeBaseResponse(kb *entity.KnowledgeBase) *dto.KnowledgeBaseResponse {
	return &dto.KnowledgeBaseResponse{
		ID: kb.ID, Name: kb.Name, Description: kb.Description, Icon: kb.Icon,
		DefaultLanguage: kb.DefaultLanguage, QAEnabled: kb.QAEnabled,
		AgentEnabled: kb.AgentEnabled, NetworkEnabled: kb.NetworkEnabled,
		DefaultChatModelID: kb.DefaultChatModelID, DefaultEmbeddingModelID: kb.DefaultEmbeddingModelID,
		DefaultRerankerModelID: kb.DefaultRerankerModelID,
		CreatedAt:              kb.CreatedAt, UpdatedAt: kb.UpdatedAt,
	}
}

// searchConfigResponse 将搜索配置实体转换为响应 DTO。
func searchConfigResponse(cfg *entity.SearchConfig) *dto.SearchConfigResponse {
	return &dto.SearchConfigResponse{
		KeywordTopK: cfg.KeywordTopK, VectorTopK: cfg.VectorTopK, RRFK: cfg.RRFK,
		RRFTopK: cfg.RRFTopK, RerankerTopK: cfg.RerankerTopK, RerankerThreshold: cfg.RerankerThreshold,
		MinimumEffectiveResult: cfg.MinimumEffectiveRate, RerankerModelID: cfg.RerankerModelID,
	}
}
