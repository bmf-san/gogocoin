.PHONY: help build run up down logs restart rebuild clean

APP_NAME := gogocoin-example
COMPOSE := docker compose

.DEFAULT_GOAL := help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the example binary
	@go build -o bin/$(APP_NAME) ./cmd/
	@echo "Built: bin/$(APP_NAME)"

run: ## Run the example (requires configs/config.yaml and BITFLYER_API_KEY/SECRET env vars)
	@[ -f configs/config.yaml ] || { echo "Error: configs/config.yaml not found. Copy configs/config.example.yaml and fill in your API credentials."; exit 1; }
	@go run ./cmd/

# Docker Compose commands (build context is the repo root)
# Requires BITFLYER_API_KEY and BITFLYER_API_SECRET
up: ## Start containers
	$(COMPOSE) up -d

down: ## Stop containers
	$(COMPOSE) down

logs: ## Show logs
	$(COMPOSE) logs -f

restart: ## Restart containers
	$(COMPOSE) restart

rebuild: ## Rebuild image and start
	$(COMPOSE) up -d --build

clean: ## Remove build artifacts
	@rm -rf bin/
