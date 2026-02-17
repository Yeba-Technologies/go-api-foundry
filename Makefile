.PHONY: run run-with-migrate migrate generate-domain build tidy docker-build docker-run dev dev-migrate test test-integration test-all test-ci

run:
	go run ./cmd/server

run-with-migrate:
	go run ./cmd/server --auto-migrate

dev:
	air

dev-migrate:
	air -- --auto-migrate

migrate:
	go run ./cmd/cli migrate

generate-domain:
	go run ./cmd/cli generate-domain

format:
	go fmt ./...

lint:
	go vet ./...

vendor:
	go mod vendor

## ---- Test targets ----

# Unit tests only — fast, no external dependencies (DB, Redis, queues).
test:
	go test -count=1 ./...

# Integration tests — requires Docker for testcontainers (Postgres, Redis).
test-integration:
	go test -tags=integration -count=1 -v ./integration/...

# All tests — unit + integration.
test-all:
	go test -tags=integration -count=1 -race ./...

# CI pipeline — all tests with race detection and coverage report.
test-ci:
	go test -tags=integration -count=1 -race -coverprofile=coverage.out ./...

## ---- End test targets ----

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy

docker-build:
	docker build -t go-api-foundry:dev .

docker-run:
	docker run --rm --env-file .env -p $${APP_PORT:-8080}:$${APP_PORT:-8080} go-api-foundry:dev
