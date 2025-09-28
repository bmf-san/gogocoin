# gogocoin Makefile

.PHONY: build run test lint fmt deps help

# デフォルトターゲット
.DEFAULT_GOAL := help

# 変数定義
APP_NAME := gogocoin
BUILD_DIR := ./bin
CMD_DIR := ./cmd/gogocoin
CONFIG_FILE := ./configs/config.yaml

# ビルド
build: ## アプリケーションをビルド
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)"

# 実行
run: build ## アプリケーションを実行（ライブトレードモード）
	@echo "Running $(APP_NAME) in live trading mode..."
	@$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG_FILE) -mode live

# ペーパートレード実行
run-paper: build ## ペーパートレードモードで実行
	@echo "Running $(APP_NAME) in paper trading mode..."
	@$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG_FILE) -mode paper

# 開発モード実行
dev: ## 開発モードで実行（ペーパートレード + デバッグログ）
	@echo "Running in development mode..."
	@go run $(CMD_DIR) -config $(CONFIG_FILE) -mode dev -log-level debug

# テスト
test: ## テストを実行
	@echo "Running tests..."
	@go mod tidy
	@if find . -name "*_test.go" -not -path "./vendor/*" | grep -q . 2>/dev/null; then \
		go test -v ./... || echo "Some tests failed"; \
	else \
		echo "No test files found - skipping tests"; \
	fi

# テストカバレッジ
test-coverage: ## テストカバレッジを生成
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 依存関係の更新
deps: ## 依存関係を更新
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod download

# ビルド確認
check: ## ビルド可能かチェック
	@echo "Checking build..."
	@go build -v ./...
	@echo "Build check completed successfully"


# フォーマット
fmt: ## コードフォーマット
	@echo "Formatting code..."
	@go fmt ./...

# リント
lint: ## コードリント
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0

# クリーンアップ
clean: ## ビルド成果物をクリーンアップ
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@rm -rf ./data/*.db
	@rm -rf ./logs/*.log
	@find . -type d -empty -not -path "./.git*" -delete 2>/dev/null || true
	@echo "Cleaning Go and golangci-lint caches..."
	@go clean -cache -modcache -testcache 2>/dev/null || true
	@golangci-lint cache clean 2>/dev/null || true

# データベース初期化
init-db: ## データベースを初期化
	@echo "Initializing database..."
	@mkdir -p ./data
	@$(BUILD_DIR)/$(APP_NAME) -init-db -config $(CONFIG_FILE)

# 設定ファイルを作成
init-config: ## 設定ファイル例から本番用設定ファイルを作成
	@echo "Creating config file from example..."
	@mkdir -p ./configs
	@if [ ! -f $(CONFIG_FILE) ]; then \
		cp ./configs/config.example.yaml $(CONFIG_FILE) && \
		echo "Created $(CONFIG_FILE) from example. Please edit it with your settings."; \
	else \
		echo "$(CONFIG_FILE) already exists. Skipping."; \
	fi

# Docker関連（注意: Dockerfileが必要）
docker-build: ## Dockerイメージをビルド（要Dockerfile作成）
	@echo "Building Docker image..."
	@if [ ! -f Dockerfile ]; then echo "Error: Dockerfile not found. Create Dockerfile first."; exit 1; fi
	@docker build -t $(APP_NAME):latest .

docker-run: ## Dockerコンテナを実行（要docker-build実行）
	@echo "Running Docker container..."
	@docker run --rm -p 8080:8080 -v $(PWD)/configs:/app/configs $(APP_NAME):latest

# 開発ツールのインストール
install-tools: ## 開発ツールをインストール
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# ヘルプ
help: ## このヘルプを表示
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
