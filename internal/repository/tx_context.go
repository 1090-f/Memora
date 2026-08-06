package repository

import (
	"context"

	"gorm.io/gorm"
)

type txContextKey struct{}

// withTx 返回携带事务 DB 的上下文。
func withTx(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, db)
}

// dbFromContext 从上下文读取事务 DB；不在事务中时返回 fallback DB。
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}
