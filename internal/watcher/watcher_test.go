package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"video-stream-processor/internal/config"

	"go.uber.org/zap"
)

func TestIsAllowedExt(t *testing.T) {
	if !isAllowedExt("foo/bar/test.mp4", []string{".mp4", ".mkv"}) {
		t.Error("Should allow .mp4")
	}
	if isAllowedExt("foo/bar/test.txt", []string{".mp4", ".mkv"}) {
		t.Error("Should not allow .txt")
	}
}

func TestFileHashConsistent(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	h1 := fileHash(fpath)
	h2 := fileHash(fpath)
	if h1 != h2 {
		t.Errorf("Hash should be consistent, got %s and %s", h1, h2)
	}
}

type mockWatcher struct {
	started *bool
}

func (m *mockWatcher) Start(ctx context.Context) {
	*m.started = true
}

func TestWatcherInterface_Mock(t *testing.T) {
	var started bool
	mw := &mockWatcher{started: &started}
	var w WatcherInterface = mw
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	if !started {
		t.Error("mockWatcher Start was not called")
	}
}

func TestScanExistingFiles(t *testing.T) {
	dir := t.TempDir()
	fileCh := make(chan string, 1)
	cfg := &config.Config{WatchDir: dir, StabilityThreshold: 1, VideoFileFormats: []string{".mp4"}}
	w := &Watcher{
		cfg:    cfg,
		log:    zap.NewNop(),
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		mu:     sync.Mutex{},
	}
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	w.scanExistingFiles()
	w.mu.Lock()
	_, ok := w.seen[fpath]
	w.mu.Unlock()
	if !ok {
		t.Error("scanExistingFiles did not add file to seen map")
	}
}

func TestRescanFiles(t *testing.T) {
	dir := t.TempDir()
	fileCh := make(chan string, 1)
	cfg := &config.Config{WatchDir: dir, StabilityThreshold: 1, VideoFileFormats: []string{".mp4"}}
	w := &Watcher{
		cfg:    cfg,
		log:    zap.NewNop(),
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		mu:     sync.Mutex{},
	}
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	w.rescanFiles()
	w.mu.Lock()
	_, ok := w.hashes[fpath]
	w.mu.Unlock()
	if !ok {
		t.Error("rescanFiles did not add file hash")
	}
}

// mockRedisStore implements redisstore.Store for testing filterFile logic
// Only implements methods needed for filterFile

type mockRedisStore struct {
	statusMap map[string]string
	hashMap   map[string]string
}

func (m *mockRedisStore) GetStreamStatus(ctx context.Context, streamID string) (string, error) {
	return m.statusMap[streamID], nil
}
func (m *mockRedisStore) GetValue(ctx context.Context, key string) (string, error) {
	return m.hashMap[key], nil
}

// Unused methods for this test
func (m *mockRedisStore) SetChunkUploaded(ctx context.Context, streamID string, chunkIdx int) error {
	return nil
}
func (m *mockRedisStore) IsChunkUploaded(ctx context.Context, streamID string, chunkIdx int) (bool, error) {
	return false, nil
}
func (m *mockRedisStore) SetStreamProgress(ctx context.Context, streamID string, chunkIdx int) error {
	return nil
}
func (m *mockRedisStore) GetStreamProgress(ctx context.Context, streamID string) (int, error) {
	return 0, nil
}
func (m *mockRedisStore) SetStreamStatus(ctx context.Context, streamID, status string) error {
	return nil
}
func (m *mockRedisStore) SetStreamTTL(ctx context.Context, streamID string, ttl time.Duration) error {
	return nil
}
func (m *mockRedisStore) ScanIncompleteStreams(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockRedisStore) SetValue(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (m *mockRedisStore) DeleteKey(ctx context.Context, key string) error { return nil }

func TestFilterFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	hash := fileHash(fpath)
	streamID := filepath.Base(fpath)
	hashKey := "file_hash:" + streamID

	cases := []struct {
		name     string
		status   string
		prevHash string
		expect   bool
	}{
		{"new file", "", "", true},
		{"changed file", "completed", "differenthash", true},
		{"already processed", "completed", hash, false},
		{"in progress", "in_progress", hash, true},
	}

	for _, c := range cases {
		store := &mockRedisStore{
			statusMap: map[string]string{streamID: c.status},
			hashMap:   map[string]string{hashKey: c.prevHash},
		}
		w := &Watcher{
			cfg:    &config.Config{VideoFileFormats: []string{".mp4"}},
			log:    zap.NewNop(),
			fileCh: nil,
			seen:   make(map[string]time.Time),
			hashes: make(map[string]string),
			mu:     sync.Mutex{},
			redis:  store,
		}
		if got := w.filterFile(fpath); got != c.expect {
			t.Errorf("%s: expected %v, got %v", c.name, c.expect, got)
		}
	}
}

// Update TestCheckStableFiles to use mockRedisStore and verify filtering
func TestCheckStableFiles_Filtered(t *testing.T) {
	dir := t.TempDir()
	fileCh := make(chan string, 1)
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	hash := fileHash(fpath)
	streamID := filepath.Base(fpath)
	hashKey := "file_hash:" + streamID
	store := &mockRedisStore{
		statusMap: map[string]string{streamID: "completed"},
		hashMap:   map[string]string{hashKey: hash},
	}
	w := &Watcher{
		cfg:    &config.Config{WatchDir: dir, StabilityThreshold: 1, VideoFileFormats: []string{".mp4"}},
		log:    zap.NewNop(),
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		mu:     sync.Mutex{},
		redis:  store,
	}
	now := time.Now().Add(-2 * time.Second)
	w.mu.Lock()
	w.seen[fpath] = now
	w.mu.Unlock()
	w.checkStableFiles(1 * time.Second)
	select {
	case <-fileCh:
		t.Error("Should not send already processed file to channel")
	case <-time.After(300 * time.Millisecond):
		// pass
	}
}

func TestCheckStableFiles_WithFilter(t *testing.T) {
	dir := t.TempDir()
	fileCh := make(chan string, 1)
	fpath := filepath.Join(dir, "test.mp4")
	os.WriteFile(fpath, []byte("somedata"), 0644)
	hash := fileHash(fpath)
	streamID := filepath.Base(fpath)
	hashKey := "file_hash:" + streamID

	// Case 1: Not completed, should send
	store1 := &mockRedisStore{
		statusMap: map[string]string{streamID: "in_progress"},
		hashMap:   map[string]string{hashKey: hash},
	}
	w1 := &Watcher{
		cfg:    &config.Config{WatchDir: dir, StabilityThreshold: 1, VideoFileFormats: []string{".mp4"}},
		log:    zap.NewNop(),
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		mu:     sync.Mutex{},
		redis:  store1,
	}
	now := time.Now().Add(-2 * time.Second)
	w1.mu.Lock()
	w1.seen[fpath] = now
	w1.mu.Unlock()
	w1.checkStableFiles(1 * time.Second)
	select {
	case file := <-fileCh:
		if file != fpath {
			t.Errorf("Expected %s, got %s", fpath, file)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for file from checkStableFiles (should send)")
	}

	// Case 2: completed and hash matches, should NOT send
	store2 := &mockRedisStore{
		statusMap: map[string]string{streamID: "completed"},
		hashMap:   map[string]string{hashKey: hash},
	}
	w2 := &Watcher{
		cfg:    &config.Config{WatchDir: dir, StabilityThreshold: 1, VideoFileFormats: []string{".mp4"}},
		log:    zap.NewNop(),
		fileCh: fileCh,
		seen:   make(map[string]time.Time),
		hashes: make(map[string]string),
		mu:     sync.Mutex{},
		redis:  store2,
	}
	w2.mu.Lock()
	w2.seen[fpath] = now
	w2.mu.Unlock()
	w2.checkStableFiles(1 * time.Second)
	select {
	case <-fileCh:
		t.Error("Should not send already processed file to channel (completed+hash match)")
	case <-time.After(300 * time.Millisecond):
		// pass
	}
}
