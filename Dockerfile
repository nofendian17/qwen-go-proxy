# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o qwen-go-proxy ./cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl
RUN addgroup -S qwen && adduser -S qwen -G qwen

WORKDIR /home/qwen
COPY --from=builder /app/qwen-go-proxy .

EXPOSE 8080

CMD ["./qwen-go-proxy"]