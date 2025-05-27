# --- Build Stage ---
FROM golang:1.24.3-alpine AS builder

# Set working directory
WORKDIR /app

# Copy source code
COPY . .

# Install build dependencies
RUN apk add --no-cache git

# Download Go modules
RUN go mod download

# Build the Go binary (static build for Alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -o video-stream-processor ./cmd/main.go

USER nobody

# --- Final Stage ---
FROM alpine:3.21 AS final

# Set working directory
WORKDIR /app

# Copy built binary and config files from builder
COPY --from=builder /app/video-stream-processor .
COPY --from=builder /app/.env.example .env
COPY --from=builder /app/deploy/prometheus/prometheus.yml /etc/prometheus/prometheus.yml

# Create non-root user and set permissions
RUN adduser -D -g '' appuser && chown appuser:appuser /app
USER appuser

# Expose Prometheus metrics port
EXPOSE 2112

# Set entrypoint
ENTRYPOINT ["./video-stream-processor"]
