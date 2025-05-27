// Package config loads and manages application configuration from .env files and environment variables.
// All key parameters, including supported video file formats, are configurable via .env.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	MinioEndpoint      string
	MinioAccessKey     string
	MinioSecretKey     string
	MinioBucket        string
	MinioUseSSL        bool
	WatchDir           string
	ChunkSize          int
	StabilityThreshold int
	StreamTimeout      int
	PrometheusPort     string
	LogLevel           string
	WorkerCount        int      // Number of parallel file processing workers
	VideoFileFormats   []string // Supported video file formats
}

func Load() *Config {
	_ = godotenv.Load()
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	chunkSize, _ := strconv.Atoi(getEnv("CHUNK_SIZE", "5242880"))
	// Increased default stability threshold for more reliable detection
	stabilityThreshold, _ := strconv.Atoi(getEnv("STABILITY_THRESHOLD", "15"))
	streamTimeout, _ := strconv.Atoi(getEnv("STREAM_TIMEOUT", "30"))
	minioUseSSL := getEnv("MINIO_USE_SSL", "false") == "true"
	workerCount, _ := strconv.Atoi(getEnv("WORKER_COUNT", "4"))
	formats := getEnv("VIDEO_FILE_FORMATS", ".mp4,.mkv")
	var videoFileFormats []string
	for _, f := range strings.Split(formats, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			if !strings.HasPrefix(f, ".") {
				f = "." + f
			}
			videoFileFormats = append(videoFileFormats, strings.ToLower(f))
		}
	}
	return &Config{
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            redisDB,
		MinioEndpoint:      getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:     getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:        getEnv("MINIO_BUCKET", "video-streams"),
		MinioUseSSL:        minioUseSSL,
		WatchDir:           getEnv("WATCH_DIR", "./input_files"),
		ChunkSize:          chunkSize,
		StabilityThreshold: stabilityThreshold,
		StreamTimeout:      streamTimeout,
		PrometheusPort:     getEnv("PROMETHEUS_PORT", "2112"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		WorkerCount:        workerCount,
		VideoFileFormats:   videoFileFormats,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
