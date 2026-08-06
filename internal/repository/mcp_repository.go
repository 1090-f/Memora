package repository

import (
	"context"
	"errors"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrMCPServerNotFound = errors.New("mcp server not found")
	ErrInvalidArgument   = errors.New("invalid argument")
	ErrDuplicateResource = errors.New("duplicate resource")
)

// MCPServerRepository 定义了 MCP Server 的数据访问接口。
type MCPServerRepository interface {
	// FindActiveByName 根据 user_id 和 name 查找未删除的 MCP Server。
	FindActiveByName(ctx context.Context, userID, name string) (*entity.MCPServer, error)
	// FindActiveByID 根据 user_id 和 id 查找未删除的 MCP Server。
	FindActiveByID(ctx context.Context, userID, serverID string) (*entity.MCPServer, error)
	// ListByUser 列出用户下所有未删除的 MCP Server。
	ListByUser(ctx context.Context, userID string) ([]entity.MCPServer, error)
	// Create 创建一个 MCP Server。
	Create(ctx context.Context, server *entity.MCPServer) error
	// UpdateStatus 更新 MCP Server 的连接状态与错误信息。
	UpdateStatus(ctx context.Context, serverID, status string, lastErr *string) error
	// UpdateEnabled 更新 MCP Server 的启用状态。
	UpdateEnabled(ctx context.Context, userID, serverID string, enabled bool) error
	// Delete 软删除 MCP Server（设置 deleted_at）。
	Delete(ctx context.Context, userID, serverID string) error
}

// MCPToolRepository 定义了 MCP Tool 的数据访问接口。
type MCPToolRepository interface {
	// FindByServer 查找指定 Server 下的所有工具。
	FindByServer(ctx context.Context, serverID string) ([]entity.MCPTool, error)
	// BatchCreate 批量创建工具。
	BatchCreate(ctx context.Context, tools []entity.MCPTool) error
	// DeleteByServer 删除指定 Server 下的所有工具。
	DeleteByServer(ctx context.Context, serverID string) error
	// UpdateEnabledByUser 更新工具启用状态，并校验工具属于用户。
	UpdateEnabledByUser(ctx context.Context, userID, toolID string, enabled bool) error
	// UpdateSchema 更新工具 Schema（Schema 变更时）。
	UpdateSchema(ctx context.Context, toolID, schemaHash string, schema []byte) error
}

// mcpServerRepository 是 MCPServerRepository 的 GORM 实现。
type mcpServerRepository struct {
	db *gorm.DB
}

// NewMCPServerRepository 创建 MCPServerRepository 实例。
func NewMCPServerRepository(db *gorm.DB) MCPServerRepository {
	return &mcpServerRepository{db: db}
}

func (r *mcpServerRepository) FindActiveByName(ctx context.Context, userID, name string) (*entity.MCPServer, error) {
	var server entity.MCPServer
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND lower(name) = lower(?) AND deleted_at IS NULL", userID, name).
		First(&server).Error
	return mapMCPServerResult(&server, err)
}

func (r *mcpServerRepository) FindActiveByID(ctx context.Context, userID, serverID string) (*entity.MCPServer, error) {
	var server entity.MCPServer
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", serverID, userID).
		First(&server).Error
	return mapMCPServerResult(&server, err)
}

func (r *mcpServerRepository) ListByUser(ctx context.Context, userID string) ([]entity.MCPServer, error) {
	var servers []entity.MCPServer
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&servers).Error
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *mcpServerRepository) Create(ctx context.Context, server *entity.MCPServer) error {
	if err := r.db.WithContext(ctx).Create(server).Error; err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrDuplicateResource
		}
		return err
	}
	return nil
}

func (r *mcpServerRepository) UpdateStatus(ctx context.Context, serverID, status string, lastErr *string) error {
	result := r.db.WithContext(ctx).Model(&entity.MCPServer{}).
		Where("id = ?", serverID).
		Updates(map[string]any{
			"connection_status": status,
			"last_tested_at":    r.db.NowFunc(),
			"last_error":        lastErr,
			"updated_at":        r.db.NowFunc(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func (r *mcpServerRepository) Delete(ctx context.Context, userID, serverID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", serverID, userID).
		Delete(&entity.MCPServer{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func (r *mcpServerRepository) UpdateEnabled(ctx context.Context, userID, serverID string, enabled bool) error {
	result := r.db.WithContext(ctx).Model(&entity.MCPServer{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", serverID, userID).
		Update("enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func mapMCPServerResult(server *entity.MCPServer, err error) (*entity.MCPServer, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMCPServerNotFound
	}
	if err != nil {
		return nil, err
	}
	return server, nil
}

// mcpToolRepository 是 MCPToolRepository 的 GORM 实现。
type mcpToolRepository struct {
	db *gorm.DB
}

// NewMCPToolRepository 创建 MCPToolRepository 实例。
func NewMCPToolRepository(db *gorm.DB) MCPToolRepository {
	return &mcpToolRepository{db: db}
}

func (r *mcpToolRepository) FindByServer(ctx context.Context, serverID string) ([]entity.MCPTool, error) {
	var tools []entity.MCPTool
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("tool_name ASC").
		Find(&tools).Error
	return tools, err
}

func (r *mcpToolRepository) BatchCreate(ctx context.Context, tools []entity.MCPTool) error {
	if len(tools) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range tools {
			if err := tx.Create(&tools[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *mcpToolRepository) DeleteByServer(ctx context.Context, serverID string) error {
	return r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Delete(&entity.MCPTool{}).Error
}

func (r *mcpToolRepository) UpdateEnabledByUser(ctx context.Context, userID, toolID string, enabled bool) error {
	result := r.db.WithContext(ctx).Model(&entity.MCPTool{}).
		Where("mcp_tools.id = ? AND mcp_tools.server_id IN (SELECT id FROM mcp_servers WHERE user_id = ? AND deleted_at IS NULL)", toolID, userID).
		Updates(map[string]any{"enabled": enabled, "updated_at": r.db.NowFunc()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func (r *mcpToolRepository) UpdateSchema(ctx context.Context, toolID, schemaHash string, schema []byte) error {
	result := r.db.WithContext(ctx).Model(&entity.MCPTool{}).
		Where("id = ?", toolID).
		Updates(map[string]any{
			"input_schema":      schema,
			"schema_hash":       schemaHash,
			"schema_changed_at": r.db.NowFunc(),
			"updated_at":        r.db.NowFunc(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}
