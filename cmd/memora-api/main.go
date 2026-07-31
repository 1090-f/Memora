package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/1090-f/Memora/internal/app/api"
	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/1090-f/Memora/internal/platform/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	app, err := api.New(api.Dependencies{
		HTTP:           cfg.HTTP,
		DatabaseHealth: func(ctx context.Context) error { return database.Check(ctx, db) },
		RedisHealth:    unavailableDependency,
		MinIOHealth:    unavailableDependency,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func unavailableDependency(context.Context) error {
	return errors.New("dependency adapter is not configured")
}
