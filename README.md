# Video Stream Processor

A production-ready, horizontally scalable video stream file processor in Go. Features async file monitoring, chunked uploads, Redis checkpointing, S3/Minio storage, Prometheus metrics, robust logging, and full Docker orchestration.

## Features

- Async file monitoring (fsnotify, debounced, hash-based change detection)
- Configurable video file formats/extensions via `.env`
- Chunked, resumable uploads (with Redis checkpointing)
- S3/Minio storage
- Prometheus metrics
- Uber zap structured logging
- Dockerized, horizontally scalable
- Test coverage: 65.3% (see coverage report for details)
- All configuration via `.env`

## Architecture

![Architecture Diagram](deploy/architecture.png)

## Usage

### 1. Configuration

- Copy `.env.example` to `.env` and edit as needed (S3/Minio, Redis, file formats, etc).
- Video file formats/extensions are set in `.env` (e.g., `VIDEO_FILE_EXTENSIONS=mp4,mkv`).

### 2. Build & Start

```sh
make build           # Build the Go app
make up              # Start all services (app, Redis, Minio, Prometheus)
```

### 3. Add Video Files

- Place video files in the `input_files/` directory. Only files with configured extensions will be processed.
- Example video files are in `example_files/` (see below).

### 4. Monitor & Operate

- Logs: `docker-compose logs -f`
- Prometheus: [http://localhost:9090](http://localhost:9090)
- Alerts: Prometheus alert rules in `deploy/prometheus/alerts.yml`

### 5. Stopping

```sh
make down            # Stop all services
```

## Directory Structure

- `/cmd` - Entrypoint
- `/internal` - All business logic (watcher, chunker, redisstore, s3uploader, etc.)
- `/input_files` - Monitored directory for new video files
- `/example_files` - Example/test video files (see below)
- `/deploy` - Monitoring configs (Prometheus, alerts)

## Example Logs

Below are example logs showing the detection and processing of video files:

```
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9009407,"caller":"app/app.go:45","msg":"Starting video stream processor","watch_dir":"/input_files"}
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9011366,"caller":"app/app.go:61","msg":"Launching workers","worker_count":4}
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9011889,"caller":"app/app.go:66","msg":"Worker started","worker_id":3}
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9012225,"caller":"app/app.go:66","msg":"Worker started","worker_id":0}
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9013448,"caller":"app/app.go:66","msg":"Worker started","worker_id":1}
2025-05-27 18:23:25 {"level":"info","ts":1748363005.9013598,"caller":"app/app.go:66","msg":"Worker started","worker_id":2}
2025-05-27 18:23:56 {"level":"info","ts":1748363036.9193096,"caller":"app/app.go:73","msg":"Worker picked up file","worker_id":3,"file":"/input_files/output_red_10hr.mp4"}
2025-05-27 18:23:56 {"level":"info","ts":1748363036.9193408,"caller":"app/process_file.go:56","msg":"Processing file","file":"/input_files/output_red_10hr.mp4","stream_id":"output_red_10hr.mp4"}
2025-05-27 18:23:56 {"level":"info","ts":1748363036.9378796,"caller":"app/app.go:73","msg":"Worker picked up file","worker_id":0,"file":"/input_files/output_blue_10hr.mp4"}
2025-05-27 18:23:56 {"level":"info","ts":1748363036.937912,"caller":"app/process_file.go:56","msg":"Processing file","file":"/input_files/output_blue_10hr.mp4","stream_id":"output_blue_10hr.mp4"}
2025-05-27 18:23:57 {"level":"info","ts":1748363037.2336743,"caller":"app/process_file.go:133","msg":"File processing complete","file":"/input_files/output_red_10hr.mp4","stream_id":"output_red_10hr.mp4"}
2025-05-27 18:23:57 {"level":"info","ts":1748363037.2618773,"caller":"app/process_file.go:133","msg":"File processing complete","file":"/input_files/output_blue_10hr.mp4","stream_id":"output_blue_10hr.mp4"}
2025-05-27 18:26:03 {"level":"info","ts":1748363163.9204466,"caller":"watcher/watcher.go:141","msg":"Watcher: file already processed and hash unchanged, skipping","file":"/input_files/output_blue_10hr.mp4","stream_id":"output_blue_10hr.mp4"}
2025-05-27 18:26:03 {"level":"info","ts":1748363163.930426,"caller":"watcher/watcher.go:141","msg":"Watcher: file already processed and hash unchanged, skipping","file":"/input_files/output_red_10hr.mp4","stream_id":"output_red_10hr.mp4"}
```

## Screenshot: Chunk Uploads

Below is a screenshot showing chunk uploads in action:

![Chunk Uploads Screenshot](Screenshot%202025-05-27%20at%2018.28.17.png)

## Example Video Files

- The `example_files/` directory contains sample video files for testing.
- These files were generated using [ffmpeg](https://ffmpeg.org/). Example command:
  ```sh
  ffmpeg -f lavfi -i color=c=red:s=1920x1080:d=36000 -c:v libx264 -preset veryfast -crf 23 -pix_fmt yuv420p output_red_10hr.mp4
  ```
- You can generate your own test files with `ffmpeg` as needed.

## Note on Playable/Streamable Chunks

- This processor splits files into binary chunks for upload.
- **If you need real-time streaming with playable video chunks (e.g., HLS, DASH), you must use a tool like `ffmpeg` to split video into proper segments (GOP-aligned, with correct headers) so each chunk is independently playable.**
- The current chunker is not suitable for direct streaming playback.

## Testing

- `make test` for unit tests
- Test coverage is currently 65.3% (see coverage report for details)

## License

MIT
