package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/1090-f/Memora/internal/app/worker"
	"github.com/1090-f/Memora/internal/platform/cache"
	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/1090-f/Memora/internal/platform/database"
	"github.com/1090-f/Memora/internal/platform/objectstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	redisClient, err := cache.Open(cfg.Redis)
	if err != nil {
		_ = sqlDB.Close()
		log.Fatal(err)
	}
	if _, err := objectstore.Open(ctx, cfg.MinIO); err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		log.Fatal(err)
	}

	app := worker.New(worker.Dependencies{
		Runners: []worker.Runner{},
		Closers: []io.Closer{sqlDB, redisClient},
	})
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
