// Package entities contains domain entities and data structures for the Qwen API proxy.
// This package defines the core business objects and their validation rules.
package entities

import (
	"fmt"
	"time"
)

// Credentials represents OAuth credentials for Qwen API
// Security note: This struct contains sensitive authentication data.
// Access tokens should be handled securely and never logged in plain text.
type Credentials struct {
	AccessToken  string `json:"access_token"`  // Sensitive: never log this value
	TokenType    string `json:"token_type"`    // e.g., "Bearer"
	RefreshToken string `json:"refresh_token"` // Sensitive: never log this value
	ExpiryDate   int64  `json:"expiry_date"`   // Unix timestamp in milliseconds
	ResourceURL  string `json:"resource_url,omitempty"`
}

// IsExpired checks if the credentials have expired with a buffer for token refresh
func (c *Credentials) IsExpired() bool {
	if c == nil || c.ExpiryDate == 0 {
		return true
	}
	// Add 5-minute buffer for token refresh
	buffer := time.Now().Add(5 * time.Minute).UnixMilli()
	return c.ExpiryDate <= buffer
}

// GetToken returns the formatted authorization token
func (c *Credentials) GetToken() string {
	if c == nil || c.AccessToken == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", c.TokenType, c.AccessToken)
}

// Sanitize returns a safe representation of credentials for logging (without sensitive data)
func (c *Credentials) Sanitize() map[string]interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"token_type":   c.TokenType,
		"expiry_date":  c.ExpiryDate,
		"resource_url": c.ResourceURL,
		"has_token":    c.AccessToken != "",
		"is_expired":   c.IsExpired(),
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
	QWENOAuthScope         string `json:"qwen_oauth_scope" env:"QWEN_OAUTH_SCOPE" env-default:"user:read"`
	QWENOAuthDeviceAuthURL string `json:"qwen_oauth_device_auth_url" env:"QWEN_OAUTH_DEVICE_AUTH_URL" env-required:"true"`

	// Storage and file paths
	QWENDir string `json:"qwen_dir" env:"QWEN_DIR" env-default:"./data"`

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

// Validate checks if the configuration is valid and has all required fields
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	if c.QWENOAuthBaseURL == "" {
		return fmt.Errorf("qwen_oauth_base_url is required")
	}
	if c.QWENOAuthClientID == "" {
		return fmt.Errorf("qwen_oauth_client_id is required")
	}
	if c.QWENOAuthDeviceAuthURL == "" {
		return fmt.Errorf("qwen_oauth_device_auth_url is required")
	}
	if c.APIBaseURL == "" {
		return fmt.Errorf("api_base_url is required")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate rate limiting values
	if c.RateLimitRequestsPerSecond <= 0 {
		return fmt.Errorf("rate_limit_rps must be positive")
	}
	if c.RateLimitBurst <= 0 {
		return fmt.Errorf("rate_limit_burst must be positive")
	}

	return nil
}

// GetServerAddress returns the full server address for HTTP server configuration
func (c *Config) GetServerAddress() string {
	if c == nil {
		return ":8080"
	}
	return fmt.Sprintf("%s:%s", c.ServerHost, c.ServerPort)
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

// Validate checks if the completion request is valid
func (r *CompletionRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("completion request is nil")
	}

	if r.Prompt == nil || r.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}

	if r.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if r.Temperature < 0 || r.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if r.TopP < 0 || r.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1")
	}

	return nil
}

// ChatCompletionRequest represents a chat completion request with validation
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
	ToolChoice       interface{}     `json:"tool_choice,omitempty"` // string or ToolChoice
	ReasoningEffort  string          `json:"reasoning_effort,omitempty" validate:"omitempty,oneof=low medium high"`
	IncludeReasoning bool            `json:"include_reasoning,omitempty"`
	StreamOptions    *StreamOptions  `json:"stream_options,omitempty"`
}

// Validate checks if the chat completion request is valid
func (r *ChatCompletionRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("chat completion request is nil")
	}

	if len(r.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}

	// Validate each message
	for i, msg := range r.Messages {
		if err := msg.Validate(); err != nil {
			return fmt.Errorf("message %d is invalid: %w", i, err)
		}
	}

	if r.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if r.Temperature < 0 || r.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if r.TopP < 0 || r.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1")
	}

	return nil
}

// ChatMessage represents a message in chat completion with validation
type ChatMessage struct {
	Role      string      `json:"role" validate:"required,oneof=system user assistant tool"`
	Content   interface{} `json:"content" validate:"required"` // Can be string or []ContentBlock
	ToolCalls []ToolCall  `json:"tool_calls,omitempty" validate:"omitempty,dive"`
}

// Validate checks if the chat message is valid
func (m *ChatMessage) Validate() error {
	if m == nil {
		return fmt.Errorf("chat message is nil")
	}

	if m.Role == "" {
		return fmt.Errorf("role is required")
	}

	validRoles := map[string]bool{
		"system":    true,
		"user":      true,
		"assistant": true,
		"tool":      true,
	}

	if !validRoles[m.Role] {
		return fmt.Errorf("invalid role: %s (must be system, user, assistant, or tool)", m.Role)
	}

	if m.Content == nil || m.Content == "" {
		return fmt.Errorf("content is required")
	}

	return nil
}

// ToolCall represents a tool call in a message
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
