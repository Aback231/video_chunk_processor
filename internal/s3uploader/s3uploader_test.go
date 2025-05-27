package s3uploader

import (
	"context"
	"errors"
	"io"
	"testing"
	"video-stream-processor/internal/config"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

type mockMinioClient struct {
	putErr    error
	putCalled []struct {
		bucket     string
		objectName string
		data       []byte
		opts       minio.PutObjectOptions
	}
}

func (m *mockMinioClient) PutObject(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	buf := make([]byte, size)
	reader.Read(buf)
	m.putCalled = append(m.putCalled, struct {
		bucket     string
		objectName string
		data       []byte
		opts       minio.PutObjectOptions
	}{bucket, objectName, buf, opts})
	return minio.UploadInfo{}, m.putErr
}

func TestUploadChunk_Success(t *testing.T) {
	mc := &mockMinioClient{}
	s := &s3Uploader{client: mc, bucket: "testbucket", log: zap.NewNop()}
	err := s.UploadChunk(context.Background(), "stream1", 2, []byte("chunkdata"))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(mc.putCalled) != 1 {
		t.Fatal("PutObject not called")
	}
	call := mc.putCalled[0]
	if call.bucket != "testbucket" || call.objectName != "stream1/chunk-00002" {
		t.Errorf("unexpected bucket/objectName: %v/%v", call.bucket, call.objectName)
	}
	if string(call.data) != "chunkdata" {
		t.Errorf("unexpected data: %s", string(call.data))
	}
}

func TestUploadChunk_Error(t *testing.T) {
	mc := &mockMinioClient{putErr: errors.New("fail")}
	s := &s3Uploader{client: mc, bucket: "b", log: zap.NewNop()}
	err := s.UploadChunk(context.Background(), "id", 1, []byte("d"))
	if err == nil || err.Error() != "fail" {
		t.Errorf("expected fail error, got %v", err)
	}
}

func TestUploadMetadata_Success(t *testing.T) {
	mc := &mockMinioClient{}
	s := &s3Uploader{client: mc, bucket: "b", log: zap.NewNop()}
	meta := []byte(`{"foo":1}`)
	err := s.UploadMetadata(context.Background(), "id", meta)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(mc.putCalled) != 1 {
		t.Fatal("PutObject not called")
	}
	call := mc.putCalled[0]
	if call.objectName != "id/metadata.json" {
		t.Errorf("unexpected objectName: %v", call.objectName)
	}
	if string(call.data) != string(meta) {
		t.Errorf("unexpected metadata: %s", string(call.data))
	}
	if call.opts.ContentType != "application/json" {
		t.Errorf("expected ContentType application/json, got %v", call.opts.ContentType)
	}
}

func TestUploadMetadata_Error(t *testing.T) {
	mc := &mockMinioClient{putErr: errors.New("failmeta")}
	s := &s3Uploader{client: mc, bucket: "b", log: zap.NewNop()}
	err := s.UploadMetadata(context.Background(), "id", []byte("{}"))
	if err == nil || err.Error() != "failmeta" {
		t.Errorf("expected failmeta error, got %v", err)
	}
}

func TestNew_Success(t *testing.T) {
	cfg := &config.Config{
		MinioEndpoint:  "localhost:9000",
		MinioAccessKey: "key",
		MinioSecretKey: "secret",
		MinioUseSSL:    false,
		MinioBucket:    "bucket",
	}
	log := zap.NewNop()
	u := New(cfg, log)
	if u == nil {
		t.Error("New should not return nil")
	}
}
