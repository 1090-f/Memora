package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
)

// main 是数据库迁移工具的入口点，支持执行迁移、引导管理员和重置密码操作。
func main() {
	if len(os.Args) != 2 {
		usage()
	}
	cfg, err := config.LoadDatabase("")
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "up", "down":
		err = database.Migrate(cfg.Database.URL, os.Args[1])
	case "bootstrap-admin":
		err = bootstrapAdmin(context.Background(), cfg)
	case "reset-admin-password":
		err = resetAdminPassword(context.Background(), cfg)
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

// usage 显示命令行用法信息并退出程序。
func usage() {
	fmt.Fprintln(os.Stderr, "usage: memora-migrate <up|down|bootstrap-admin|reset-admin-password>")
	os.Exit(2)
}

// bootstrapAdmin 引导创建管理员账户，从环境变量读取用户名、邮箱和密码。
func bootstrapAdmin(ctx context.Context, cfg *config.Config) error {
	username := strings.TrimSpace(os.Getenv("MEMORA_BOOTSTRAP_ADMIN_USERNAME"))
	email := strings.TrimSpace(os.Getenv("MEMORA_BOOTSTRAP_ADMIN_EMAIL"))
	if username == "" {
		return fmt.Errorf("MEMORA_BOOTSTRAP_ADMIN_USERNAME is required")
	}
	db, err := database.InitPostgres(ctx, &cfg.Database)
	if err != nil {
		return err
	}
	defer database.ClosePostgres(db)

	var exists bool
	if err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check bootstrap administrator: %w", err)
	}
	if exists {
		log.Printf("bootstrap administrator %q already exists; credentials were not changed", username)
		return nil
	}

	password := os.Getenv("MEMORA_BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" || password == "" {
		return fmt.Errorf("MEMORA_BOOTSTRAP_ADMIN_EMAIL and MEMORA_BOOTSTRAP_ADMIN_PASSWORD are required for first bootstrap")
	}
	if err := validateAdminPassword(cfg.App.Mode, password); err != nil {
		return err
	}
	hash, err := service.HashPassword(password)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Exec(`
		INSERT INTO users (username, email, password_hash, nickname, status)
		VALUES (?, ?, ?, ?, 'active')
		ON CONFLICT (username) DO NOTHING`,
		username, email, hash, username)
	if result.Error != nil {
		return fmt.Errorf("bootstrap administrator: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("bootstrap administrator %q was created concurrently; credentials were not changed", username)
	}
	return nil
}

// resetAdminPassword 重置指定管理员账户的密码，从环境变量读取用户名和新密码。
func resetAdminPassword(ctx context.Context, cfg *config.Config) error {
	username := strings.TrimSpace(os.Getenv("MEMORA_BOOTSTRAP_ADMIN_USERNAME"))
	password := os.Getenv("MEMORA_BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return fmt.Errorf("MEMORA_BOOTSTRAP_ADMIN_USERNAME and MEMORA_BOOTSTRAP_ADMIN_PASSWORD are required")
	}
	if err := validateAdminPassword(cfg.App.Mode, password); err != nil {
		return err
	}
	hash, err := service.HashPassword(password)
	if err != nil {
		return err
	}
	db, err := database.InitPostgres(ctx, &cfg.Database)
	if err != nil {
		return err
	}
	defer database.ClosePostgres(db)
	result := db.WithContext(ctx).Exec(`
		UPDATE users
		SET password_hash = ?, status = 'active', deleted_at = NULL, updated_at = now()
		WHERE username = ?`, hash, username)
	if result.Error != nil {
		return fmt.Errorf("reset administrator password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("administrator %q does not exist", username)
	}
	return nil
}

// validateAdminPassword 验证管理员密码强度，确保长度足够且不是示例密码。
func validateAdminPassword(mode, password string) error {
	if len(password) < 12 {
		return fmt.Errorf("administrator password must contain at least 12 characters")
	}
	if mode == "release" && strings.HasPrefix(strings.ToLower(password), "change-me") {
		return fmt.Errorf("administrator password must not use an example value in release mode")
	}
	return nil
}
