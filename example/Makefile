.PHONY: help build run clean

APP_NAME := gogocoin-example

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

clean: ## Remove build artifacts
	@rm -rf bin/
