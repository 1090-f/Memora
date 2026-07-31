//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/1090-f/Memora/internal/platform/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

func TestMigrateCreatesExtensionsAndUsers(t *testing.T) {
	dbURL := requireDatabaseURL(t)
	require.NoError(t, database.Migrate(context.Background(), dbURL, database.Up))
	t.Cleanup(func() {
		require.NoError(t, database.Migrate(context.Background(), dbURL, database.Down))
	})

	requireTable(t, dbURL, "users")
	requireExtension(t, dbURL, "vector")
	requireExtension(t, dbURL, "pgcrypto")
}

func TestDownMigrationDropsUsersAndKeepsSharedExtensions(t *testing.T) {
	dbURL := requireDatabaseURL(t)
	require.NoError(t, database.Migrate(context.Background(), dbURL, database.Up))
	require.NoError(t, database.Migrate(context.Background(), dbURL, database.Down))

	requireNoTable(t, dbURL, "users")
	requireExtension(t, dbURL, "vector")
	requireExtension(t, dbURL, "pgcrypto")
}

func requireDatabaseURL(t *testing.T) string {
	t.Helper()
	dbURL := os.Getenv("MEMORA_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("MEMORA_TEST_DATABASE_URL is not set")
	}
	return dbURL
}

func requireTable(t *testing.T, dbURL, name string) {
	t.Helper()
	db := openDB(t, dbURL)
	var exists bool
	require.NoError(t, db.QueryRowContext(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, name).Scan(&exists))
	require.True(t, exists, "table %q does not exist", name)
}

func requireNoTable(t *testing.T, dbURL, name string) {
	t.Helper()
	db := openDB(t, dbURL)
	var exists bool
	require.NoError(t, db.QueryRowContext(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, name).Scan(&exists))
	require.False(t, exists, "table %q still exists", name)
}

func requireExtension(t *testing.T, dbURL, name string) {
	t.Helper()
	db := openDB(t, dbURL)
	var exists bool
	require.NoError(t, db.QueryRowContext(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)", name).Scan(&exists))
	require.True(t, exists, "extension %q does not exist", name)
}

func openDB(t *testing.T, dbURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}
