// Package chunker provides logic for splitting files into binary chunks for upload and processing.
// Note: Chunks are not guaranteed to be independently playable video segments. For real-time streaming,
// use a tool like ffmpeg to split video into proper segments (e.g., HLS/DASH).
package chunker

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

type Chunk struct {
	Index     int
	Data      []byte
	Checksum  string
	Timestamp time.Time
}

type Chunker interface {
	ChunkFile(ctx context.Context, filePath string, chunkSize int) (<-chan Chunk, error)
}

type fileChunker struct{}

func New() Chunker {
	return &fileChunker{}
}

func (c *fileChunker) ChunkFile(ctx context.Context, filePath string, chunkSize int) (<-chan Chunk, error) {
	out := make(chan Chunk)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(out)
		defer f.Close()
		r := bufio.NewReaderSize(f, chunkSize)
		idx := 0
		for {
			buf := make([]byte, chunkSize)
			n, err := io.ReadFull(r, buf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if n > 0 {
					hash := sha256.Sum256(buf[:n])
					out <- Chunk{Index: idx, Data: buf[:n], Checksum: hex.EncodeToString(hash[:]), Timestamp: time.Now()}
				}
				break
			}
			if err != nil {
				break
			}
			hash := sha256.Sum256(buf)
			out <- Chunk{Index: idx, Data: buf, Checksum: hex.EncodeToString(hash[:]), Timestamp: time.Now()}
			idx++
		}
	}()
	return out, nil
}
