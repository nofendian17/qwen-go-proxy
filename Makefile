# =============================================================================
# Qwen Go Proxy - Makefile for Docker and Release Operations
# =============================================================================

.PHONY: help build build-all push push-all release test clean docker-login

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Variables
REGISTRY ?= ghcr.io/nofendian17
IMAGE_NAME = qwen-go-proxy
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "latest")
PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
PROJECT_NAME = qwen-go-proxy

# Build targets
build: ## Build Docker image for current platform
	docker build -t $(REGISTRY)/$(IMAGE_NAME):$(VERSION) .

build-all: ## Build multi-platform Docker images
	docker buildx build \
		--platform $(PLATFORMS) \
		--tag $(REGISTRY)/$(IMAGE_NAME):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE_NAME):latest \
		--push \
		.

build-local: ## Build multi-platform Docker images locally
	docker buildx build \
		--platform $(PLATFORMS) \
		--tag $(REGISTRY)/$(IMAGE_NAME):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE_NAME):latest \
		--load \
		.

# Push targets
docker-login: ## Log in to GitHub Container Registry
	@echo "Logging in to $(REGISTRY)..."
	@docker login $(REGISTRY)

push: docker-login ## Push Docker image to registry
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

push-latest: docker-login ## Push latest tag to registry
	docker tag $(REGISTRY)/$(IMAGE_NAME):$(VERSION) $(REGISTRY)/$(IMAGE_NAME):latest
	docker push $(REGISTRY)/$(IMAGE_NAME):latest

push-all: docker-login ## Push all platform-specific images
	docker buildx build \
		--platform $(PLATFORMS) \
		--tag $(REGISTRY)/$(IMAGE_NAME):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE_NAME):latest \
		--push \
		.

# Bake targets (using docker-bake.hcl)
bake: ## Build using Docker Bake
	docker buildx bake -f docker-bake.hcl

bake-push: ## Build and push using Docker Bake
	docker buildx bake -f docker-bake.hcl --push

# Development targets
test: ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-docker: ## Test Docker image locally
	@echo "Testing Docker image..."
	docker run --rm -p 8080:8080 $(REGISTRY)/$(IMAGE_NAME):$(VERSION) &
	@sleep 5
	@curl -f http://localhost:8080/health || (echo "Health check failed" && exit 1)
	@pkill -f qwen-go-proxy || true

# Release targets
release: ## Full release process (requires goreleaser)
	@echo "Releasing version $(VERSION)..."
	goreleaser release --clean

release-snapshot: ## Create snapshot release
	goreleaser release --snapshot --clean

release-check: ## Check release configuration
	goreleaser check

# Utility targets
clean: ## Clean up Docker resources
	docker system prune -f
	docker volume prune -f

clean-images: ## Remove all qwen-go-proxy images
	docker images $(REGISTRY)/$(IMAGE_NAME) -q | xargs -r docker rmi -f

info: ## Show build information
	@echo "Registry: $(REGISTRY)"
	@echo "Image: $(IMAGE_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Platforms: $(PLATFORMS)"
	@echo "Git SHA: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# Docker Compose targets
compose-up: ## Start services with Docker Compose
	docker-compose up -d

compose-down: ## Stop services with Docker Compose
	docker-compose down

compose-logs: ## Show Docker Compose logs
	docker-compose logs -f

compose-pull: ## Pull latest images with Docker Compose
	docker-compose pull

# Development workflow targets
dev-setup: ## Set up development environment
	go mod download
	go install github.com/goreleaser/goreleaser/v2@latest

dev-build: ## Build binary locally
	go build -o bin/qwen-go-proxy ./cmd/server

dev-run: dev-build ## Run binary locally
	./bin/qwen-go-proxy