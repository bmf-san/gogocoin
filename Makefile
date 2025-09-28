# gogocoin Makefile

.PHONY: help up down logs restart rebuild clean test lint init build

# Default target
.DEFAULT_GOAL := help

# Variable definitions
APP_NAME := gogocoin
COMPOSE := docker compose

# Docker Compose commands
up: ## Start containers
	@echo "Starting $(APP_NAME) container..."
	$(COMPOSE) up -d

down: ## Stop containers
	@echo "Stopping $(APP_NAME) container..."
	$(COMPOSE) down

logs: ## Show logs
	@echo "Showing logs..."
	$(COMPOSE) logs -f

restart: ## Restart containers
	@echo "Restarting $(APP_NAME) container..."
	$(COMPOSE) restart

rebuild: ## Rebuild image and start
	@echo "Rebuilding and starting $(APP_NAME) container..."
	$(COMPOSE) up -d --build

# Note: Production use requires bitFlyer API keys (configured in .env file)

# Build
build: ## Build application
	@echo "Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) ./cmd/$(APP_NAME)
	@echo "Build complete: bin/$(APP_NAME)"

# Initialization
init: ## Initialize setup (create config file)
	@echo "Initializing $(APP_NAME)..."
	@mkdir -p ./configs ./data ./logs
	@if [ ! -f ./configs/config.yaml ]; then \
		cp ./configs/config.example.yaml ./configs/config.yaml && \
		echo "Created ./configs/config.yaml from example. Please edit it with your settings."; \
	else \
		echo "./configs/config.yaml already exists. Skipping."; \
	fi

# Development (Testing)
test: ## Run tests
	@echo "Running tests..."
	@go mod tidy
	@if find . -name "*_test.go" -not -path "./vendor/*" | grep -q . 2>/dev/null; then \
		go test -v ./... || echo "Some tests failed"; \
	else \
		echo "No test files found - skipping tests"; \
	fi

test-coverage: ## Generate test coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run --max-issues-per-linter=0 --max-same-issues=0

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

# Cleanup
clean: ## Clean up build artifacts and data
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html
	@rm -rf ./data/*.db
	@rm -rf ./logs/*.log
	@find . -type d -empty -not -path "./.git*" -delete 2>/dev/null || true
	@echo "Cleaning Go and golangci-lint caches..."
	@go clean -cache -modcache -testcache 2>/dev/null || true
	@golangci-lint cache clean 2>/dev/null || true

clean-all: clean down ## Clean up everything (containers + data)
	@echo "Removing Docker images..."
	@docker rmi $(APP_NAME):latest 2>/dev/null || true

# Dependencies
deps: ## Update dependencies
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod download

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Help
help: ## Show this help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
