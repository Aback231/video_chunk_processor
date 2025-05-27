// This package contains the main application entrypoint and file processing logic for the video stream processor.
// It is responsible for orchestrating file monitoring, chunked uploads, checkpointing, and finalization.
//
// Key features:
//   - Asynchronous file monitoring with debounce
//   - Parallel file processing with sequential chunk uploads
//   - Redis-based checkpointing and resumability
//   - S3/Minio chunk and metadata uploads
//   - Prometheus metrics and structured logging
//   - Graceful shutdown and horizontal scalability
//
// Example usage:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	cfg := config.Load()
//	log := logger.New(cfg.LogLevel)
//	app.Run(ctx, cfg, log)
//
// This will start the processor, monitor the input directory, and process video files as they appear.
//
// For more details, see the README.md and architecture diagrams.
package app

import (
	"context"
	"sync"
	"time"
	"video-stream-processor/internal/config"
	"video-stream-processor/internal/metrics"
	"video-stream-processor/internal/redisstore"
	"video-stream-processor/internal/s3uploader"
	"video-stream-processor/internal/watcher"

	"go.uber.org/zap"
)

// Run starts the main event loop for the video stream processor.
// It initializes metrics, logging, Redis, S3, and the file watcher.
// It launches a configurable number of parallel workers to process files detected in the watched directory.
// Each file is processed in a separate goroutine, but chunk uploads for a single file are sequential to maintain order.
// The function blocks until a shutdown signal is received, then waits for all workers to finish.
func Run(ctx context.Context, cfg *config.Config, log *zap.Logger) {
	metrics.Init(cfg.PrometheusPort)
	log.Info("Starting video stream processor", zap.String("watch_dir", cfg.WatchDir))

	redisClient := redisstore.New(cfg, log)
	s3Client := s3uploader.New(cfg, log)

	var wg sync.WaitGroup
	fileCh := make(chan string, 100)

	// Pass redisClient to watcher.New
	var w watcher.WatcherInterface = watcher.New(cfg, log, fileCh, redisClient)
	go w.Start(ctx)

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 4 // fallback default
	}
	log.Info("Launching workers", zap.Int("worker_count", workerCount))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Info("Worker started", zap.Int("worker_id", workerID))
			for {
				select {
				case <-ctx.Done():
					log.Info("Worker shutting down", zap.Int("worker_id", workerID))
					return
				case file := <-fileCh:
					log.Info("Worker picked up file", zap.Int("worker_id", workerID), zap.String("file", file))
					metrics.FilesInProgress.Inc()
					start := time.Now()
					processFile(ctx, file, cfg, log, redisClient, s3Client)
					metrics.FilesInProgress.Dec()
					metrics.FileProcessingDuration.Observe(time.Since(start).Seconds())
				}
			}
		}(i)
	}

	<-ctx.Done()
	log.Info("Waiting for workers to finish...")
	wg.Wait()
	log.Info("All workers finished. Shutdown complete.")
}
