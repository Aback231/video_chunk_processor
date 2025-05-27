// Package redisstore provides Redis-backed checkpointing and progress tracking for resumable uploads.
// It is used to track chunk upload status, stream progress, and support recovery after interruptions.
package redisstore

import (
	"context"
	"fmt"
	"time"
	"video-stream-processor/internal/config"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type Store interface {
	SetChunkUploaded(ctx context.Context, streamID string, chunkIdx int) error
	IsChunkUploaded(ctx context.Context, streamID string, chunkIdx int) (bool, error)
	SetStreamProgress(ctx context.Context, streamID string, chunkIdx int) error
	GetStreamProgress(ctx context.Context, streamID string) (int, error)
	SetStreamStatus(ctx context.Context, streamID, status string) error
	GetStreamStatus(ctx context.Context, streamID string) (string, error)
	SetStreamTTL(ctx context.Context, streamID string, ttl time.Duration) error
	ScanIncompleteStreams(ctx context.Context) ([]string, error)
	// Generic key-value helpers for file hash/status logic
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key, value string, ttl time.Duration) error
	DeleteKey(ctx context.Context, key string) error
}

type RedisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type redisStore struct {
	client RedisClient
	log    *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) Store {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &redisStore{client: client, log: log}
}

func (r *redisStore) SetChunkUploaded(ctx context.Context, streamID string, chunkIdx int) error {
	key := "chunk_uploaded:" + streamID + ":" + itoa(chunkIdx)
	return r.client.Set(ctx, key, true, 0).Err()
}

func (r *redisStore) IsChunkUploaded(ctx context.Context, streamID string, chunkIdx int) (bool, error) {
	key := "chunk_uploaded:" + streamID + ":" + itoa(chunkIdx)
	res, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	return res == "1" || res == "true", err
}

func (r *redisStore) SetStreamProgress(ctx context.Context, streamID string, chunkIdx int) error {
	key := "stream_progress:" + streamID
	return r.client.Set(ctx, key, chunkIdx, 0).Err()
}

func (r *redisStore) GetStreamProgress(ctx context.Context, streamID string) (int, error) {
	key := "stream_progress:" + streamID
	return r.client.Get(ctx, key).Int()
}

func (r *redisStore) SetStreamStatus(ctx context.Context, streamID, status string) error {
	key := "stream_status:" + streamID
	return r.client.Set(ctx, key, status, 0).Err()
}

func (r *redisStore) GetStreamStatus(ctx context.Context, streamID string) (string, error) {
	key := "stream_status:" + streamID
	return r.client.Get(ctx, key).Result()
}

func (r *redisStore) SetStreamTTL(ctx context.Context, streamID string, ttl time.Duration) error {
	key := "stream_status:" + streamID
	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *redisStore) ScanIncompleteStreams(ctx context.Context) ([]string, error) {
	var streams []string
	iter := r.client.Scan(ctx, 0, "stream_status:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		status, err := r.client.Get(ctx, key).Result()
		if err == nil && status != "completed" {
			streams = append(streams, key[len("stream_status:"):])
		}
	}
	return streams, iter.Err()
}

func (r *redisStore) GetValue(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *redisStore) SetValue(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisStore) DeleteKey(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func itoa(i int) string {
	return fmt.Sprintf("%05d", i)
}
