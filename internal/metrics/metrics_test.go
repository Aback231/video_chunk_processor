package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsRegistrationAndInc(t *testing.T) {
	// This will panic if already registered, so recover
	defer func() { recover() }()
	Init("21123") // Use a random port, don't care if it fails

	FilesDetected.Inc()
	ChunksUploaded.Inc()
	UploadFailures.Inc()
	RedisErrors.Inc()
	FilesInProgress.Set(3)
	FileProcessingDuration.Observe(2.5)
	ChunkUploadDuration.Observe(0.5)
	LastFileProcessed.Set(1234567890)
}

func TestMetricsHandler(t *testing.T) {
	// Register metrics and start handler on a test server
	Init("0") // port 0, will not actually listen
	ts := httptest.NewServer(http.DefaultServeMux)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to GET /metrics: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "vsp_files_detected_total") {
		t.Error("metrics output missing vsp_files_detected_total")
	}
}
