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
		return fmt.Errorf("connect postgres maintenance database: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists); err != nil {
		return fmt.Errorf("check database %q: %w", databaseName, err)
	}
	if exists {
		return nil
	}

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+quotePostgresIdentifier(databaseName)); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("create database %q: %w", databaseName, err)
	}
	return nil
}

func databaseBootstrapTarget(databaseURL string) (string, string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", "", fmt.Errorf("parse database URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", "", fmt.Errorf("database URL must use postgres or postgresql scheme")
	}

	escapedName := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if escapedName == "" || strings.Contains(escapedName, "/") {
		return "", "", fmt.Errorf("database URL must include exactly one database name")
	}
	databaseName, err := url.PathUnescape(escapedName)
	if err != nil {
		return "", "", fmt.Errorf("decode database name: %w", err)
	}
	if databaseName == "" || strings.ContainsRune(databaseName, '\x00') {
		return "", "", fmt.Errorf("database URL contains an invalid database name")
	}

	parsed.Path = "/postgres"
	parsed.RawPath = ""
	return parsed.String(), databaseName, nil
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
