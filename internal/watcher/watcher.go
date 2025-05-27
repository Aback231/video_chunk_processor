// Package watcher provides a file system watcher that detects new and changed video files for processing.
// It supports hash-based change detection, configurable video file formats, and is designed for testability.
// Only files with extensions specified in the config are processed.
//
// The watcher is used by the main application to trigger chunked uploads and processing workflows.
//
// For details, see README.md and architecture diagrams.
package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"video-stream-processor/internal/config"
	"video-stream-processor/internal/metrics"
	"video-stream-processor/internal/redisstore"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// WatcherInterface defines the contract for a file watcher.
type WatcherInterface interface {
	Start(ctx context.Context)
}

type Watcher struct {
	cfg    *config.Config
	log    *zap.Logger
	fileCh chan<- string
	seen   map[string]time.Time
	hashes map[string]string // file path -> last known hash
	mu     sync.Mutex
	redis  redisstore.Store // Add redis client to watcher
}

// New returns a new Watcher that implements WatcherInterface.
func New(cfg *config.Config, log *zap.Logger, fileCh chan<- string, redis redisstore.Store) WatcherInterface {
	return &Watcher{
		cfg:    cfg,
		log:    log,
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		redis:  redis,
	}
}

func (w *Watcher) Start(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.log.Fatal("Failed to create watcher", zap.Error(err))
	}
	defer watcher.Close()

	if err := watcher.Add(w.cfg.WatchDir); err != nil {
		w.log.Fatal("Failed to watch dir", zap.String("dir", w.cfg.WatchDir), zap.Error(err))
	}

	// Scan for existing files on startup
	w.scanExistingFiles()

	debounce := time.Duration(w.cfg.StabilityThreshold) * time.Second

	// Start a goroutine to periodically rescan the directory for new files
	go w.periodicRescan(ctx, debounce)

	// Start a goroutine to periodically check for stable files
	go func() {
		ticker := time.NewTicker(time.Second) // Check every second for stable files
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.checkStableFiles(debounce)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Chmod) != 0 {
				w.mu.Lock()
				w.seen[event.Name] = time.Now()
				w.mu.Unlock()
			}
		case err := <-watcher.Errors:
			w.log.Error("Watcher error", zap.Error(err))
		}
		// No longer call checkStableFiles here; handled by ticker goroutine
	}
}

// scanExistingFiles scans the watch directory for video files on startup and adds them to the seen map.
func (w *Watcher) scanExistingFiles() {
	files, err := filepath.Glob(filepath.Join(w.cfg.WatchDir, "*"))
	if err != nil {
		w.log.Error("Failed to scan watch dir", zap.Error(err))
		return
	}
	now := time.Now()
	for _, file := range files {
		if isAllowedExt(file, w.cfg.VideoFileFormats) {
			w.mu.Lock()
			w.seen[file] = now.Add(-2 * time.Duration(w.cfg.StabilityThreshold) * time.Second) // Mark as old enough
			w.mu.Unlock()
		}
	}
}

// isAllowedExt checks if the file extension is in the allowed list.
func isAllowedExt(file string, allowedExts []string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	for _, allowed := range allowedExts {
		if ext == allowed {
			return true
		}
	}
	return false
}

// filterFile checks Redis for the file's hash and status. Returns true if the file is new or changed.
func (w *Watcher) filterFile(file string) bool {
	hash := fileHash(file)
	streamID := filepath.Base(file)
	hashKey := "file_hash:" + streamID
	status, _ := w.redis.GetStreamStatus(context.Background(), streamID)
	prevHash, _ := w.redis.GetValue(context.Background(), hashKey)
	if prevHash == hash && status == "completed" {
		w.log.Info("Watcher: file already processed and hash unchanged, skipping", zap.String("file", file), zap.String("stream_id", streamID))
		return false
	}
	return true
}

func (w *Watcher) checkStableFiles(debounce time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	for file, last := range w.seen {
		if now.Sub(last) > debounce {
			ext := strings.ToLower(filepath.Ext(file))
			for _, allowed := range w.cfg.VideoFileFormats {
				if ext == allowed {
					if w.filterFile(file) {
						w.fileCh <- file
						metrics.FilesDetected.Inc()
					}
					break
				}
			}
			delete(w.seen, file)
		}
	}
}

// periodicRescan periodically scans the directory for new or changed files.
func (w *Watcher) periodicRescan(ctx context.Context, debounce time.Duration) {
	ticker := time.NewTicker(debounce)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.rescanFiles()
		}
	}
}

// rescanFiles checks for new files and for file content changes (by hash).
func (w *Watcher) rescanFiles() {
	files, err := filepath.Glob(filepath.Join(w.cfg.WatchDir, "*"))
	if err != nil {
		w.log.Error("Failed to rescan watch dir", zap.Error(err))
		return
	}
	now := time.Now()
	for _, file := range files {
		if isAllowedExt(file, w.cfg.VideoFileFormats) {
			hash := fileHash(file)
			w.mu.Lock()
			prevHash, seen := w.hashes[file]
			if !seen || prevHash != hash {
				w.seen[file] = now.Add(-2 * time.Duration(w.cfg.StabilityThreshold) * time.Second)
				w.hashes[file] = hash
			}
			w.mu.Unlock()
		}
	}
}

// fileHash returns a short hash of the file contents (SHA256 hex, first 16 chars).
func fileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.CopyN(h, f, 10*1024*1024) // Only hash first 10MB for speed
	return hex.EncodeToString(h.Sum(nil))[:16]
}
