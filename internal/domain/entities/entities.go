// Package entities contains pure domain entities and data structures for the Qwen API proxy.
// This package defines the core business objects without any infrastructure dependencies.
package entities

import (
	"fmt"
	"time"
)

// Credentials Security note: This struct contains sensitive authentication data.
// Access tokens should be handled securely and never logged in plain text.
type Credentials struct {
	AccessToken  string `json:"access_token"`  // Sensitive: never log this value
	TokenType    string `json:"token_type"`    // e.g., "Bearer"
	RefreshToken string `json:"refresh_token"` // Sensitive: never log this value
	ExpiryDate   int64  `json:"expiry_date"`   // Unix timestamp in milliseconds
	ResourceURL  string `json:"resource_url,omitempty"`
}

// Sanitize returns a safe representation of credentials for logging (without sensitive data).
// This is a domain method that doesn't depend on external infrastructure.
func (c *Credentials) Sanitize() map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"token_type":   c.TokenType,
		"expiry_date":  c.ExpiryDate,
		"resource_url": c.ResourceURL,
		"has_token":    c.AccessToken != "",
	}
}

// Config holds all configuration for the application.
// This struct supports JSON unmarshaling for configuration files and environment variables.
type Config struct {
	// Server configuration
	ServerPort   int           `json:"server_port" env:"SERVER_PORT" env-default:"8080"`
	ServerHost   string        `json:"server_host" env:"SERVER_HOST" env-default:"0.0.0.0"`
	ReadTimeout  time.Duration `json:"read_timeout" env:"READ_TIMEOUT" env-default:"30s"`
	WriteTimeout time.Duration `json:"write_timeout" env:"WRITE_TIMEOUT" env-default:"30s"`

	// Qwen OAuth configuration
	QWENOAuthBaseURL       string `json:"qwen_oauth_base_url" env:"QWEN_OAUTH_BASE_URL" env-required:"true"`
	QWENOAuthClientID      string `json:"qwen_oauth_client_id" env:"QWEN_OAUTH_CLIENT_ID" env-required:"true"`
	QWENOAuthScope         string `json:"qwen_oauth_scope" env:"QWEN_OAUTH_SCOPE" env-default:"openid profile email model.completion"`
	QWENOAuthDeviceAuthURL string `json:"qwen_oauth_device_auth_url" env:"QWEN_OAUTH_DEVICE_AUTH_URL" env-required:"true"`

	// Storage and file paths
	QWENDir string `json:"qwen_dir" env:"QWEN_DIR" env-default:".qwen"`

	// Token management
	TokenRefreshBuffer time.Duration `json:"token_refresh_buffer" env:"TOKEN_REFRESH_BUFFER" env-default:"5m"`
	ShutdownTimeout    time.Duration `json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" env-default:"30s"`

	// Logging configuration
	DebugMode bool   `json:"debug_mode" env:"DEBUG_MODE" env-default:"false"`
	LogLevel  string `json:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogFormat string `json:"log_format" env:"LOG_FORMAT" env-default:"json"`

	// Rate limiting
	RateLimitRequestsPerSecond int `json:"rate_limit_rps" env:"RATE_LIMIT_RPS" env-default:"10"`
	RateLimitBurst             int `json:"rate_limit_burst" env:"RATE_LIMIT_BURST" env-default:"20"`

	// API configuration
	APIBaseURL string `json:"api_base_url" env:"API_BASE_URL" env-required:"true"`

	// Security
	TrustedProxies []string `json:"trusted_proxies" env:"TRUSTED_PROXIES" env-separator:","`
	EnableTLS      bool     `json:"enable_tls" env:"ENABLE_TLS" env-default:"false"`
	TLSCertFile    string   `json:"tls_cert_file" env:"TLS_CERT_FILE"`
	TLSKeyFile     string   `json:"tls_key_file" env:"TLS_KEY_FILE"`
}

// GetServerAddress returns the full server address for HTTP server configuration.
// This is a domain helper method that formats the address.
func (c *Config) GetServerAddress() string {
	if c == nil {
		return ":8080"
	}
	return fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
}

// CompletionRequest represents a completion request with validation
type CompletionRequest struct {
	Model            string         `json:"model,omitempty" validate:"omitempty,min=1,max=100"`
	Prompt           interface{}    `json:"prompt" validate:"required"` // string or []string
	MaxTokens        int            `json:"max_tokens,omitempty" validate:"omitempty,min=1,max=4096"`
	Temperature      float64        `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	TopP             float64        `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
	Stream           bool           `json:"stream,omitempty"`
	Logprobs         int            `json:"logprobs,omitempty" validate:"omitempty,min=0,max=5"`
	Echo             bool           `json:"echo,omitempty"`
	Stop             interface{}    `json:"stop,omitempty"` // string or []string
	Suffix           string         `json:"suffix,omitempty" validate:"omitempty,max=100"`
	User             string         `json:"user,omitempty" validate:"omitempty,max=100"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty" validate:"omitempty,min=-2,max=2"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty" validate:"omitempty,min=-2,max=2"`
	BestOf           int            `json:"best_of,omitempty" validate:"omitempty,min=1,max=20"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	Seed             *int           `json:"seed,omitempty"`
}

// ChatCompletionRequest represents a chat completion request.
// This entity contains the data structure for chat completion API requests.
type ChatCompletionRequest struct {
	Model            string          `json:"model,omitempty" validate:"omitempty,min=1,max=100"`
	Messages         []ChatMessage   `json:"messages" validate:"required,min=1,dive"`
	MaxTokens        int             `json:"max_tokens,omitempty" validate:"omitempty,min=1,max=4096"`
	Temperature      float64         `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	TopP             float64         `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
	Stream           bool            `json:"stream,omitempty"`
	Stop             interface{}     `json:"stop,omitempty"` // string or []string
	User             string          `json:"user,omitempty" validate:"omitempty,max=100"`
	FrequencyPenalty float64         `json:"frequency_penalty,omitempty" validate:"omitempty,min=-2,max=2"`
	PresencePenalty  float64         `json:"presence_penalty,omitempty" validate:"omitempty,min=-2,max=2"`
	LogitBias        map[string]int  `json:"logit_bias,omitempty"`
	Logprobs         bool            `json:"logprobs,omitempty"`
	TopLogprobs      int             `json:"top_logprobs,omitempty" validate:"omitempty,min=0,max=20"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	Seed             *int            `json:"seed,omitempty"`
	Tools            []Tool          `json:"tools,omitempty" validate:"omitempty,dive"`
	ToolChoice       any             `json:"tool_choice,omitempty"` // string or ToolChoice
	ReasoningEffort  string          `json:"reasoning_effort,omitempty" validate:"omitempty,oneof=low medium high"`
	IncludeReasoning bool            `json:"include_reasoning,omitempty"`
	StreamOptions    *StreamOptions  `json:"stream_options,omitempty"`
}

// ChatMessage represents a message in chat completion.
// This entity contains the message structure for chat conversations.
type ChatMessage struct {
	Role      string      `json:"role" validate:"required,oneof=system user assistant tool"`
	Content   any         `json:"content" validate:"required"` // Can be string or []ContentBlock
	ToolCalls []ToolCall  `json:"tool_calls,omitempty" validate:"omitempty,dive"`
}

// ToolCall represents a tool call in a message.
// This entity represents function calling specifications.
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// ContentBlock represents a content block in a message (for multimodal)
type ContentBlock struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in a content block
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ResponseFormat represents response format options
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema interface{} `json:"json_schema,omitempty"`
}

// Tool represents a tool that can be called
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function tool
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// ToolChoice represents tool choice options
type ToolChoice struct {
	Type     string   `json:"type"`
	Function Function `json:"function,omitempty"`
}

// StreamOptions represents streaming options
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   *Usage             `json:"usage,omitempty"`
}

// CompletionChoice represents a choice in completion response
type CompletionChoice struct {
	Text         string      `json:"text"`
	Index        int         `json:"index"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *Usage                 `json:"usage,omitempty"`
}

// ChatCompletionChoice represents a choice in chat completion response
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	Delta        ChatMessage `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     *Logprobs   `json:"logprobs,omitempty"`
}

// Logprobs represents log probabilities for tokens
type Logprobs struct {
	Content []TokenLogprob `json:"content,omitempty"`
}

// TokenLogprob represents log probability information for a token
type TokenLogprob struct {
	Token       string            `json:"token"`
	Logprob     float64           `json:"logprob"`
	Bytes       []int             `json:"bytes,omitempty"`
	TopLogprobs []TopLogprobEntry `json:"top_logprobs,omitempty"`
}

// TopLogprobEntry represents a top log probability entry
type TopLogprobEntry struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelInfo represents model information
type ModelInfo struct {
	ID         string            `json:"id"`
	Object     string            `json:"object"`
	Created    int64             `json:"created"`
	OwnedBy    string            `json:"owned_by"`
	Permission []ModelPermission `json:"permission"`
}

// ModelPermission represents model permissions
type ModelPermission struct {
	ID                 string      `json:"id"`
	Object             string      `json:"object"`
	Created            int64       `json:"created"`
	AllowCreateEngine  bool        `json:"allow_create_engine"`
	AllowSampling      bool        `json:"allow_sampling"`
	AllowLogprobs      bool        `json:"allow_logprobs"`
	AllowSearchIndices bool        `json:"allow_search_indices"`
	AllowView          bool        `json:"allow_view"`
	AllowFineTuning    bool        `json:"allow_fine_tuning"`
	Organization       string      `json:"organization"`
	Group              interface{} `json:"group"`
	IsBlocking         bool        `json:"is_blocking"`
}
