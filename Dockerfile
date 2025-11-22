FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# GoReleaser will copy the pre-built binary, so we don't need to build here
# This Dockerfile is used by GoReleaser which provides the built binary

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl
RUN addgroup -S qwen && adduser -S qwen -G qwen

WORKDIR /home/qwen
# GoReleaser copies the pre-built binary here
COPY qwen-go-proxy .

EXPOSE 8080
USER qwen
CMD ["./qwen-go-proxy"]