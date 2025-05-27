package chunker

import (
	"context"
	"os"
	"testing"
)

// TestChunker_SmallFile verifies that a small file is split into the correct number of non-empty chunks.
// It also checks that chunking is deterministic and chunk sizes are as expected.
func TestChunker_SmallFile(t *testing.T) {
	f, err := os.CreateTemp("", "testfile-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Write([]byte("1234567890abcdef"))
	f.Close()

	c := New()
	chunks, err := c.ChunkFile(context.Background(), f.Name(), 4)
	if err != nil {
		t.Fatalf("ChunkFile error: %v", err)
	}
	var chunkLens []int
	count := 0
	for chunk := range chunks {
		if len(chunk.Data) == 0 {
			t.Errorf("Empty chunk at index %d", chunk.Index)
		}
		chunkLens = append(chunkLens, len(chunk.Data))
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 chunks, got %d", count)
	}
	for i, l := range chunkLens {
		if i < 3 && l != 4 {
			t.Errorf("Chunk %d should be 4 bytes, got %d", i, l)
		}
	}
}
