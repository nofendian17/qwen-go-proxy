# Docker Bake configuration for multi-platform builds
# Usage: docker buildx bake -f docker-bake.hcl

variable "REGISTRY" {
  default = "ghcr.io/nofendian17"
}

variable "TAG" {
  default = "latest"
}

variable "CACHE_TAG" {
  default = "cache"
}

variable "PROJECT_NAME" {
  default = "qwen-go-proxy"
}

group "default" {
  targets = ["image"]
}

group "all" {
  targets = ["image-amd64", "image-arm64", "image-armv7"]
}

target "image" {
  inherits = ["image-amd64", "image-arm64", "image-armv7"]
}

target "image-amd64" {
  context = "."
  dockerfile = "Dockerfile"
  platforms = ["linux/amd64"]
  tags = [
    "${REGISTRY}/${PROJECT_NAME}:${TAG}-amd64",
    "${REGISTRY}/${PROJECT_NAME}:latest-amd64",
  ]
  cache-from = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-amd64"
  ]
  cache-to = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-amd64,mode=max"
  ]
  labels = {
    "org.opencontainers.image.title" = "qwen-go-proxy"
    "org.opencontainers.image.description" = "Qwen API Proxy with OpenAI-compatible endpoints"
    "org.opencontainers.image.url" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.source" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.version" = "${TAG}"
    "org.opencontainers.image.created" = "${timestamp()}"
    "org.opencontainers.image.revision" = "${git_sha()}"
    "org.opencontainers.image.licenses" = "MIT"
  }
}

target "image-arm64" {
  context = "."
  dockerfile = "Dockerfile"
  platforms = ["linux/arm64"]
  tags = [
    "${REGISTRY}/${PROJECT_NAME}:${TAG}-arm64",
    "${REGISTRY}/${PROJECT_NAME}:latest-arm64",
  ]
  cache-from = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-arm64"
  ]
  cache-to = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-arm64,mode=max"
  ]
  labels = {
    "org.opencontainers.image.title" = "qwen-go-proxy"
    "org.opencontainers.image.description" = "Qwen API Proxy with OpenAI-compatible endpoints"
    "org.opencontainers.image.url" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.source" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.version" = "${TAG}"
    "org.opencontainers.image.created" = "${timestamp()}"
    "org.opencontainers.image.revision" = "${git_sha()}"
    "org.opencontainers.image.licenses" = "MIT"
  }
}

target "image-armv7" {
  context = "."
  dockerfile = "Dockerfile"
  platforms = ["linux/arm/v7"]
  tags = [
    "${REGISTRY}/${PROJECT_NAME}:${TAG}-armv7",
    "${REGISTRY}/${PROJECT_NAME}:latest-armv7",
  ]
  cache-from = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-armv7"
  ]
  cache-to = [
    "type=registry,ref=${REGISTRY}/${PROJECT_NAME}:${CACHE_TAG}-armv7,mode=max"
  ]
  labels = {
    "org.opencontainers.image.title" = "qwen-go-proxy"
    "org.opencontainers.image.description" = "Qwen API Proxy with OpenAI-compatible endpoints"
    "org.opencontainers.image.url" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.source" = "https://github.com/nofendian17/qwen-go"
    "org.opencontainers.image.version" = "${TAG}"
    "org.opencontainers.image.created" = "${timestamp()}"
    "org.opencontainers.image.revision" = "${git_sha()}"
    "org.opencontainers.image.licenses" = "MIT"
  }
}