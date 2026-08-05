package database

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrate 执行数据库迁移，direction 支持 "up"（向上迁移）和 "down"（向下回滚）
func Migrate(databaseURL, direction string) error {
	switch direction {
	case "up":
		if err := ensurePostgresDatabase(databaseURL); err != nil {
			return err
		}
	case "down":
	default:
		return fmt.Errorf("不支持的迁移方向 %q", direction)
	}

	source, err := migrationSource()
	if err != nil {
		return err
	}
	migrator, err := migrate.New(source, databaseURL)
	if err != nil {
		return fmt.Errorf("创建迁移器失败: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()
	switch direction {
	case "up":
		err = migrator.Up()
	case "down":
		err = migrator.Down()
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("数据库迁移 %s 失败: %w", direction, err)
	}
	return nil
}

func migrationSource() (string, error) {
	candidates := []string{"scripts/migrations"}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "migrations"))
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && info.IsDir() {
			return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String(), nil
		}
	}
	return "", errors.New("找不到 scripts/migrations 目录")
}
