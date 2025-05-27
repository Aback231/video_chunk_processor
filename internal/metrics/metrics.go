// Package metrics defines and registers Prometheus metrics for the video stream processor.
// Metrics cover file detection, chunk uploads, failures, and Redis errors, and are exposed for monitoring.
package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	FilesDetected = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "vsp_files_detected_total",
			Help: "Total number of files detected for processing.",
		},
	)
	ChunksUploaded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "vsp_chunks_uploaded_total",
			Help: "Total number of chunks uploaded.",
		},
	)
	UploadFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "vsp_upload_failures_total",
			Help: "Total number of upload failures.",
		},
	)
	RedisErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "vsp_redis_errors_total",
			Help: "Total number of Redis errors.",
		},
	)
	FilesInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "vsp_files_in_progress",
			Help: "Current number of files being processed.",
		},
	)
	FileProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vsp_file_processing_duration_seconds",
			Help:    "Histogram of file processing durations.",
			Buckets: prometheus.ExponentialBuckets(1, 2, 8), // 1s, 2s, 4s, ...
		},
	)
	ChunkUploadDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vsp_chunk_upload_duration_seconds",
			Help:    "Histogram of chunk upload durations.",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 8), // 0.1s, 0.2s, ...
		},
	)
	LastFileProcessed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "vsp_last_file_processed_unixtime",
			Help: "Unix timestamp of the last successfully processed file.",
		},
	)
	initOnce sync.Once
)

func Init(port string) {
	initOnce.Do(func() {
		prometheus.MustRegister(FilesDetected, ChunksUploaded, UploadFailures, RedisErrors,
			FilesInProgress, FileProcessingDuration, ChunkUploadDuration, LastFileProcessed)
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(":"+port, nil)
		}()
	})
}
