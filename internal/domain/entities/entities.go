// Package entities contains domain entities and data structures for the Qwen API proxy.
// This package defines the core business objects and their validation rules.
package entities

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

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

	// Streaming configuration
	StreamingMaxErrors           int           `json:"streaming_max_errors" env:"STREAMING_MAX_ERRORS" env-default:"10"`
	StreamingBufferSize          int           `json:"streaming_buffer_size" env:"STREAMING_BUFFER_SIZE" env-default:"4096"`
	StreamingTimeoutSeconds      int           `json:"streaming_timeout_seconds" env:"STREAMING_TIMEOUT_SECONDS" env-default:"900"`
	StreamingWindowSize          int           `json:"streaming_window_size" env:"STREAMING_WINDOW_SIZE" env-default:"5"`
	StreamingSimilarityThreshold float64       `json:"streaming_similarity_threshold" env:"STREAMING_SIMILARITY_THRESHOLD" env-default:"0.8"`
	StreamingTimeWindow          time.Duration `json:"streaming_time_window" env:"STREAMING_TIME_WINDOW" env-default:"2s"`
	StreamingMinConfidence       float64       `json:"streaming_min_confidence" env:"STREAMING_MIN_CONFIDENCE" env-default:"0.7"`

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

// StreamingState represents the current state of stream processing
type StreamingState int

const (
	StateInitial StreamingState = iota
	StateStuttering
	StateNormalFlow
	StateRecovering
	StateTerminating
)

func (s StreamingState) String() string {
	switch s {
	case StateInitial:
		return "Initial"
	case StateStuttering:
		return "Stuttering"
	case StateNormalFlow:
		return "NormalFlow"
	case StateRecovering:
		return "Recovering"
	case StateTerminating:
		return "Terminating"
	default:
		return "Unknown"
	}
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (cs CircuitState) String() string {
	switch cs {
	case CircuitClosed:
		return "Closed"
	case CircuitOpen:
		return "Open"
	case CircuitHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// StutteringAction represents the recommended action based on stuttering analysis
type StutteringAction int

const (
	StutteringActionBuffer StutteringAction = iota
	StutteringActionFlush
	StutteringActionForward
	StutteringActionWait
)

func (sa StutteringAction) String() string {
	switch sa {
	case StutteringActionBuffer:
		return "Buffer"
	case StutteringActionFlush:
		return "Flush"
	case StutteringActionForward:
		return "Forward"
	case StutteringActionWait:
		return "Wait"
	default:
		return "Unknown"
	}
}

// ChunkType represents the type of chunk received
type ChunkType int

const (
	ChunkTypeData ChunkType = iota
	ChunkTypeDone
	ChunkTypeMalformed
	ChunkTypeEmpty
	ChunkTypeUnknown
)

func (c ChunkType) String() string {
	switch c {
	case ChunkTypeData:
		return "Data"
	case ChunkTypeDone:
		return "Done"
	case ChunkTypeMalformed:
		return "Malformed"
	case ChunkTypeEmpty:
		return "Empty"
	case ChunkTypeUnknown:
		return "Unknown"
	default:
		return "Invalid"
	}
}

// ContentChunk represents a chunk of content with metadata
type ContentChunk struct {
	Content    string
	Timestamp  time.Time
	Length     int
	TokenCount int
	ChunkIndex int
}

// StutteringResult represents the result of stuttering analysis
type StutteringResult struct {
	IsStuttering bool
	Confidence   float64
	Reason       string
	ShouldBuffer bool
	ShouldFlush  bool
	NextAction   StutteringAction
}

// ParsedChunk represents a parsed chunk with metadata
type ParsedChunk struct {
	Type        ChunkType
	RawLine     string
	Content     string
	IsValid     bool
	Error       error
	Metadata    map[string]interface{}
	ParsedAt    time.Time
	HasContent  bool
	ContentText string
}

// StreamState holds the current state of stream processing
type StreamState struct {
	Current        StreamingState
	IsStuttering   bool
	Buffer         string
	ChunkCount     int
	ErrorCount     int
	LastValidChunk time.Time
	StartTime      time.Time
}

// CircuitBreaker implements the circuit breaker pattern for upstream resilience
type CircuitBreaker struct {
	MaxFailures      int
	ResetTimeout     time.Duration
	State            CircuitState
	FailureCount     int
	SuccessCount     int
	LastFailureTime  time.Time
	LastSuccessTime  time.Time
	HalfOpenMaxTries int
	HalfOpenTries    int
}

// StreamingConfig holds configuration for the streaming handler
type StreamingConfig struct {
	MaxErrors           int
	BufferSize          int
	TimeoutSeconds      int
	WindowSize          int
	SimilarityThreshold float64
	TimeWindow          time.Duration
	MinConfidence       float64
}

// StutteringDetector provides advanced stuttering detection
type StutteringDetector struct {
	WindowSize          int
	SimilarityThreshold float64
	ContentHistory      []ContentChunk
	TimeWindow          time.Duration
	MinConfidence       float64
}

// StreamingMetrics holds metrics for streaming operations
type StreamingMetrics struct {
	ChunksProcessed   int
	ErrorsEncountered int
	Duration          time.Duration
	StateTransitions  []StateTransition
	StutteringEvents  []StutteringEvent
}

// StateTransition represents a state transition with metadata
type StateTransition struct {
	From      StreamingState
	To        StreamingState
	Reason    string
	Timestamp time.Time
}

// StutteringEvent represents a stuttering detection event
type StutteringEvent struct {
	Detected   bool
	Confidence float64
	Reason     string
	Timestamp  time.Time
}

// IncrementChunk increments the chunk counter and updates last valid chunk time
func (s *StreamState) IncrementChunk() {
	s.ChunkCount++
	s.LastValidChunk = time.Now()
}

// IncrementError increments the error counter
func (s *StreamState) IncrementError() {
	s.ErrorCount++
}

// TransitionTo changes the state and logs the transition (for use by processors)
func (s *StreamState) TransitionTo(newState StreamingState, reason string) {
	s.Current = newState
	// Note: Logging is handled by the processor
}

// AnalyzeStuttering performs comprehensive stuttering analysis
func (sd *StutteringDetector) AnalyzeStuttering(current, previous string) StutteringResult {
	now := time.Now()

	// Create content chunks
	currentChunk := ContentChunk{
		Content:    current,
		Timestamp:  now,
		Length:     len(current),
		TokenCount: sd.estimateTokenCount(current),
		ChunkIndex: len(sd.ContentHistory),
	}

	// Add to history
	sd.addToHistory(currentChunk)

	// If this is the first chunk, it's always stuttering
	if len(sd.ContentHistory) <= 1 {
		return StutteringResult{
			IsStuttering: true,
			Confidence:   1.0,
			Reason:       "first chunk",
			ShouldBuffer: true,
			NextAction:   StutteringActionBuffer,
		}
	}

	// Multi-factor analysis
	prefixMatch := sd.analyzePrefixMatch(current, previous)
	lengthProgression := sd.analyzeLengthProgression()
	timingPattern := sd.analyzeTimingPattern()
	contentSimilarity := sd.analyzeContentSimilarity(current, previous)

	// Weighted confidence scoring
	confidence := (prefixMatch*0.3 + lengthProgression*0.3 + timingPattern*0.2 + contentSimilarity*0.2)

	isStuttering := confidence >= sd.MinConfidence

	result := StutteringResult{
		IsStuttering: isStuttering,
		Confidence:   confidence,
		Reason:       sd.buildReasonString(prefixMatch, lengthProgression, timingPattern, contentSimilarity),
		ShouldBuffer: isStuttering,
		ShouldFlush:  !isStuttering && len(sd.ContentHistory) > 1,
	}

	if isStuttering {
		result.NextAction = StutteringActionBuffer
	} else if result.ShouldFlush {
		result.NextAction = StutteringActionFlush
	} else {
		result.NextAction = StutteringActionForward
	}

	return result
}

// analyzePrefixMatch checks if current content starts with previous content
func (sd *StutteringDetector) analyzePrefixMatch(current, previous string) float64 {
	if previous == "" || current == "" {
		return 0.0
	}

	// Check if current starts with previous (indicating stuttering)
	if strings.HasPrefix(current, previous) {
		overlap := float64(len(previous)) / float64(len(current))
		return overlap
	}

	return 0.0
}

// analyzeLengthProgression checks if content length is increasing appropriately
func (sd *StutteringDetector) analyzeLengthProgression() float64 {
	if len(sd.ContentHistory) < 2 {
		return 0.0
	}

	recent := sd.ContentHistory[len(sd.ContentHistory)-2:]
	totalLength := 0
	for _, chunk := range recent {
		totalLength += chunk.Length
	}

	// Expect some length progression
	avgLength := float64(totalLength) / float64(len(recent))
	return math.Min(1.0, avgLength/10.0) // Normalize
}

// analyzeTimingPattern analyzes chunk arrival timing
func (sd *StutteringDetector) analyzeTimingPattern() float64 {
	if len(sd.ContentHistory) < 3 {
		return 0.0
	}

	recent := sd.ContentHistory[len(sd.ContentHistory)-3:]
	timeDiffs := make([]time.Duration, 0, len(recent)-1)

	for i := 1; i < len(recent); i++ {
		diff := recent[i].Timestamp.Sub(recent[i-1].Timestamp)
		timeDiffs = append(timeDiffs, diff)
	}

	// Check for irregular timing (potential stuttering)
	irregularCount := 0
	for _, diff := range timeDiffs {
		if diff > sd.TimeWindow/2 {
			irregularCount++
		}
	}

	return float64(irregularCount) / float64(len(timeDiffs))
}

// analyzeContentSimilarity uses simple Levenshtein-like distance
func (sd *StutteringDetector) analyzeContentSimilarity(current, previous string) float64 {
	if previous == "" {
		return 0.0
	}

	// Simple similarity based on common prefix length
	minLen := math.Min(float64(len(current)), float64(len(previous)))
	commonPrefix := 0
	for i := 0; i < int(minLen); i++ {
		if current[i] == previous[i] {
			commonPrefix++
		} else {
			break
		}
	}

	return float64(commonPrefix) / math.Max(float64(len(current)), float64(len(previous)))
}

// buildReasonString builds a reason string from analysis factors
func (sd *StutteringDetector) buildReasonString(prefix, length, timing, similarity float64) string {
	reasons := []string{}
	if prefix > 0.5 {
		reasons = append(reasons, "prefix match")
	}
	if length < 0.3 {
		reasons = append(reasons, "poor length progression")
	}
	if timing > 0.5 {
		reasons = append(reasons, "irregular timing")
	}
	if similarity > 0.8 {
		reasons = append(reasons, "high content similarity")
	}

	if len(reasons) == 0 {
		return "normal pattern"
	}
	return strings.Join(reasons, ", ")
}

// addToHistory adds a chunk to the content history
func (sd *StutteringDetector) addToHistory(chunk ContentChunk) {
	sd.ContentHistory = append(sd.ContentHistory, chunk)

	// Maintain window size
	if len(sd.ContentHistory) > sd.WindowSize {
		sd.ContentHistory = sd.ContentHistory[1:]
	}
}

// estimateTokenCount provides a rough token count estimate
func (sd *StutteringDetector) estimateTokenCount(content string) int {
	// Rough estimation: ~4 characters per token
	return len(content) / 4
}

// Custom Error Types for better error handling and classification

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeNetwork        ErrorType = "network"
	ErrorTypeRateLimit      ErrorType = "rate_limit"
	ErrorTypeConfiguration  ErrorType = "configuration"
	ErrorTypeInternal       ErrorType = "internal"
	ErrorTypeExternal       ErrorType = "external"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeStreaming      ErrorType = "streaming"
)

// AppError represents a structured application error with context
type AppError struct {
	Type       ErrorType              `json:"type"`
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Underlying error                  `json:"-"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Type, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Underlying
}

// NewAuthError creates a new authentication error
func NewAuthError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeAuthentication,
		Code:       "AUTH_FAILED",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Code:       "VALIDATION_FAILED",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeNetwork,
		Code:       "NETWORK_ERROR",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(message string, retryAfter int) *AppError {
	return &AppError{
		Type:      ErrorTypeRateLimit,
		Code:      "RATE_LIMIT_EXCEEDED",
		Message:   message,
		Timestamp: time.Now(),
		Context: map[string]interface{}{
			"retry_after": retryAfter,
		},
	}
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeConfiguration,
		Code:       "CONFIG_ERROR",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// NewStreamingError creates a new streaming error
func NewStreamingError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeStreaming,
		Code:       "STREAMING_ERROR",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, details string, underlying error) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Code:       "INTERNAL_ERROR",
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Underlying: underlying,
	}
}

// WithRequestID adds request ID to the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithContext adds additional context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// IsAuthError checks if the error is an authentication error
func IsAuthError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeAuthentication
	}
	return false
}

// IsRateLimitError checks if the error is a rate limit error
func IsRateLimitError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeRateLimit
	}
	return false
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeValidation
	}
	return false
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeNetwork
	}
	return false
}

// IsStreamingError checks if the error is a streaming error
func IsStreamingError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeStreaming
	}
	return false
}

// GetErrorType returns the error type from an error, or ErrorTypeInternal if not an AppError
func GetErrorType(err error) ErrorType {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}
	return ErrorTypeInternal
}
