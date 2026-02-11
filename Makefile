.PHONY: run run-with-migrate migrate generate-domain build tidy docker-build docker-run dev dev-migrate

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

test:
	go test ./...

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy

docker-build:
	docker build -t go-api-foundry:dev .

docker-run:
	docker run --rm --env-file .env -p $${APP_PORT:-8080}:$${APP_PORT:-8080} go-api-foundry:dev
