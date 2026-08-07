package repository

import (
	"context"

	"gorm.io/gorm"
)

// txContextKey 是 context 中存放事务 DB 的私有键，仅本包内部使用。
type txContextKey struct{}

// withTx 返回携带事务 DB 的上下文。
func withTx(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, db)
}

// dbFromContext 从上下文读取事务 DB；不在事务中时返回 fallback DB。
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	// 优先取事务 DB：保证同一业务事务内的所有仓储操作共用同一连接与锁、可整体回滚。
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}
