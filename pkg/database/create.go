package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const databaseBootstrapTimeout = 10 * time.Second

// ensurePostgresDatabase creates the target database when it does not exist.
func ensurePostgresDatabase(databaseURL string) error {
	maintenanceURL, databaseName, err := databaseBootstrapTarget(databaseURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), databaseBootstrapTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, maintenanceURL)
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 维护数据库失败: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists); err != nil {
		return fmt.Errorf("检查数据库 %q 失败: %w", databaseName, err)
	}
	if exists {
		return nil
	}

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+quotePostgresIdentifier(databaseName)); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("创建数据库 %q 失败: %w", databaseName, err)
	}
	return nil
}

func databaseBootstrapTarget(databaseURL string) (string, string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", "", fmt.Errorf("解析数据库连接串失败: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", "", fmt.Errorf("数据库连接串必须使用 postgres 或 postgresql 协议")
	}

	escapedName := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if escapedName == "" || strings.Contains(escapedName, "/") {
		return "", "", fmt.Errorf("数据库连接串必须只包含一个数据库名称")
	}
	databaseName, err := url.PathUnescape(escapedName)
	if err != nil {
		return "", "", fmt.Errorf("解码数据库名称失败: %w", err)
	}
	if databaseName == "" || strings.ContainsRune(databaseName, '\x00') {
		return "", "", fmt.Errorf("数据库连接串包含无效的数据库名称")
	}

	parsed.Path = "/postgres"
	parsed.RawPath = ""
	return parsed.String(), databaseName, nil
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
