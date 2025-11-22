# ===============================================
# RUNTIME IMAGE FOR GORELEASER
# This Dockerfile is optimized for GoReleaser builds.
# The binary is provided by GoReleaser.
# ===============================================

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

# Copy the binary from GoReleaser
COPY --chown=qwen:qwen qwen-go-proxy /usr/local/bin/qwen-go-proxy

# Expose default port (documentation only)
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Switch to non-root user
USER qwen

# Default command
CMD ["/usr/local/bin/qwen-go-proxy"]
