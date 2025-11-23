# ===============================================
# BUILDER IMAGE
# ===============================================
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

# Build the binary with the original name 'qwen-go-proxy'
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o qwen-go-proxy ./cmd/server/

# ===============================================
# RUNTIME IMAGE
# ===============================================
FROM alpine:3.22.0

# Install dependencies including tzdata and curl for healthcheck
RUN apk --no-cache add \
    ca-certificates \
    curl \
    tzdata \
    && rm -rf /var/cache/apk/*

# Setup Timezone
ENV TZ=Asia/Jakarta
RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

# Create non-root user with specific UID/GID (1001) matching previous version
RUN addgroup -g 1001 qwen && \
    adduser -D -s /bin/sh -u 1001 -G qwen qwen

# Set working directory
WORKDIR /app

# Copy the binary from builder to /usr/local/bin (matching previous path)
COPY --from=builder /app/qwen-go-proxy /usr/local/bin/qwen-go-proxy

# Copy .env.example
COPY .env.example .env.example

# Set permissions
RUN chown -R qwen:qwen /app /usr/local/bin/qwen-go-proxy

# Set environment variables
ENV SERVER_PORT=8080
ENV SERVER_HOST=0.0.0.0

# Expose default port
EXPOSE 8080

# Health check endpoint (Restored from previous version)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Switch to non-root user
USER qwen

# Default command
CMD ["/usr/local/bin/qwen-go-proxy"]
