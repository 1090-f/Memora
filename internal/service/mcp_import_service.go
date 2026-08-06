package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/mcp"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/google/uuid"
)

// MCPServerRepository 是 MCP ImportService 的接口。
type MCPServerRepository interface {
	FindActiveByName(ctx context.Context, userID, name string) (*entity.MCPServer, error)
	FindActiveByID(ctx context.Context, userID, serverID string) (*entity.MCPServer, error)
	ListByUser(ctx context.Context, userID string) ([]entity.MCPServer, error)
	Create(ctx context.Context, server *entity.MCPServer) error
	UpdateStatus(ctx context.Context, serverID, status string, lastErr *string) error
	UpdateEnabled(ctx context.Context, userID, serverID string, enabled bool) error
	Delete(ctx context.Context, userID, serverID string) error
}

// MCPToolRepository 是 MCP ImportService 的工具接口。
type MCPToolRepository interface {
	FindByServer(ctx context.Context, serverID string) ([]entity.MCPTool, error)
	BatchCreate(ctx context.Context, tools []entity.MCPTool) error
	DeleteByServer(ctx context.Context, serverID string) error
	UpdateEnabledByUser(ctx context.Context, userID, toolID string, enabled bool) error
	UpdateSchema(ctx context.Context, toolID, schemaHash string, schema []byte) error
}

// ImportService 定义了 MCP Server 导入的管理接口。
type ImportService interface {
	Import(ctx context.Context, userID string, req *request.MCPImportRequest) (*response.MCPImportResponse, error)
	List(ctx context.Context, userID string) (*response.MCPServerListResponse, error)
	GetDetail(ctx context.Context, userID string, serverID string) (*response.MCPServerDetailResponse, error)
	Delete(ctx context.Context, userID string, serverID string) error
	TestConnection(ctx context.Context, userID string, serverID string) (*response.MCPTestResult, error)
	DiscoverTools(ctx context.Context, userID string, serverID string) (*response.MCPDiscoverResult, error)
	UpdateToolStatus(ctx context.Context, userID string, toolID string, enabled bool) error
	UpdateServerEnabled(ctx context.Context, userID string, serverID string, enabled bool) error
}

// importService 是 ImportService 的实现。
type importService struct {
	servers        MCPServerRepository
	tools          MCPToolRepository
	cfg            *config.Config
	clientProvider func() mcp.MCPClient
}

// NewImportService 创建 ImportService 实例。
func NewImportService(servers MCPServerRepository, tools MCPToolRepository, cfg *config.Config) ImportService {
	return &importService{
		servers: servers,
		tools:   tools,
		cfg:     cfg,
		clientProvider: func() mcp.MCPClient {
			return mcp.NewMCPClient()
		},
	}
}

// Import 执行 MCP Server 导入流程。
func (s *importService) Import(ctx context.Context, userID string, req *request.MCPImportRequest) (*response.MCPImportResponse, error) {
	if req == nil || len(req.MCPServers) == 0 {
		return nil, repository.ErrInvalidArgument
	}
	if len(req.MCPServers) > 20 {
		return nil, repository.ErrInvalidArgument
	}

	imported := make([]response.ImportedServer, 0, len(req.MCPServers))
	failed := make([]response.FailedServer, 0)

	for name, serverConfig := range req.MCPServers {
		name = strings.TrimSpace(name)
		if name == "" {
			failed = append(failed, response.FailedServer{
				Name:    name,
				Error:   "INVALID_ARGUMENT",
				Message: "server name is empty",
			})
			continue
		}

		server, err := s.importSingleServer(ctx, userID, name, serverConfig)
		if err != nil {
			failed = append(failed, response.FailedServer{
				Name:    name,
				Error:   mapErrorToCode(err),
				Message: err.Error(),
			})
			continue
		}

		imported = append(imported, server)
	}

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

	if err := s.servers.Create(ctx, serverEntity); err != nil {
		if errors.Is(err, repository.ErrDuplicateResource) {
			return response.ImportedServer{}, repository.ErrDuplicateResource
		}
		return response.ImportedServer{}, err
	}

	target := s.buildTarget(serverEntity)
	client := s.clientProvider()
	connCtx, connCancel := context.WithTimeout(ctx, time.Duration(serverEntity.ConnectTimeoutMs)*time.Millisecond)
	defer connCancel()
	if err := mcp.TestConnection(connCtx, client, target, 0); err != nil {
		statusErr := err.Error()
		if statusErr == "" {
			statusErr = "connection failed"
		}
		if updErr := s.servers.UpdateStatus(ctx, serverEntity.ID, "unavailable", &statusErr); updErr != nil {
			return response.ImportedServer{}, updErr
		}
		serverEntity.ConnectionStatus = "unavailable"
		serverEntity.LastError = &statusErr
	} else {
		if updErr := s.servers.UpdateStatus(ctx, serverEntity.ID, "available", nil); updErr != nil {
			return response.ImportedServer{}, updErr
		}
		serverEntity.ConnectionStatus = "available"
		serverEntity.LastError = nil
	}

	tools, warnings := s.discoverAndImportTools(ctx, serverEntity.ID, target)

	return response.ImportedServer{
		Server:   response.ConvertToServerSummary(serverEntity, tools, warnings),
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

	connectTimeoutMs := 5000
	callTimeoutMs := 30000
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

	connectTimeoutMs := 5000
	callTimeoutMs := 30000
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
	timeout := 10 * time.Second
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
		return toolSummaries, []string{fmt.Sprintf("工具导入失败: %s", err.Error())}
	}

	return toolSummaries, nil
}

func (s *importService) buildTarget(server *entity.MCPServer) mcp.MCPServerTarget {
	target := mcp.MCPServerTarget{Transport: server.Transport}

	if server.Transport == "streamable_http" {
		if server.URL != nil {
			target.URL = *server.URL
		}
		if server.HeadersCiphertext != nil {
			headers, _ := mcp.DecryptStringMap(server.HeadersCiphertext, mcp.DeriveKey(s.cfg.MCP.EncryptionKey))
			target.Headers = headers
		}
	} else {
		if server.Command != nil {
			target.Command = *server.Command
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
