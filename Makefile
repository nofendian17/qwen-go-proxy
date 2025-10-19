# Qwen Go Proxy Makefile

.PHONY: help build test clean docker-build docker-run release release-dry-run install-goreleaser

# Default target
help: ## Show this help message
	@echo "Qwen Go Proxy - Development Makefile"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the binary for current platform
	go build -o bin/qwen-go-proxy ./cmd/server

build-all: ## Build binaries for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/qwen-go-proxy-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build -o bin/qwen-go-proxy-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -o bin/qwen-go-proxy-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build -o bin/qwen-go-proxy-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o bin/qwen-go-proxy-windows-amd64.exe ./cmd/server
	@echo "Binaries built in bin/ directory"

test: ## Run all tests
	go test ./...

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-verbose: ## Run tests with verbose output
	go test -v ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go code
	go fmt ./...
	goimports -w .

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image
	docker build -t qwen-go-proxy .

docker-run: ## Run Docker container
	docker run -p 8080:8080 --env-file .env qwen-go-proxy

docker-compose-up: ## Start with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop docker-compose
	docker-compose down

install-goreleaser: ## Install GoReleaser
	go install github.com/goreleaser/goreleaser@latest

release-dry-run: ## Test release process without publishing
	goreleaser release --clean --skip-publish --snapshot

release-check: ## Check if release is ready
	goreleaser check

release: ## Create a release (requires GITHUB_TOKEN)
	@echo "Creating release..."
	@echo "Make sure you have:"
	@echo "1. Tagged the release: git tag v1.0.0"
	@echo "2. Pushed the tag: git push origin v1.0.0"
	@echo "3. Set GITHUB_TOKEN environment variable"
	@goreleaser release --clean

version: ## Show current version from git
	@echo "Current version: $(shell git describe --tags --abbrev=0 2>/dev/null || echo 'v0.0.0-dev')"

deps: ## Download dependencies
	go mod download
	go mod tidy

deps-update: ## Update dependencies
	go get -u ./...
	go mod tidy

generate: ## Generate code (mocks, etc.)
	go generate ./...

# Development setup
dev-setup: ## Setup development environment
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# CI targets
ci-test: test ## Run tests for CI
ci-lint: lint ## Run linting for CI
ci-build: build ## Run build for CI