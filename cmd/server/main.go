package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/1090-f/Memora/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application := app.NewServer()
	if err := application.Initialize(ctx); err != nil {
		log.Fatal(err)
	}
	if err := application.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
