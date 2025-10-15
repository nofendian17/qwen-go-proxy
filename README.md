# Qwen Go Proxy

A production-ready HTTP proxy server that provides OpenAI-compatible API endpoints for Alibaba's Qwen AI models, written in Go. This proxy enables seamless integration with applications expecting OpenAI API format while using Qwen's authentication and AI capabilities.

## Features

- 🏗️ **OpenAI-Compatible Endpoints**: `/v1/chat/completions`, `/v1/completions`, `/v1/models`
- 🔐 **OAuth2 Authentication**: Automatic device flow authentication with Qwen
- ⚡ **High Performance**: Built with Gin framework for low latency
- 🛡️ **Security Features**: Rate limiting, CORS support, security headers
- 📊 **Monitoring**: Health checks, detailed system metrics, structured logging
- 🐳 **Docker Support**: Containerized deployment with Docker Compose
- 🔄 **Token Management**: Automatic refresh of OAuth tokens
- 🎛️ **Configurable**: Environment-based configuration with sensible defaults

## Requirements

### System Requirements
- **Go**: 1.24.8 or later (for building from source)
- **Docker**: Latest version (for containerized deployment)
- **Operating System**: Linux, macOS, or Windows with WSL2

### External Requirements
- **Qwen Account**: Active account with Alibaba Cloud Qwen
- **Network Access**: Outbound HTTPS access to `chat.qwen.ai` and `portal.qwen.ai`
- **Storage**: Write access for credential storage (default: `.qwen` directory)

### Optional Dependencies
- **git**: For version control (recommended)
- **make**: For using Makefile commands (if provided)

## Installation

### Option 1: Docker (Recommended)

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd qwen-go
   ```

2. **Configure environment** (optional):
   Create a `.env` file:
   ```env
   SERVER_PORT=:8080
   SERVER_HOST=0.0.0.0
   LOG_LEVEL=info
   DEBUG_MODE=false
   RATE_LIMIT_RPS=10
   RATE_LIMIT_BURST=20
   ```

3. **Start with Docker Compose**:
   ```bash
   docker-compose up -d
   ```

### Option 2: Build from Source

1. **Install Go 1.24.8+**:
   ```bash
   # On Ubuntu/Debian
   wget https://go.dev/dl/go1.24.8.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.24.8.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin
   ```

2. **Clone and build**:
   ```bash
   git clone <repository-url>
   cd qwen-go
   go mod download
   go build -o qwen-go-proxy ./cmd/server
   ```

3. **Run the server**:
   ```bash
   ./qwen-go-proxy
   ```

## Usage

### First-Time Setup

On first startup, the proxy will automatically initiate OAuth2 device authentication:

1. The server will provide a login URL
2. Visit the URL in your browser and authenticate with Qwen
3. The proxy will store credentials locally in `.qwen/` directory

### API Endpoints

#### Health Checks
- `GET /` - Basic server status
- `GET /health` - OpenAI-compatible health check
- `GET /health/detailed` - Comprehensive health status with metrics

#### Authentication
- `GET /auth` - Initiate OAuth2 authentication flow

#### OpenAI-Compatible APIs
- `GET /v1/models` - List available models
- `POST /v1/chat/completions` - Chat completions (streaming supported)
- `POST /v1/completions` - Text completions

### Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `SERVER_PORT` | `:8080` | Server port |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `DEBUG_MODE` | `false` | Enable debug mode |
| `RATE_LIMIT_RPS` | `10` | Requests per second limit |
| `RATE_LIMIT_BURST` | `20` | Burst capacity for rate limiting |
| `QWEN_DIR` | `.qwen` | Directory for credential storage |
| `READ_TIMEOUT` | `30s` | HTTP read timeout |
| `WRITE_TIMEOUT` | `30s` | HTTP write timeout |

## Integration with n8n

n8n is a powerful workflow automation tool that can integrate with various APIs. This proxy enables you to use Qwen AI models in your n8n workflows through HTTP Request nodes.

### Basic Setup in n8n

1. **Add HTTP Request Node**:
   - Method: `POST`
   - URL: `http://your-proxy-host:8080/v1/chat/completions`

2. **Headers** (if required):
   ```json
   {
     "Content-Type": "application/json"
   }
   ```

3. **Request Body** (Chat Completion Example):
   ```json
   {
     "model": "qwen3-coder-plus",
     "messages": [
       {
         "role": "user",
         "content": "Hello, how can I help you?"
       }
     ],
     "max_tokens": 150,
     "temperature": 0.7
   }
   ```

### Advanced n8n Workflows

#### Chatbot Automation Workflow

1. **Webhook Node**: Receives incoming messages
2. **HTTP Request Node**: Sends message to Qwen API proxy
3. **Function Node**: Processes the response
4. **HTTP Request Node**: Sends response back to user

Example workflow configuration:

```json
{
  "nodes": [
    {
      "name": "Webhook",
      "type": "n8n-nodes-base.webhook",
      "parameters": {
        "path": "chat",
        "responseMode": "responseNode"
      }
    },
    {
      "name": "Qwen Chat",
      "type": "n8n-nodes-base.httpRequest",
      "parameters": {
        "method": "POST",
        "url": "http://localhost:8080/v1/chat/completions",
        "sendBody": true,
        "bodyContentType": "json",
        "bodyParameters": {
          "model": "qwen3-coder-plus",
          "messages": "={{ $json.body.messages }}",
          "max_tokens": 500
        }
      }
    },
    {
      "name": "Response",
      "type": "n8n-nodes-base.respondToWebhook",
      "parameters": {
        "respondWith": "json",
        "responseBody": "={{ $json }}"
      }
    }
  ],
  "connections": {
    "Webhook": { "main": [[{ "node": "Qwen Chat", "type": "main", "index": 0 }]] },
    "Qwen Chat": { "main": [[{ "node": "Response", "type": "main", "index": 0 }]] }
  }
}
```

#### Content Moderation Workflow

```json
{
  "nodes": [
    {
      "name": "Content Input",
      "type": "n8n-nodes-base.manualTrigger"
    },
    {
      "name": "Moderation Check",
      "type": "n8n-nodes-base.httpRequest",
      "parameters": {
        "method": "POST",
        "url": "http://localhost:8080/v1/chat/completions",
        "bodyParameters": {
          "model": "qwen-turbo",
          "messages": [
            {
              "role": "system",
              "content": "You are a content moderator. Analyze the following content and determine if it contains inappropriate material. Respond with 'safe' or 'unsafe' followed by a brief explanation."
            },
            {
              "role": "user",
              "content": "={{ $json.input_content }}"
            }
          ]
        }
      }
    },
    {
      "name": "Decision",
      "type": "n8n-nodes-base.switch",
      "parameters": {
        "conditions": {
          "string": [
            {
              "value1": "={{ $json.choices[0].message.content }}",
              "operation": "contains",
              "value2": "safe"
            }
          ]
        }
      }
    }
  ]
}
```

### Best Practices for n8n Integration

1. **Rate Limiting**: Respect the proxy's rate limits (default 10 RPS)
2. **Error Handling**: Add error handling nodes for API failures
3. **Streaming**: Use streaming responses for real-time chat applications
4. **Authentication**: No additional auth headers needed - OAuth is handled by proxy
5. **Health Monitoring**: Use `/health/detailed` endpoint for monitoring proxy status
6. **Batch Processing**: Group multiple requests when processing bulk data

### Production Considerations

- **Load Balancing**: Run multiple proxy instances behind a load balancer
- **Monitoring**: Use n8n's built-in monitoring or external tools
- **Security**: Keep proxy behind firewall, use HTTPS in production
- **Scaling**: Adjust rate limits based on your n8n workflow requirements

## Development

### Building
```bash
go build -o qwen-go-proxy ./cmd/server
```

### Testing
```bash
go test ./...
```

### Docker Development
```bash
docker-compose up --build
```

## Troubleshooting

### Common Issues

1. **Authentication Failures**:
   - Ensure Qwen account has API access
   - Check network connectivity to `chat.qwen.ai`
   - Clear `.qwen/` directory and re-authenticate

2. **Rate Limiting**:
   - Increase `RATE_LIMIT_RPS` and `RATE_LIMIT_BURST` in configuration
   - Implement exponential backoff in n8n workflows

3. **Connection Refused**:
   - Verify server is running on correct host/port
   - Check firewall rules
   - Use `docker-compose logs` for container debugging

### Logs
View logs with:
```bash
docker-compose logs -f qwen-api-proxy
```

Or from source:
```bash
LOG_LEVEL=debug ./qwen-go-proxy
```

## License

[Add your license here]

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and add tests
4. Submit a pull request

## Support

For issues and questions:
- Check the troubleshooting section above
- Review n8n documentation for workflow examples
- Open an issue in the repository