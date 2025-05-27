package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"video-stream-processor/internal/app"
	"video-stream-processor/internal/config"
	"video-stream-processor/internal/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.LogLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("Shutting down...")
		cancel()
	}()

	app.Run(ctx, cfg, log)
}
