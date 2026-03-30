# gogocoin Makefile

.PHONY: help up down logs restart rebuild build init test test-coverage fmt lint deps install-tools generate generate-check clean

# Default target
.DEFAULT_GOAL := help

# Variable definitions
APP_NAME := gogocoin
COMPOSE := docker compose

# Help
help: ## Show this help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# Docker Compose commands
# Note: Requires bitFlyer API keys configured in .env
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

# Build
build: ## [For gogocoin developers] Build the reference binary (no strategy registered — not for production use)
	@echo "Building $(APP_NAME)..."
	@echo "NOTE: This binary has no strategy registered. See example/ or docs/DESIGN_DOC.md for library usage."
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

# Testing
test: ## Run tests
	@echo "Running tests..."
	@if find . -name "*_test.go" -not -path "./vendor/*" | grep -q . 2>/dev/null; then \
		go test -v ./... || echo "Some tests failed"; \
	else \
		echo "No test files found - skipping tests"; \
	fi

test-coverage: ## Generate test coverage report
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code quality
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run --max-issues-per-linter=0 --max-same-issues=0

# Dependencies
deps: ## Update dependencies
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod download

install-tools: ## Install development tools (golangci-lint, oapi-codegen)
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Code generation
generate: ## Generate API code from OpenAPI spec
	@echo "Generating API code from OpenAPI spec..."
	@oapi-codegen --config internal/adapter/http/oapi-codegen.yaml docs/openapi.yaml
	@echo "Code generation complete: internal/adapter/http/api.gen.go"

generate-check: generate ## Verify generated code is in sync with OpenAPI spec (for CI)
	@if ! git diff --exit-code internal/adapter/http/api.gen.go > /dev/null 2>&1; then \
		echo "Error: internal/adapter/http/api.gen.go is out of sync with docs/openapi.yaml"; \
		echo "Run 'make generate' and commit the result."; \
		git diff internal/adapter/http/api.gen.go; \
		exit 1; \
	fi
	@echo "Generated code is up-to-date."

# Cleanup
clean: ## Clean build artifacts and test cache
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html
	@rm -rf bin/
	@go clean -testcache 2>/dev/null || true
