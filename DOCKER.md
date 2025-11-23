# Docker Multi-Platform Build Guide

This document explains the Docker configuration for building multi-platform images of Qwen Go Proxy for GitHub Container Registry (ghcr).

## Overview

The project supports building Docker images for multiple architectures via GitHub Actions:
- `linux/amd64` - Standard x86_64 servers and desktops
- `linux/arm64` - ARM64 servers (AWS Graviton, Apple M1/M2, etc.)

## Configuration Files

### 1. Dockerfile
Optimized for multi-platform builds with:
- Alpine Linux 3.22 as base image for consistency
- Proper user permissions (UID/GID 1001) for security
- Non-root user setup
- Minimal dependencies for smaller image size

### 2. .github/workflows/release.yml
GitHub Actions workflow for automated releases:
- Uses `docker/setup-qemu-action` and `docker/setup-buildx-action`
- Builds and pushes multi-platform images to GHCR
- Generates SBOM for security compliance

## Building Images

### Using Makefile

For local development (single platform):
```bash
# Build Docker image locally
make docker-build
```

### Using Docker Compose

```bash
# Start the service
make docker-compose-up
# or
docker compose up -d --build
```

## Image Tags

The project uses semantic versioning for image tags pushed to GHCR:

- `ghcr.io/nofendian17/qwen-go-proxy:1.0.0` - Versioned
- `ghcr.io/nofendian17/qwen-go-proxy:v1` - Major version
- `ghcr.io/nofendian17/qwen-go-proxy:v1.0` - Major.Minor
- `ghcr.io/nofendian17/qwen-go-proxy:latest` - Latest release

## Running with Docker Compose

The `docker-compose.yml` file supports flexible image selection:

```bash
# Use latest version (default)
docker compose up -d

# Use specific version
IMAGE_TAG=ghcr.io/nofendian17/qwen-go-proxy:1.0.0 docker compose up -d
```

## Development Workflow

### Local Development
```bash
# Set up development environment (creates auths/ directory)
make docker-setup

# Build and run locally
make docker-run
```

## Release Process

1. **Create and push a tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will**:
   - Run tests
   - Build binaries using GoReleaser
   - Build and push multi-arch Docker images using Docker Buildx
   - Generate SBOM
   - Create GitHub release

3. **Verify release**:
   ```bash
   # Pull image
   docker pull ghcr.io/nofendian17/qwen-go-proxy:1.0.0
   ```

## Troubleshooting

### Runtime Issues
- Check permissions on the `auths` directory (should be writable by UID 1001).
- Review container logs with `docker compose logs`.

### Security Considerations

- Images run as non-root user (UID/GID 1001).
- Minimal Alpine base image reduces attack surface.
- SBOM generation for dependency tracking.
