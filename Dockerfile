# ===============================================
# MULTI-PLATFORM BUILD AND RUNTIME IMAGE
# This Dockerfile can be used both for direct builds and GoReleaser.
# Supports amd64, arm64, and arm/v7 architectures.
# ===============================================

# BUILDER STAGE
# Use Go Alpine image for building the binary
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

# Install git for Go modules
RUN apk --no-cache add git ca-certificates curl

# Create non-root user for security with consistent UID/GID
RUN addgroup -g 1001 qwen && \
    adduser -D -s /bin/sh -u 1001 -G qwen qwen

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o qwen-go-proxy ./cmd/server

# RUNTIME STAGE
# Use specific Alpine version for consistency across platforms
FROM alpine:3.19

# Install minimal dependencies
RUN apk --no-cache add \
    ca-certificates \
    curl \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user for security with consistent UID/GID
RUN addgroup -g 1001 qwen && \
    adduser -D -s /bin/sh -u 1001 -G qwen qwen

# Set working directory
WORKDIR /home/qwen

# Copy the binary from builder stage
COPY --from=builder --chown=qwen:qwen /app/qwen-go-proxy .

# Expose default port (documentation only)
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["./qwen-go-proxy"]
