package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/usecases/proxy"
)

// Constants for API responses and error handling
const (
	// Default model for completions
	DefaultModel = "qwen3-coder-plus"

	// OpenAI-compatible object types
	ObjectList           = "list"
	ObjectTextCompletion = "text_completion"

	// Error types
	ErrorTypeInvalidRequest = "invalid_request_error"
	ErrorTypeInternal       = "internal_error"
	ErrorTypeAuthentication = "authentication_error"

	// Error messages
	ErrMsgInvalidJSON      = "Invalid JSON"
	ErrMsgMissingPrompt    = "Missing prompt field"
	ErrMsgUnexpectedFormat = "Unexpected response format"
	ErrMsgAuthFailed       = "Authentication failed"
	ErrMsgInternalError    = "An internal error occurred"

	// Response messages
	MsgUserAuthenticated   = "User is authenticated"
	MsgAuthInitiated       = "Device authentication initiated. Please complete the authentication process in your browser."
	MsgHealthy             = "healthy"
	MsgAuthStatusInitiated = "authentication_initiated"

	// Content types
	ContentTypeText = "text"

	// HTTP status codes for common responses
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusInternalServerError = http.StatusInternalServerError
)

// APIController handles API requests
type APIController struct {
	proxyUseCase proxy.ProxyUseCaseInterface
	logger       logging.LoggerInterface
}

// NewAPIController creates a new API controller
func NewAPIController(proxyUseCase proxy.ProxyUseCaseInterface, logger logging.LoggerInterface) *APIController {
	if proxyUseCase == nil {
		panic("proxyUseCase cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	return &APIController{
		proxyUseCase: proxyUseCase,
		logger:       logger,
	}
}

// sendErrorResponse sends a standardized error response
func (ctrl *APIController) sendErrorResponse(c *gin.Context, statusCode int, errorType, message string) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}
	ctrl.logger.Error("API error response", "request_id", requestID, "status", statusCode, "type", errorType, "message", message)
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errorType,
			"code":    statusCode,
		},
	})
}

// sendValidationError sends a validation error response
func (ctrl *APIController) sendValidationError(c *gin.Context, message string) {
	ctrl.sendErrorResponse(c, StatusBadRequest, ErrorTypeInvalidRequest, message)
}

// sendInternalError sends an internal server error response
func (ctrl *APIController) sendInternalError(c *gin.Context, err error) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}
	ctrl.logger.Error("Internal server error", "request_id", requestID, "error", err)
	ctrl.sendErrorResponse(c, StatusInternalServerError, ErrorTypeInternal, ErrMsgInternalError)
}

// validateJSONRequest validates and binds JSON request
func (ctrl *APIController) validateJSONRequest(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		requestID := c.GetString("request_id")
		if requestID == "" {
			requestID = "unknown"
		}
		ctrl.logger.Error("JSON binding failed", "request_id", requestID, "error", err)
		ctrl.sendValidationError(c, ErrMsgInvalidJSON)
		return false
	}
	return true
}

// OpenAIHealthHandler returns health check in OpenAI-compatible format
func (ctrl *APIController) OpenAIHealthHandler(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}
	ctrl.logger.Debug("Health check requested", "request_id", requestID)
	c.JSON(StatusOK, gin.H{
		"status": MsgHealthy,
	})
}

// AuthenticateHandler checks authentication status and initiates device auth if needed
func (ctrl *APIController) AuthenticateHandler(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}
	ctrl.logger.Debug("Authentication check requested", "request_id", requestID)

	// First check if user is already authenticated
	credentials, err := ctrl.proxyUseCase.CheckAuthentication()
	if err == nil && credentials != nil {
		// User is authenticated
		ctrl.logger.Info("User is already authenticated", "request_id", requestID)
		c.JSON(StatusOK, gin.H{
			"authenticated": true,
			"message":       MsgUserAuthenticated,
			"resource_url":  credentials.ResourceURL,
		})
		return
	}

	// User is not authenticated, initiate device authentication
	ctrl.logger.Info("User not authenticated, initiating device authentication", "request_id", requestID)
	err = ctrl.proxyUseCase.AuthenticateManually()
	if err != nil {
		ctrl.logger.Error("Authentication initiation failed", "request_id", requestID, "error", err)
		c.JSON(StatusInternalServerError, gin.H{
			"authenticated": false,
			"error": gin.H{
				"message": ErrMsgAuthFailed,
				"type":    ErrorTypeAuthentication,
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(StatusOK, gin.H{
		"authenticated": false,
		"message":       MsgAuthInitiated,
		"status":        MsgAuthStatusInitiated,
	})
}

// OpenAIModelsHandler returns models in OpenAI-compatible format
func (ctrl *APIController) OpenAIModelsHandler(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}
	ctrl.logger.Debug("Models list requested", "request_id", requestID)

	models, err := ctrl.proxyUseCase.GetModels()
	if err != nil {
		ctrl.sendInternalError(c, err)
		return
	}
	ctrl.logger.Info("Retrieved models", "request_id", requestID, "count", len(models))

	// Convert to OpenAI format
	openAIModels := make([]gin.H, len(models))
	for i, model := range models {
		openAIModels[i] = gin.H{
			"id":         model.ID,
			"object":     model.Object,
			"created":    model.Created,
			"owned_by":   model.OwnedBy,
			"permission": model.Permission,
		}
	}

	c.JSON(StatusOK, gin.H{
		"object": ObjectList,
		"data":   openAIModels,
	})
}

// OpenAICompletionsHandler handles OpenAI-style completions (non-chat)
func (ctrl *APIController) OpenAICompletionsHandler(c *gin.Context) {
	ctrl.logger.Debug("OpenAI completions request received")

	var body map[string]interface{}
	if !ctrl.validateJSONRequest(c, &body) {
		return
	}

	prompt, ok := extractString(body["prompt"])
	if !ok {
		ctrl.sendValidationError(c, ErrMsgMissingPrompt)
		return
	}

	stream := extractBool(body["stream"])
	ctrl.logger.Info("Processing completion request", "stream", stream, "prompt_length", len(prompt))

	chatReq := ctrl.buildChatRequestFromCompletion(body, prompt, stream)

	if stream {
		ctrl.StreamChatCompletionsHandler(c, chatReq)
		return
	}

	ctrl.handleNonStreamingCompletion(c, chatReq)
}

// buildChatRequestFromCompletion converts completion request to chat completion format
func (ctrl *APIController) buildChatRequestFromCompletion(body map[string]interface{}, prompt string, stream bool) *entities.ChatCompletionRequest {
	chatReq := &entities.ChatCompletionRequest{
		Model: DefaultModel,
		Messages: []entities.ChatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: stream,
	}

	// Copy optional parameters
	ctrl.copyCompletionParameters(body, chatReq)
	return chatReq
}

// copyCompletionParameters copies relevant parameters from completion to chat request
func (ctrl *APIController) copyCompletionParameters(body map[string]interface{}, chatReq *entities.ChatCompletionRequest) {
	if maxTokens, ok := extractFloat64(body["max_tokens"]); ok {
		chatReq.MaxTokens = int(maxTokens)
	}
	if temperature, ok := extractFloat64(body["temperature"]); ok {
		chatReq.Temperature = temperature
	}
	if topP, ok := extractFloat64(body["top_p"]); ok {
		chatReq.TopP = topP
	}
}

// handleNonStreamingCompletion handles non-streaming completion responses
func (ctrl *APIController) handleNonStreamingCompletion(c *gin.Context, chatReq *entities.ChatCompletionRequest) {
	response, err := ctrl.proxyUseCase.ChatCompletions(chatReq)
	if err != nil {
		ctrl.sendInternalError(c, err)
		return
	}

	if len(response.Choices) == 0 {
		ctrl.sendErrorResponse(c, StatusInternalServerError, ErrorTypeInternal, ErrMsgUnexpectedFormat)
		return
	}

	completionResponse := ctrl.buildCompletionResponse(response)
	ctrl.logger.Info("Completion response sent", "id", response.ID, "usage", response.Usage)
	c.JSON(StatusOK, completionResponse)
}

// buildCompletionResponse converts chat completion response to completion format
func (ctrl *APIController) buildCompletionResponse(response *entities.ChatCompletionResponse) gin.H {
	return gin.H{
		"id":      response.ID,
		"object":  ObjectTextCompletion,
		"created": response.Created,
		"model":   response.Model,
		"choices": []gin.H{
			{
				"text":          extractTextContent(response.Choices[0].Message.Content),
				"index":         0,
				"logprobs":      nil,
				"finish_reason": response.Choices[0].FinishReason,
			},
		},
		"usage": response.Usage,
	}
}

// ChatCompletionsHandler handles chat completion requests
func (ctrl *APIController) ChatCompletionsHandler(c *gin.Context) {
	ctrl.logger.Debug("Chat completions request received")

	var req entities.ChatCompletionRequest
	if !ctrl.validateJSONRequest(c, &req) {
		return
	}

	ctrl.logger.Info("Processing chat completion", "model", req.Model, "stream", req.Stream, "messages", len(req.Messages))

	if req.Stream {
		ctrl.StreamChatCompletionsHandler(c, &req)
		return
	}

	ctrl.handleNonStreamingChatCompletion(c, &req)
}

// handleNonStreamingChatCompletion handles non-streaming chat completion responses
func (ctrl *APIController) handleNonStreamingChatCompletion(c *gin.Context, req *entities.ChatCompletionRequest) {
	response, err := ctrl.proxyUseCase.ChatCompletions(req)
	if err != nil {
		ctrl.sendInternalError(c, err)
		return
	}

	ctrl.logger.Info("Chat completion response sent", "id", response.ID, "usage", response.Usage)
	c.JSON(StatusOK, response)
}

// StreamChatCompletionsHandler handles streaming chat completion requests
func (ctrl *APIController) StreamChatCompletionsHandler(c *gin.Context, req *entities.ChatCompletionRequest) {
	ctrl.logger.Debug("Streaming chat completion initiated", "model", req.Model)

	err := ctrl.proxyUseCase.StreamChatCompletions(req, c.Writer)
	if err != nil {
		// For streaming, we can't send JSON error after headers are set
		// The error would have been logged in the use case
		ctrl.logger.Error("Streaming chat completion failed", "error", err)
		return
	}

	ctrl.logger.Debug("Streaming chat completion completed successfully")
}

// extractFloat64 safely extracts a float64 value from interface{}
func extractFloat64(value interface{}) (float64, bool) {
	if f, ok := value.(float64); ok {
		return f, true
	}
	return 0, false
}

// extractString safely extracts a string value from interface{}
func extractString(value interface{}) (string, bool) {
	if s, ok := value.(string); ok {
		return s, true
	}
	return "", false
}

// extractBool safely extracts a bool value from interface{} (defaulting to false)
func extractBool(value interface{}) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

// extractTextContent extracts text content from ChatMessage.Content
// Handles both string and []ContentBlock formats
func extractTextContent(content interface{}) string {
	if content == nil {
		return ""
	}

	// If it's a string, return it directly
	if str, ok := content.(string); ok {
		return str
	}

	// If it's an array of content blocks, extract text from text blocks
	if blocks, ok := content.([]interface{}); ok {
		var textParts []string
		for _, block := range blocks {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, ok := blockMap["type"].(string); ok && blockType == ContentTypeText {
					if text, ok := blockMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				}
			}
		}
		return strings.Join(textParts, "")
	}

	return ""
}
