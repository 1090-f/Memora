package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Direction string

const (
	Up   Direction = "up"
	Down Direction = "down"
)

// Migrate applies all migrations in the requested direction.
func Migrate(ctx context.Context, databaseURL string, direction Direction) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	sourceURL, err := migrationsSourceURL()
	if err != nil {
		return err
	}
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	done := make(chan error, 1)
	go func() {
		switch direction {
		case Up:
			done <- m.Up()
		case Down:
			done <- m.Down()
		default:
			done <- fmt.Errorf("unsupported migration direction %q", direction)
		}
	}()

	select {
	case <-ctx.Done():
		m.GracefulStop <- true
		<-done
		return ctx.Err()
	case err := <-done:
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("migrate %s: %w", direction, err)
		}
		return nil
	}
}

func migrationsSourceURL() (string, error) {
	candidates := []string{filepath.Join("migrations")}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "migrations"))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
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
	return "", fmt.Errorf("locate migrations directory")
}
