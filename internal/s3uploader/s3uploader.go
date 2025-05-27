// Package s3uploader handles uploading video file chunks and metadata to S3/Minio-compatible object storage.
// Used by the main processor to persist video data and metadata for further processing or playback.
package s3uploader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"video-stream-processor/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// putObjecter defines the interface for PutObject used by s3Uploader (for mocking in tests)
type putObjecter interface {
	PutObject(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

type Uploader interface {
	UploadChunk(ctx context.Context, streamID string, chunkIdx int, data []byte) error
	UploadMetadata(ctx context.Context, streamID string, metadata []byte) error
}

type s3Uploader struct {
	client putObjecter
	bucket string
	log    *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) Uploader {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		log.Fatal("Failed to create minio client", zap.Error(err))
	}
	return &s3Uploader{client: client, bucket: cfg.MinioBucket, log: log}
}

func (s *s3Uploader) UploadChunk(ctx context.Context, streamID string, chunkIdx int, data []byte) error {
	objectName := streamID + "/chunk-" + itoa(chunkIdx)
	_, err := s.client.PutObject(ctx, s.bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return err
}

func (s *s3Uploader) UploadMetadata(ctx context.Context, streamID string, metadata []byte) error {
	objectName := streamID + "/metadata.json"
	_, err := s.client.PutObject(ctx, s.bucket, objectName, bytes.NewReader(metadata), int64(len(metadata)), minio.PutObjectOptions{ContentType: "application/json"})
	return err
}

func itoa(i int) string {
	return fmt.Sprintf("%05d", i)
}
