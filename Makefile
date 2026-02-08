.PHONY: build build-mcp test test-unit lint docker-build docker-run docker-down clean

APP_NAME := nexus
MCP_NAME := nexus-mcp
BUILD_DIR := bin

build:
	@echo "Building $(APP_NAME)..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/nexus

build-mcp:
	@echo "Building $(MCP_NAME)..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(MCP_NAME) ./cmd/nexus-mcp

build-all: build build-mcp

test:
	@echo "Running all tests..."
	go test ./... -v -count=1 -race

test-unit:
	@echo "Running unit tests..."
	go test ./internal/... -v -count=1 -race

test-mcp:
	@echo "Running MCP tests..."
	go test ./internal/mcp/... -v -count=1 -race

test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	@echo "Running linter..."
	golangci-lint run ./...

docker-build:
	@echo "Building Docker image..."
	docker compose build

docker-run:
	@echo "Starting services..."
	docker compose up -d

docker-down:
	@echo "Stopping services..."
	docker compose down

docker-logs:
	docker compose logs -f nexus

run-mcp:
	@echo "Running MCP server (stdio)..."
	NEXUS_MCP_TRANSPORT=stdio go run ./cmd/nexus-mcp

run-mcp-sse:
	@echo "Running MCP server (SSE on :8081)..."
	NEXUS_MCP_TRANSPORT=sse go run ./cmd/nexus-mcp

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	docker compose down -v 2>/dev/null || true
