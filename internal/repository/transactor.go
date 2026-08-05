package repository

import (
	"context"

	"gorm.io/gorm"
)

// Transactor 提供跨 Repository 的数据库事务边界。
// Service 负责开启事务，Repository 不决定业务事务边界。
type Transactor interface {
	// WithTx 在单个数据库事务中执行 fn；fn 返回错误时整体回滚。
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// transactor 是 Transactor 的 GORM 实现。
type transactor struct{ db *gorm.DB }

// NewTransactor 构造基于 GORM 的事务器。
func NewTransactor(db *gorm.DB) Transactor { return &transactor{db: db} }

// WithTx 使用 GORM 事务执行 fn，并把事务 DB 注入 context 供 Repository 读取。
func (t *transactor) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(withTx(ctx, tx))
	})
}
