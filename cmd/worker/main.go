package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/1090-f/Memora/internal/app"
)

// main 启动 Memora 后台任务处理器，监听中断信号以实现优雅关闭。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application := app.NewWorker()
	if err := application.Initialize(ctx); err != nil {
		log.Fatalf("Worker 初始化失败: %v\n", err)
	}
	if err := application.Run(ctx); err != nil {
		log.Fatalf("Worker 启动失败: %v\n", err)
	}
}
