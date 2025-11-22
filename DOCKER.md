# Docker Multi-Platform Build Guide

This document explains the Docker configuration for building multi-platform images of Qwen Go Proxy for GitHub Container Registry (ghcr).

## Overview

The project supports building Docker images for multiple architectures:
- `linux/amd64` - Standard x86_64 servers and desktops
- `linux/arm64` - ARM64 servers (AWS Graviton, Apple M1/M2, etc.)
- `linux/arm/v7` - ARM v7 devices (Raspberry Pi 3/4, etc.)

## Configuration Files

### 1. Dockerfile
Optimized for multi-platform builds with:
- Alpine Linux 3.19 as base image for consistency
- Support for dynamic platform detection
- Proper user permissions (UID/GID 1001)
- Health check endpoint
- Minimal dependencies for smaller image size

### 2. .goreleaser.yaml
Configured for automated multi-platform builds:
- Separate Docker builds for each architecture
- Proper image tagging with version and architecture suffixes
- Multi-arch manifest creation
- OCI-compliant labels
- BuildKit integration for faster builds

### 3. docker-bake.hcl
Alternative build configuration using Docker Bake:
- Parallel builds for all platforms
- Registry caching for faster subsequent builds
- Flexible variable substitution
- Consistent labeling across platforms

### 4. .github/workflows/release.yml
GitHub Actions workflow for automated releases:
- Separate test job for quality assurance
- Multi-platform QEMU support
- BuildKit builder setup
- SBOM generation for security compliance
- Artifact collection for traceability

## Building Images

### Using GoReleaser (Recommended)

For releases:
```bash
# Full release (requires proper git tag)
make release

# Snapshot build for testing
make release-snapshot
```

### Using Docker Bake

For manual builds:
```bash
# Build all platforms
make bake

# Build and push all platforms
make bake-push

# Build specific platform
docker buildx bake -f docker-bake.hcl image-amd64
```

### Using Docker Buildx Directly

For custom builds:
```bash
# Build all platforms and push
make build-all

# Build locally for current platform
make build

# Build all platforms locally
make build-local
```

## Image Tags

The project uses semantic versioning for image tags:

### Architecture-specific tags
- `ghcr.io/nofendian17/qwen-go-proxy:1.0.0-amd64`
- `ghcr.io/nofendian17/qwen-go-proxy:1.0.0-arm64`
- `ghcr.io/nofendian17/qwen-go-proxy:1.0.0-armv7`

### Multi-arch manifest tags
- `ghcr.io/nofendian17/qwen-go-proxy:1.0.0` - Versioned
- `ghcr.io/nofendian17/qwen-go-proxy:v1.0` - Major version
- `ghcr.io/nofendian17/qwen-go-proxy:v1.0.0` - Major.Minor
- `ghcr.io/nofendian17/qwen-go-proxy:latest` - Latest release

## Running with Docker Compose

The `docker-compose.yml` file supports flexible image selection:

```bash
# Use latest version (default)
docker-compose up -d

# Use specific version
IMAGE_TAG=ghcr.io/nofendian17/qwen-go-proxy:1.0.0 docker-compose up -d

# Use specific architecture
IMAGE_TAG=ghcr.io/nofendian17/qwen-go-proxy:1.0.0-arm64 docker-compose up -d
```

## Development Workflow

### Local Development
```bash
# Set up development environment
make dev-setup

# Build and run locally
make dev-run

# Test Docker image locally
make test-docker
```

### Testing Multi-Platform Support
```bash
# Build all platforms locally
make build-local

# Test specific platform
docker run --platform linux/arm64 --rm -p 8080:8080 ghcr.io/nofendian17/qwen-go-proxy:latest
```

## Release Process

1. **Create and push a tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will**:
   - Run all tests
   - Build binaries for all platforms
   - Build Docker images for all architectures
   - Create and push multi-arch manifests
   - Generate SBOM
   - Create GitHub release

3. **Verify release**:
   ```bash
   # Pull multi-arch image
   docker pull ghcr.io/nofendian17/qwen-go-proxy:1.0.0
   
   # Verify manifest
   docker manifest inspect ghcr.io/nofendian17/qwen-go-proxy:1.0.0
   ```

## Troubleshooting

### Build Issues
- Ensure QEMU is installed for cross-platform builds
- Check BuildKit is properly configured
- Verify registry permissions

### Runtime Issues
- Check platform compatibility with `docker run --platform`
- Verify health check endpoint is accessible
- Review container logs with `docker-compose logs`

### Caching Issues
- Clear Docker cache: `make clean`
- Reset BuildKit builder: `docker buildx rm builder && docker buildx create --use`

## Security Considerations

- Images run as non-root user (UID/GID 1001)
- Minimal Alpine base image reduces attack surface
- SBOM generation for dependency tracking
- Regular base image updates
- No sensitive data in image layers

## Performance Optimizations

- Multi-stage builds (handled by GoReleaser)
- Registry caching between builds
- Parallel platform builds
- Layer caching with BuildKit
- Minimal dependencies in runtime image