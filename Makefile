.PHONY: build run test docker-up docker-down migrate lint clean help

BINARY_NAME := idempot-api

build:
	go build -o bin/$(BINARY_NAME) ./cmd/api

run:
	go run ./cmd/api/main.go

run-with-config:
	CONFIG_FILE=internal/config/config.yaml go run ./cmd/api/main.go

run-production:
	CONFIG_FILE=internal/config/config.production.yaml go run ./cmd/api/main.go

test:
	go test -v ./...

test-race:
	go test -race -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build:
	docker-compose build

migrate:
	docker-compose exec -T postgres psql -U postgres -d idempot < internal/repository/migration/init.sql

lint:
	golangci-lint run

clean:
	rm -rf bin/
	docker-compose down -v

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build           - Build the application"
	@echo "  make run             - Run the application with default config"
	@echo "  make run-with-config - Run with specific config file"
	@echo "  make run-production  - Run in production mode"
	@echo "  make test            - Run tests"
	@echo "  make test-race       - Run tests with race detector"
	@echo "  make test-cover      - Run tests with coverage"
	@echo "  make docker-up       - Start docker containers"
	@echo "  make docker-down     - Stop docker containers"
	@echo "  make docker-logs     - View docker logs"
	@echo "  make docker-build    - Build docker images"
	@echo "  make migrate         - Run database migrations manually"
	@echo "  make lint            - Run linter"
	@echo "  make clean           - Clean build artifacts and volumes"
