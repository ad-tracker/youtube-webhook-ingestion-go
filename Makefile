.PHONY: help build run test clean docker-build docker-up docker-down migrate

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	go build -o server cmd/server/main.go

run: ## Run the application
	go run cmd/server/main.go

test: ## Run tests
	go test ./... -v

test-coverage: ## Run tests with coverage
	go test ./... -coverprofile=coverage.txt -covermode=atomic
	go tool cover -html=coverage.txt -o coverage.html

clean: ## Clean build artifacts
	rm -f server coverage.txt coverage.html

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run

tidy: ## Tidy go modules
	go mod tidy

docker-build: ## Build Docker image
	docker-compose build

docker-up: ## Start all services with Docker Compose
	docker-compose up -d

docker-down: ## Stop all services
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

migrate: ## Run database migrations
	psql -h localhost -U postgres -d adtracker -f migrations/001_init_schema.sql

.DEFAULT_GOAL := help
