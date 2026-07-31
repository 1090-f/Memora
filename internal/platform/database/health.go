package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const pingTimeout = 5 * time.Second

// Check verifies that the database connection is healthy.
func Check(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("check postgres: nil database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get postgres connection pool: %w", err)
	}
	if err := ping(ctx, sqlDB); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func ping(ctx context.Context, db *sql.DB) error {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return db.PingContext(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return db.PingContext(timeoutCtx)
}
