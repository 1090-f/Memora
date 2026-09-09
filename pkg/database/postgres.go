package database

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitPostgres 初始化PostgreSQL数据库连接并配置连接池参数
func InitPostgres(ctx context.Context, cfg *config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: NewGormLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("打开 PostgreSQL 连接失败: %w", err)
	}
	if err := registerGormTracing(db); err != nil {
		return nil, fmt.Errorf("注册 PostgreSQL Trace 回调失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 PostgreSQL 连接池失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("Ping PostgreSQL 失败: %w", err)
	}
	return db, nil
}

// ClosePostgres 关闭PostgreSQL数据库连接
func ClosePostgres(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CheckPostgres 检查PostgreSQL数据库连接是否健康
func CheckPostgres(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("PostgreSQL 尚未初始化")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
