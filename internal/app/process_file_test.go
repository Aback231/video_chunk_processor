package app

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"video-stream-processor/internal/config"
	"video-stream-processor/internal/redisstore"
	"video-stream-processor/internal/s3uploader"

	"go.uber.org/zap"
)

type mockRedis struct {
	redisstore.Store
	chunkUploaded map[int]bool
	progress      int
	status        string
	hash          string
	calls         map[string]int
	failIsChunk   bool // add this flag
	failSetChunk  bool // add this flag
}

func (m *mockRedis) IsChunkUploaded(ctx context.Context, streamID string, chunkIdx int) (bool, error) {
	m.calls["IsChunkUploaded"]++
	if m.failIsChunk {
		return false, errors.New("redis error")
	}
	return m.chunkUploaded[chunkIdx], nil
}
func (m *mockRedis) SetChunkUploaded(ctx context.Context, streamID string, chunkIdx int) error {
	m.calls["SetChunkUploaded"]++
	if m.failSetChunk {
		return errors.New("fail set chunk uploaded")
	}
	m.chunkUploaded[chunkIdx] = true
	return nil
}
func (m *mockRedis) SetStreamProgress(ctx context.Context, streamID string, chunkIdx int) error {
	m.calls["SetStreamProgress"]++
	m.progress = chunkIdx
	return nil
}
func (m *mockRedis) GetValue(ctx context.Context, key string) (string, error) {
	m.calls["GetValue"]++
	return m.hash, nil
}
func (m *mockRedis) SetValue(ctx context.Context, key, value string, ttl time.Duration) error {
	m.calls["SetValue"]++
	m.hash = value
	return nil
}
func (m *mockRedis) GetStreamStatus(ctx context.Context, streamID string) (string, error) {
	m.calls["GetStreamStatus"]++
	return m.status, nil
}
func (m *mockRedis) SetStreamStatus(ctx context.Context, streamID, status string) error {
	m.calls["SetStreamStatus"]++
	m.status = status
	return nil
}
func (m *mockRedis) SetStreamTTL(ctx context.Context, streamID string, ttl time.Duration) error {
	m.calls["SetStreamTTL"]++
	return nil
}
func (m *mockRedis) DeleteKey(ctx context.Context, key string) error {
	m.calls["DeleteKey"]++
	return nil
}

// Only used for error simulation
func (m *mockRedis) IsChunkUploadedErr(ctx context.Context, streamID string, chunkIdx int) (bool, error) {
	return false, errors.New("redis error")
}

// ---
type mockS3 struct {
	s3uploader.Uploader
	failChunk bool
	failMeta  bool
	calls     map[string]int
}

func (m *mockS3) UploadChunk(ctx context.Context, streamID string, chunkIdx int, data []byte) error {
	m.calls["UploadChunk"]++
	if m.failChunk {
		return errors.New("fail chunk")
	}
	return nil
}
func (m *mockS3) UploadMetadata(ctx context.Context, streamID string, metadata []byte) error {
	m.calls["UploadMetadata"]++
	if m.failMeta {
		return errors.New("fail meta")
	}
	return nil
}

func TestProcessFile_Success(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
	if redis.status != "completed" {
		t.Error("status not set to completed")
	}
	if s3.calls["UploadChunk"] == 0 {
		t.Error("UploadChunk not called")
	}
	if s3.calls["UploadMetadata"] == 0 {
		t.Error("UploadMetadata not called")
	}
}

func TestProcessFile_AlreadyProcessed(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, status: "completed", calls: map[string]int{}}
	redis.hash = fileHash(f)
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
	if s3.calls["UploadChunk"] > 0 {
		t.Error("should not upload chunk if already processed")
	}
}

func TestProcessFile_ChunkingError(t *testing.T) {
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), "/notfound.mp4", cfg, log, redis, s3)
	// Should not panic or call s3
	if s3.calls["UploadChunk"] > 0 {
		t.Error("should not upload chunk on chunking error")
	}
}

func TestProcessFile_ChunkUploadError(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}}
	s3 := &mockS3{failChunk: true, calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
	if s3.calls["UploadChunk"] == 0 {
		t.Error("UploadChunk should be called even if it fails")
	}
}

func TestProcessFile_RedisError(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}, failIsChunk: true}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
}

func TestProcessFile_MetadataUploadError(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}}
	s3 := &mockS3{failMeta: true, calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
}

func TestProcessFile_HashChanged(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}, hash: "oldhash"}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
	if redis.calls["DeleteKey"] == 0 {
		t.Error("DeleteKey should be called if hash changed")
	}
}

func TestProcessFile_FailedHash(t *testing.T) {
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), "/doesnotexist.mp4", cfg, log, redis, s3)
	// Should not panic or call s3
	if s3.calls["UploadChunk"] > 0 {
		t.Error("should not upload chunk if file hash fails")
	}
}

func TestProcessFile_ChunkSetChunkUploadedError(t *testing.T) {
	dir := t.TempDir()
	f := dir + "/test.mp4"
	os.WriteFile(f, []byte("somedata"), 0644)
	cfg := &config.Config{ChunkSize: 4}
	redis := &mockRedis{chunkUploaded: map[int]bool{}, calls: map[string]int{}, failSetChunk: true}
	s3 := &mockS3{calls: map[string]int{}}
	log := zap.NewNop()
	processFile(context.Background(), f, cfg, log, redis, s3)
}
