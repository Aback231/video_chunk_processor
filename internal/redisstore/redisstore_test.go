package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type mockRedisClient struct {
	setCalls []struct {
		key   string
		value any
	}
	getMap map[string]struct {
		val string
		err error
	}
	expireKeys []string
	delKeys    []string
	scanKeys   []string
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	m.setCalls = append(m.setCalls, struct {
		key   string
		value any
	}{key, value})
	return &redis.StatusCmd{}
}
func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	if v, ok := m.getMap[key]; ok {
		return redis.NewStringResult(v.val, v.err)
	}
	return redis.NewStringResult("", nil)
}
func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	m.expireKeys = append(m.expireKeys, key)
	return redis.NewBoolResult(true, nil)
}
func (m *mockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	cmd := redis.NewScanCmd(ctx, nil)
	cmd.SetVal(m.scanKeys, 0)
	cmd.SetErr(nil)
	return cmd
}
func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.delKeys = append(m.delKeys, keys...)
	return redis.NewIntResult(int64(len(keys)), nil)
}

func TestSetAndGetChunkUploaded(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	err := rs.SetChunkUploaded(context.Background(), "stream1", 1)
	if err != nil {
		t.Errorf("SetChunkUploaded failed: %v", err)
	}
	client.getMap["chunk_uploaded:stream1:00001"] = struct {
		val string
		err error
	}{val: "1", err: nil}
	ok, err := rs.IsChunkUploaded(context.Background(), "stream1", 1)
	if err != nil || !ok {
		t.Errorf("IsChunkUploaded should return true, got %v, %v", ok, err)
	}
	client.getMap["chunk_uploaded:stream1:00001"] = struct {
		val string
		err error
	}{val: "0", err: nil}
	ok, _ = rs.IsChunkUploaded(context.Background(), "stream1", 1)
	if ok {
		t.Error("IsChunkUploaded should return false for 0")
	}
}

func TestSetAndGetStreamProgress(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	err := rs.SetStreamProgress(context.Background(), "stream1", 5)
	if err != nil {
		t.Errorf("SetStreamProgress failed: %v", err)
	}
	client.getMap["stream_progress:stream1"] = struct {
		val string
		err error
	}{val: "42", err: nil}
	val, err := rs.GetStreamProgress(context.Background(), "stream1")
	if err != nil || val != 42 {
		t.Errorf("GetStreamProgress should return 42, got %v, %v", val, err)
	}
}

func TestSetAndGetStreamStatus(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	err := rs.SetStreamStatus(context.Background(), "stream1", "completed")
	if err != nil {
		t.Errorf("SetStreamStatus failed: %v", err)
	}
	client.getMap["stream_status:stream1"] = struct {
		val string
		err error
	}{val: "completed", err: nil}
	val, err := rs.GetStreamStatus(context.Background(), "stream1")
	if err != nil || val != "completed" {
		t.Errorf("GetStreamStatus should return completed, got %v, %v", val, err)
	}
}

func TestSetStreamTTL(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	err := rs.SetStreamTTL(context.Background(), "stream1", 10*time.Second)
	if err != nil {
		t.Errorf("SetStreamTTL failed: %v", err)
	}
}

func TestScanIncompleteStreams(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{
		"stream_status:foo": {val: "in_progress", err: nil},
		"stream_status:bar": {val: "completed", err: nil},
	}, scanKeys: []string{"stream_status:foo", "stream_status:bar"}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	streams, err := rs.ScanIncompleteStreams(context.Background())
	if err != nil {
		t.Errorf("ScanIncompleteStreams failed: %v", err)
	}
	if len(streams) != 1 || streams[0] != "foo" {
		t.Errorf("Expected only 'foo' as incomplete, got %v", streams)
	}
}

func TestGetSetDeleteValue(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	err := rs.SetValue(context.Background(), "k1", "v1", 0)
	if err != nil {
		t.Errorf("SetValue failed: %v", err)
	}
	client.getMap["k1"] = struct {
		val string
		err error
	}{val: "v1", err: nil}
	val, err := rs.GetValue(context.Background(), "k1")
	if err != nil || val != "v1" {
		t.Errorf("GetValue should return v1, got %v, %v", val, err)
	}
	err = rs.DeleteKey(context.Background(), "k1")
	if err != nil {
		t.Errorf("DeleteKey failed: %v", err)
	}
}

func TestIsChunkUploadedNil(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{
		"chunk_uploaded:stream1:00001": {val: "", err: redis.Nil},
	}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	ok, err := rs.IsChunkUploaded(context.Background(), "stream1", 1)
	if err != nil || ok {
		t.Errorf("IsChunkUploaded should return false for redis.Nil, got %v, %v", ok, err)
	}
}

func TestGetStreamProgressError(t *testing.T) {
	client := &mockRedisClient{getMap: map[string]struct {
		val string
		err error
	}{
		"stream_progress:stream1": {val: "", err: errors.New("fail")},
	}}
	rs := &redisStore{client: client, log: zap.NewNop()}
	_, err := rs.GetStreamProgress(context.Background(), "stream1")
	if err == nil {
		t.Error("GetStreamProgress should return error")
	}
}
