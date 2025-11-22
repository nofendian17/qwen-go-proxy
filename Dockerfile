# ===============================================
# FINAL RUNTIME IMAGE (NO BUILD STAGE REQUIRED)
# GoReleaser already builds the binary in advance.
# ===============================================

FROM alpine:latest

# Install minimal dependencies
RUN apk --no-cache add ca-certificates curl

# Non-root user for security
RUN addgroup -S qwen && adduser -S qwen -G qwen

WORKDIR /home/qwen

# GoReleaser will copy the compiled binary into this location
COPY qwen-go-proxy .

# Fix permissions for non-root execution
RUN chown qwen:qwen /home/qwen/qwen-go-proxy

USER qwen

CMD ["./qwen-go-proxy"]
