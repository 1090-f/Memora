package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/1090-f/Memora/internal/service"
	"gorm.io/gorm"
)

// bootstrapAdmin 在服务启动时自动创建管理员账户，从环境变量读取用户名、邮箱和密码。
// 未配置用户名时跳过；账户已存在时不会修改凭据。
func bootstrapAdmin(ctx context.Context, db *gorm.DB, mode string) error {
	username := strings.TrimSpace(os.Getenv("MEMORA_BOOTSTRAP_ADMIN_USERNAME"))
	if username == "" {
		return nil
	}

	var exists bool
	if err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists).Error; err != nil {
		return fmt.Errorf("检查引导管理员失败: %w", err)
	}
	if exists {
		return nil
	}

	email := strings.TrimSpace(os.Getenv("MEMORA_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("MEMORA_BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" || password == "" {
		return fmt.Errorf("引导管理员 %q 需要提供环境变量 MEMORA_BOOTSTRAP_ADMIN_EMAIL 和 MEMORA_BOOTSTRAP_ADMIN_PASSWORD", username)
	}
	if err := validateAdminPassword(mode, password); err != nil {
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
		return fmt.Errorf("引导管理员失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("引导管理员 %q 已存在，凭据未变更", username)
	}
	return nil
}

// validateAdminPassword 验证管理员密码强度，确保长度足够且不是示例密码。
func validateAdminPassword(mode, password string) error {
	if len(password) < 12 {
		return fmt.Errorf("管理员密码长度至少需要 12 个字符")
	}
	if mode == "release" && strings.HasPrefix(strings.ToLower(password), "change-me") {
		return fmt.Errorf("发布模式下管理员密码不能使用示例值")
	}
	return nil
}
