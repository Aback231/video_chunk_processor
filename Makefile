# Makefile for video-stream-processor

.PHONY: build run test up down clean coverage restart

build:
	go build -o bin/video-stream-processor ./cmd/main.go

run:
	go run ./cmd/main.go

test:
	go test -v -cover ./internal/...

up:
	docker-compose up --build -d

down:
	docker-compose down

clean:
	rm -rf bin/

coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

restart:
	docker-compose down
	docker-compose up --build -d --force-recreate
