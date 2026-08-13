package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/mcp"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/google/uuid"
)

// importService 是 ImportService 的实现。
type importService struct {
	servers        repository.MCPServerRepository
	tools          repository.MCPToolRepository
	cfg            *config.Config
	clientProvider func() mcp.MCPClient
}

// NewImportService 创建 ImportService 实例。
func NewImportService(servers repository.MCPServerRepository, tools repository.MCPToolRepository, cfg *config.Config) ImportService {
	return &importService{
		servers: servers,
		tools:   tools,
		cfg:     cfg,
		clientProvider: func() mcp.MCPClient {
			return mcp.NewMCPClient()
		},
	}
}

// Import 执行 MCP Server 导入流程。使用并发工作池加速导入。
func (s *importService) Import(ctx context.Context, userID string, req *request.MCPImportRequest) (*response.MCPImportResponse, error) {
	if req == nil || len(req.MCPServers) == 0 {
		return nil, repository.ErrInvalidArgument
	}
	if len(req.MCPServers) > 20 {
		return nil, repository.ErrInvalidArgument
	}

	type jobResult struct {
		name   string
		server response.ImportedServer
		err    error
	}

	var (
		mu       sync.Mutex
		imported = make([]response.ImportedServer, 0, len(req.MCPServers))
		failed   = make([]response.FailedServer, 0)
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 3) // 最多 3 个并发
	)

	for name, serverConfig := range req.MCPServers {
		name := strings.TrimSpace(name)
		if name == "" {
			mu.Lock()
			failed = append(failed, response.FailedServer{
				Name:    name,
				Error:   "INVALID_ARGUMENT",
				Message: "server name is empty",
			})
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(n string, cfg request.MCPServerConfig) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			server, err := s.importSingleServer(ctx, userID, n, cfg)

			mu.Lock()
			if err != nil {
				failed = append(failed, response.FailedServer{
					Name:    n,
					Error:   mapErrorToCode(err),
					Message: err.Error(),
				})
			} else {
				imported = append(imported, server)
			}
			mu.Unlock()
		}(name, serverConfig)
	}

	wg.Wait()

	return &response.MCPImportResponse{
		Imported: imported,
		Failed:   failed,
		Summary: response.ImportSummary{
			Total:    len(req.MCPServers),
			Imported: len(imported),
			Failed:   len(failed),
		},
	}, nil
}

func (s *importService) importSingleServer(ctx context.Context, userID string, name string, config request.MCPServerConfig) (response.ImportedServer, error) {
	transport := determineTransport(&config)

	// 查重
	_, err := s.servers.FindActiveByName(ctx, userID, name)
	if err == nil {
		return response.ImportedServer{}, repository.ErrDuplicateResource
	}
	if !errors.Is(err, repository.ErrMCPServerNotFound) {
		return response.ImportedServer{}, err
	}

	var serverEntity *entity.MCPServer
	switch transport {
	case "streamable_http":
		serverEntity, err = s.importHTTPServer(ctx, userID, name, config)
	case "stdio":
		serverEntity, err = s.importStdioServer(ctx, userID, name, config)
	default:
		return response.ImportedServer{}, fmt.Errorf("unsupported transport: %s", transport)
	}
	if err != nil {
		return response.ImportedServer{}, err
	}

	// 连接测试必须通过后才允许写入数据库，避免不可用的 server 出现在 MCP 列表。
	target := s.buildTarget(serverEntity)
	client := s.clientProvider()

	// 打开一个 MCP 会话，连接测试和工具发现共用同一会话，避免 stdio 重复冷启动
	connCtx, connCancel := context.WithTimeout(ctx, time.Duration(serverEntity.ConnectTimeoutMs)*time.Millisecond)
	defer connCancel()
	session, sessionErr := client.OpenSession(connCtx, target, time.Duration(serverEntity.ConnectTimeoutMs)*time.Millisecond)
	if sessionErr != nil {
		return response.ImportedServer{}, fmt.Errorf("%w: open MCP session: %v", repository.ErrMCPConnectionFailed, sessionErr)
	}
	defer session.Close()

	// 连接测试（Initialize 握手）
	if initErr := session.Initialize(connCtx); initErr != nil {
		return response.ImportedServer{}, fmt.Errorf("%w: %v", repository.ErrMCPConnectionFailed, initErr)
	}

	serverEntity.ConnectionStatus = "available"
	serverEntity.LastError = nil
	if err := s.servers.Create(ctx, serverEntity); err != nil {
		if errors.Is(err, repository.ErrDuplicateResource) {
			return response.ImportedServer{}, repository.ErrDuplicateResource
		}
		return response.ImportedServer{}, err
	}
	if err := s.servers.UpdateStatus(ctx, serverEntity.ID, "available", nil); err != nil {
		return response.ImportedServer{}, err
	}

	// 在同一会话上进行工具发现
	discoverCtx, discoverCancel := context.WithTimeout(ctx, 30*time.Second)
	defer discoverCancel()
	discoveredTools, listErr := session.ListTools(discoverCtx)
	if listErr != nil {
		return response.ImportedServer{
			Server:   response.ConvertToServerSummary(serverEntity, nil, nil),
			Warnings: []string{fmt.Sprintf("工具发现失败: %s", listErr.Error())},
		}, nil
	}

	// 处理并保存发现的工具
	toolEntities := make([]entity.MCPTool, 0, len(discoveredTools))
	toolSummaries := make([]response.MCPToolSummary, 0, len(discoveredTools))
	now := time.Now().UTC()
	for _, t := range discoveredTools {
		schemaHash := computeSchemaHash(t.InputSchema)
		toolEntity := entity.MCPTool{
			ID:            uuid.New().String(),
			ServerID:      serverEntity.ID,
			ToolName:      t.Name,
			Description:   &t.Description,
			InputSchema:   t.InputSchema,
			SchemaHash:    schemaHash,
			ReadOnly:      true,
			Enabled:       false,
			DiscoveredAt:  now,
			LastCheckedAt: now,
		}
		toolEntities = append(toolEntities, toolEntity)
		toolSummaries = append(toolSummaries, response.MCPToolSummary{
			ID:          "",
			ToolName:    t.Name,
			Description: &t.Description,
			ReadOnly:    true,
			Enabled:     false,
		})
	}
	var warnings []string
	if batchErr := s.tools.BatchCreate(ctx, toolEntities); batchErr != nil {
		if errors.Is(batchErr, repository.ErrDuplicateResource) {
			warnings = []string{"工具已导入，不能重复导入"}
		} else {
			warnings = []string{fmt.Sprintf("工具导入失败: %s", batchErr.Error())}
		}
	}

	return response.ImportedServer{
		Server:   response.ConvertToServerSummary(serverEntity, toolSummaries, warnings),
		Warnings: warnings,
	}, nil
}

func (s *importService) importHTTPServer(ctx context.Context, userID string, name string, config request.MCPServerConfig) (*entity.MCPServer, error) {
	if config.URL == nil || *config.URL == "" {
		return nil, fmt.Errorf("url is required for http transport")
	}
	allowHTTP := s.cfg.MCP.AllowLocalHTTP && s.cfg.App.Mode == "debug"
	if err := mcp.ValidateURL(*config.URL, allowHTTP); err != nil {
		return nil, err
	}
	if err := mcp.ValidateHeaders(config.Headers); err != nil {
		return nil, err
	}
	headersCiphertext, err := mcp.EncryptStringMap(config.Headers, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt headers: %w", err)
	}
	masked := mcp.MaskStringMap(config.Headers)
	authMasked := extractAuthMasked(masked)

	connectTimeoutMs := 15000 // HTTP 连接通常较快，15 秒足够
	callTimeoutMs := 120000   // 工具调用可能耗时较长，默认 2 分钟
	maxResponseBytes := 1024 * 1024
	enabled := true

	if config.ConnectTimeoutMs != nil && *config.ConnectTimeoutMs > 0 {
		connectTimeoutMs = *config.ConnectTimeoutMs
	}
	if config.CallTimeoutMs != nil && *config.CallTimeoutMs > 0 {
		callTimeoutMs = *config.CallTimeoutMs
	}
	if config.MaxResponseBytes != nil && *config.MaxResponseBytes > 0 {
		maxResponseBytes = *config.MaxResponseBytes
	}
	if config.Enabled != nil {
		enabled = *config.Enabled
	}

	return &entity.MCPServer{
		BaseEntity:        entity.BaseEntity{ID: uuid.New().String()},
		UserID:            userID,
		Name:              name,
		Description:       config.Description,
		Transport:         "streamable_http",
		URL:               config.URL,
		HeadersCiphertext: headersCiphertext,
		AuthMasked:        authMasked,
		ConnectTimeoutMs:  connectTimeoutMs,
		CallTimeoutMs:     callTimeoutMs,
		MaxResponseBytes:  maxResponseBytes,
		Enabled:           enabled,
		ConnectionStatus:  "unknown",
	}, nil
}

func (s *importService) importStdioServer(ctx context.Context, userID string, name string, config request.MCPServerConfig) (*entity.MCPServer, error) {
	if config.Command == nil || *config.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}
	command := *config.Command
	args := make([]string, 0, len(config.Args))
	if config.Args != nil {
		args = append(args, config.Args...)
	}
	env := make(map[string]string)
	if config.Env != nil {
		env = config.Env
	}

	if err := mcp.ValidateCommand(command, s.cfg.MCP.StdioCommandWhitelist); err != nil {
		return nil, err
	}
	if err := mcp.ValidateArgs(args); err != nil {
		return nil, err
	}
	if err := mcp.ValidateEnv(env); err != nil {
		return nil, err
	}
	argsCiphertext, err := mcp.EncryptStringSlice(args, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt args: %w", err)
	}
	envCiphertext, err := mcp.EncryptStringMap(env, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt env: %w", err)
	}
	masked := mcp.MaskStringMap(env)
	authMasked := extractAuthMasked(masked)

	connectTimeoutMs := 45000 // stdio 需启动子进程（npx/pip 等），45 秒保证首次安装不超时
	callTimeoutMs := 120000   // 工具调用可能耗时较长，默认 2 分钟
	maxResponseBytes := 1024 * 1024
	enabled := true

	if config.CallTimeoutMs != nil && *config.CallTimeoutMs > 0 {
		callTimeoutMs = *config.CallTimeoutMs
	}
	if config.MaxResponseBytes != nil && *config.MaxResponseBytes > 0 {
		maxResponseBytes = *config.MaxResponseBytes
	}
	if config.Enabled != nil {
		enabled = *config.Enabled
	}

	return &entity.MCPServer{
		BaseEntity:       entity.BaseEntity{ID: uuid.New().String()},
		UserID:           userID,
		Name:             name,
		Description:      config.Description,
		Transport:        "stdio",
		Command:          config.Command,
		CWD:              config.CWD,
		ArgsCiphertext:   argsCiphertext,
		EnvCiphertext:    envCiphertext,
		AuthMasked:       authMasked,
		ConnectTimeoutMs: connectTimeoutMs,
		CallTimeoutMs:    callTimeoutMs,
		MaxResponseBytes: maxResponseBytes,
		Enabled:          enabled,
		ConnectionStatus: "unknown",
	}, nil
}

func (s *importService) discoverAndImportTools(ctx context.Context, serverID string, target mcp.MCPServerTarget) ([]response.MCPToolSummary, []string) {
	client := s.clientProvider()
	timeout := 30 * time.Second
	discoverCtx, discoverCancel := context.WithTimeout(ctx, timeout)
	defer discoverCancel()

	tools, err := mcp.DiscoverTools(discoverCtx, client, target, timeout)
	if err != nil {
		return []response.MCPToolSummary{}, []string{fmt.Sprintf("工具发现失败: %s", err.Error())}
	}

	toolEntities := make([]entity.MCPTool, 0, len(tools))
	toolSummaries := make([]response.MCPToolSummary, 0, len(tools))
	now := time.Now().UTC()

	for _, t := range tools {
		schemaHash := computeSchemaHash(t.InputSchema)
		toolEntity := entity.MCPTool{
			ID:            uuid.New().String(),
			ServerID:      serverID,
			ToolName:      t.Name,
			Description:   &t.Description,
			InputSchema:   t.InputSchema,
			SchemaHash:    schemaHash,
			ReadOnly:      true, // P0 默认只读
			Enabled:       false,
			DiscoveredAt:  now,
			LastCheckedAt: now,
		}
		toolEntities = append(toolEntities, toolEntity)
		toolSummaries = append(toolSummaries, response.MCPToolSummary{
			ID:          "",
			ToolName:    t.Name,
			Description: &t.Description,
			ReadOnly:    true,
			Enabled:     false,
		})
	}

	if err := s.tools.BatchCreate(ctx, toolEntities); err != nil {
		if errors.Is(err, repository.ErrDuplicateResource) {
			return toolSummaries, []string{buildDuplicateToolWarning(ctx, s, serverID, toolEntities)}
		}
		return toolSummaries, []string{fmt.Sprintf("工具导入失败: %s", err.Error())}
	}

	return toolSummaries, nil
}

// buildDuplicateToolWarning 生成"工具已导入，不能重复导入"的警告信息，并列出已存在的工具名。
func buildDuplicateToolWarning(ctx context.Context, s *importService, serverID string, toolEntities []entity.MCPTool) string {
	existing, findErr := s.tools.FindByServer(ctx, serverID)
	if findErr == nil {
		existingSet := make(map[string]struct{}, len(existing))
		for _, e := range existing {
			existingSet[e.ToolName] = struct{}{}
		}
		var duplicates []string
		for _, t := range toolEntities {
			if _, ok := existingSet[t.ToolName]; ok {
				duplicates = append(duplicates, t.ToolName)
			}
		}
		if len(duplicates) > 0 {
			return fmt.Sprintf("工具已导入，不能重复导入: %s", strings.Join(duplicates, ", "))
		}
	}
	return "工具已导入，不能重复导入"
}

func (s *importService) buildTarget(server *entity.MCPServer) mcp.MCPServerTarget {
	target := mcp.MCPServerTarget{Transport: server.Transport, MaxResponseBytes: server.MaxResponseBytes}

	if server.Transport == "streamable_http" {
		if server.URL != nil {
			target.URL = *server.URL
		}
		if server.HeadersCiphertext != nil {
			headers, _ := mcp.DecryptStringMap(server.HeadersCiphertext, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
			target.Headers = headers
		}
	} else if server.Transport == "stdio" {
		if server.Command != nil {
			target.Command = *server.Command
		}
		if server.CWD != nil {
			target.CWD = *server.CWD
		}
		if server.ArgsCiphertext != nil {
			args, _ := mcp.DecryptStringSlice(server.ArgsCiphertext, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
			target.Args = args
		}
		if server.EnvCiphertext != nil {
			env, _ := mcp.DecryptStringMap(server.EnvCiphertext, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
			target.Env = env
		}
	}

	return target
}

func (s *importService) List(ctx context.Context, userID string) (*response.MCPServerListResponse, error) {
	servers, err := s.servers.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	summaries := make([]response.MCPServerSummary, 0, len(servers))
	for i := range servers {
		tools, err := s.tools.FindByServer(ctx, servers[i].ID)
		if err != nil {
			return nil, err
		}
		toolSummaries := make([]response.MCPToolSummary, 0, len(tools))
		for _, t := range tools {
			toolSummaries = append(toolSummaries, response.ConvertToToolSummary(&t))
		}
		summaries = append(summaries, response.ConvertToServerSummary(&servers[i], toolSummaries, []string{}))
	}
	return &response.MCPServerListResponse{Servers: summaries}, nil
}

func (s *importService) GetDetail(ctx context.Context, userID string, serverID string) (*response.MCPServerDetailResponse, error) {
	server, err := s.servers.FindActiveByID(ctx, userID, serverID)
	if err != nil {
		return nil, err
	}
	tools, err := s.tools.FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	toolDetails := make([]response.MCPToolDetail, 0, len(tools))
	toolSummaries := make([]response.MCPToolSummary, 0, len(tools))
	for _, t := range tools {
		toolDetails = append(toolDetails, response.ConvertToToolDetail(&t))
		toolSummaries = append(toolSummaries, response.ConvertToToolSummary(&t))
	}
	return &response.MCPServerDetailResponse{
		Server: response.ConvertToServerSummary(server, toolSummaries, []string{}),
		Tools:  toolDetails,
	}, nil
}

func (s *importService) Delete(ctx context.Context, userID string, serverID string) error {
	if err := s.servers.Delete(ctx, userID, serverID); err != nil {
		return err
	}
	if err := s.tools.DeleteByServer(ctx, serverID); err != nil {
		return err
	}
	return nil
}

func (s *importService) TestConnection(ctx context.Context, userID string, serverID string) (*response.MCPTestResult, error) {
	server, err := s.servers.FindActiveByID(ctx, userID, serverID)
	if err != nil {
		return nil, err
	}
	target := s.buildTarget(server)
	client := s.clientProvider()
	testCtx, testCancel := context.WithTimeout(ctx, time.Duration(server.ConnectTimeoutMs)*time.Millisecond)
	defer testCancel()
	start := time.Now()
	if err := mcp.TestConnection(testCtx, client, target, 0); err != nil {
		statusErr := err.Error()
		if statusErr == "" {
			statusErr = "connection failed"
		}
		if updErr := s.servers.UpdateStatus(ctx, serverID, "unavailable", &statusErr); updErr != nil {
			return nil, updErr
		}
		return &response.MCPTestResult{
			Success:        false,
			Available:      false,
			ResponseTimeMs: time.Since(start).Milliseconds(),
			ErrorMessage:   statusErr,
			LastTestedAt:   time.Now().UTC(),
		}, nil
	}
	if updErr := s.servers.UpdateStatus(ctx, serverID, "available", nil); updErr != nil {
		return nil, updErr
	}
	return &response.MCPTestResult{
		Success:        true,
		Available:      true,
		ResponseTimeMs: time.Since(start).Milliseconds(),
		LastTestedAt:   time.Now().UTC(),
	}, nil
}

func (s *importService) DiscoverTools(ctx context.Context, userID string, serverID string) (*response.MCPDiscoverResult, error) {
	server, err := s.servers.FindActiveByID(ctx, userID, serverID)
	if err != nil {
		return nil, err
	}
	target := s.buildTarget(server)
	tools, warnings := s.discoverAndImportTools(ctx, serverID, target)
	return &response.MCPDiscoverResult{
		Tools:    tools,
		Warnings: warnings,
	}, nil
}

func (s *importService) UpdateToolStatus(ctx context.Context, userID string, toolID string, enabled bool) error {
	if err := s.tools.UpdateEnabledByUser(ctx, userID, toolID, enabled); err != nil {
		return err
	}
	return nil
}

func (s *importService) UpdateServerEnabled(ctx context.Context, userID string, serverID string, enabled bool) error {
	if err := s.servers.UpdateEnabled(ctx, userID, serverID, enabled); err != nil {
		return err
	}
	return nil
}

// ListEnabledTools 返回当前用户已启用（Server 与 Tool 均启用）的 MCP 工具。
// 供 Agent 准备阶段的第一层拦截使用：只把已启用的 MCP 工具加入模型可见的工具集合。
func (s *importService) ListEnabledTools(ctx context.Context, userID string) ([]response.MCPEnabledTool, error) {
	tools, err := s.tools.ListEnabledByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]response.MCPEnabledTool, 0, len(tools))
	for i := range tools {
		result = append(result, response.MCPEnabledTool{
			ServerID: tools[i].ServerID,
			ToolName: tools[i].ToolName,
			ReadOnly: tools[i].ReadOnly,
		})
	}
	return result, nil
}

// ListEnabledToolsForRegistry 实现 tools.MCPToolProvider 接口。
// 返回用户已启用的 MCP 工具的完整元数据，包含 Server 连接信息和工具 Schema，
// 供工具注册表在 Agent 启动前动态加载（第一层校验）。
func (s *importService) ListEnabledToolsForRegistry(ctx context.Context, userID string) ([]tools.MCPToolMetadata, error) {
	// 1. 获取用户所有已启用的工具实体（包含 ServerID 和工具基础信息）
	enabledTools, err := s.tools.ListEnabledByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询已启用工具失败: %w", err)
	}

	if len(enabledTools) == 0 {
		return []tools.MCPToolMetadata{}, nil
	}

	// 2. 收集所有涉及的 Server ID
	serverIDs := make(map[string]struct{})
	for i := range enabledTools {
		serverIDs[enabledTools[i].ServerID] = struct{}{}
	}

	// 3. 批量查询所有相关的 Server 实体（包含连接配置）
	serverMap := make(map[string]*entity.MCPServer)
	for serverID := range serverIDs {
		server, err := s.servers.FindActiveByID(ctx, userID, serverID)
		if err != nil {
			// Server 不存在或无权访问时跳过该 Server 的所有工具
			continue
		}
		serverMap[serverID] = server
	}

	// 4. 组装工具元数据列表
	result := make([]tools.MCPToolMetadata, 0, len(enabledTools))
	for i := range enabledTools {
		tool := &enabledTools[i]
		server, ok := serverMap[tool.ServerID]
		if !ok {
			// Server 不可用，跳过该工具
			continue
		}

		// 构造 Server 连接目标（复用 buildTarget，自动解密 headers/args/env）
		target := s.buildTarget(server)

		// 构造工具元数据
		toolMetadata := mcp.MCPServerTool{
			Name:        tool.ToolName,
			Description: derefString(tool.Description),
			InputSchema: tool.InputSchema,
		}

		// 添加到结果集
		result = append(result, tools.MCPToolMetadata{
			ServerID:      server.ID,
			ServerTarget:  target,
			ToolMetadata:  toolMetadata,
			Enabled:       tool.Enabled,
			CallTimeoutMs: server.CallTimeoutMs,
		})
	}

	return result, nil
}

// CheckToolAvailable 实现 tools.ToolAvailabilityChecker。
// 这是第二层拦截：在 MCP 工具真正执行前，向数据层动态复核 Server 与 Tool 的启用状态，
// 从而捕捉运行过程中前端动态禁用工具的情况（注册时的快照已经不可信）。
func (s *importService) CheckToolAvailable(ctx context.Context, userID contracts.ID, spec contracts.ToolSpec) (bool, error) {
	// 仅对 MCP 工具做动态复核；内置工具 SourceID 为空，直接放行。
	if spec.Type != contracts.ToolTypeMCP || spec.SourceID == "" {
		return true, nil
	}
	enabled, err := s.tools.IsEnabled(ctx, string(userID), spec.SourceID, spec.Name)
	if err != nil {
		return false, fmt.Errorf("check mcp tool availability: %w", err)
	}
	return enabled, nil
}

// 编译期断言：importService 满足 tools.ToolAvailabilityChecker，可供 Executor 注入。
var _ tools.ToolAvailabilityChecker = (*importService)(nil)

// 编译期断言：importService 满足 tools.MCPToolProvider，可供 MCPToolRefresher 注入。
var _ tools.MCPToolProvider = (*importService)(nil)

// determineTransport 根据 config 判定传输类型。
func determineTransport(config *request.MCPServerConfig) string {
	if config.Transport != nil && (*config.Transport == "streamable_http" || *config.Transport == "stdio") {
		return *config.Transport
	}
	if config.Command != nil && *config.Command != "" {
		return "stdio"
	}
	return "streamable_http"
}

// computeSchemaHash 计算 input schema 的 SHA256 hash，用于检测 Schema 变更。
func computeSchemaHash(schema json.RawMessage) string {
	if schema == nil {
		return ""
	}
	hash := sha256.Sum256(schema)
	return hex.EncodeToString(hash[:])
}

// extractAuthMasked 从 headers/env 脱敏 map 中提取脱敏后的 Authorization/Apisecret 作为 auth_masked。
func extractAuthMasked(masked map[string]string) *string {
	for k, v := range masked {
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Api-Key") {
			return &v
		}
	}
	return nil
}

// mapErrorToCode 将内部错误映射到响应错误码。
func mapErrorToCode(err error) string {
	if errors.Is(err, repository.ErrMCPConnectionFailed) {
		return string(contracts.ErrMCPConnectionFailed)
	}
	if errors.Is(err, repository.ErrDuplicateResource) {
		return string(contracts.ErrDuplicateResource)
	}
	if errors.Is(err, repository.ErrMCPServerNotFound) {
		return string(contracts.ErrResourceNotFound)
	}
	if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "required") {
		return string(contracts.ErrInvalidArgument)
	}
	return string(contracts.ErrInternal)
}
