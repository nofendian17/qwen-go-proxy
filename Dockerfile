# ===============================================
# MULTI-PLATFORM RUNTIME IMAGE
# GoReleaser already builds the binary in advance.
# Supports amd64, arm64, and arm/v7 architectures.
# ===============================================

# Use specific Alpine version for consistency across platforms
FROM --platform=$BUILDPLATFORM alpine:3.19

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

# GoReleaser will copy the compiled binary into this location
# The binary name and architecture will be handled by GoReleaser
COPY qwen-go-proxy .

# Fix permissions for non-root execution
RUN chown -R qwen:qwen /home/qwen

# Switch to non-root user
USER qwen

# Expose default port (documentation only)
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["./qwen-go-proxy"]
