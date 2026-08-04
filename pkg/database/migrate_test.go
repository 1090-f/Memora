package database

import "testing"

func TestDatabaseBootstrapTargetUsesPostgresMaintenanceDatabase(t *testing.T) {
	maintenanceURL, databaseName, err := databaseBootstrapTarget("postgres://memora:p%40ss@localhost:5432/memora_app?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if databaseName != "memora_app" {
		t.Fatalf("database name = %q, want %q", databaseName, "memora_app")
	}
	wantURL := "postgres://memora:p%40ss@localhost:5432/postgres?sslmode=disable"
	if maintenanceURL != wantURL {
		t.Fatalf("maintenance URL = %q, want %q", maintenanceURL, wantURL)
	}
}

func TestDatabaseBootstrapTargetRequiresDatabaseName(t *testing.T) {
	if _, _, err := databaseBootstrapTarget("postgres://memora:secret@localhost:5432"); err == nil {
		t.Fatal("expected missing database name to be rejected")
	}
}

func TestQuotePostgresIdentifierEscapesQuotes(t *testing.T) {
	if got, want := quotePostgresIdentifier(`memora"prod`), `"memora""prod"`; got != want {
		t.Fatalf("quoted identifier = %q, want %q", got, want)
	}
}
