package main

import (
	"context"
	"fmt"
	"os"

	"github.com/1090-f/Memora/internal/platform/database"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: memora-migrate <up|down>")
		os.Exit(2)
	}
	databaseURL := os.Getenv("MEMORA_DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "MEMORA_DATABASE_URL is required")
		os.Exit(2)
	}
	if err := database.Migrate(context.Background(), databaseURL, database.Direction(os.Args[1])); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
