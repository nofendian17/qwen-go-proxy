FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd ./cmd
COPY ./internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o qwen-go-proxy ./cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl
RUN addgroup -S qwen && adduser -S qwen -G qwen

WORKDIR /home/qwen
COPY --from=builder /app/qwen-go-proxy .

EXPOSE 8080
USER qwen
CMD ["./qwen-go-proxy"]