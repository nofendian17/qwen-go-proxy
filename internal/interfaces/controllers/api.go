package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/usecases/proxy"
)

// APIController handles API requests
type APIController struct {
	proxyUseCase *proxy.ProxyUseCase
	logger       *logging.Logger
}

// NewAPIController creates a new API controller
func NewAPIController(proxyUseCase *proxy.ProxyUseCase, logger *logging.Logger) *APIController {
	return &APIController{
		proxyUseCase: proxyUseCase,
		logger:       logger,
	}
}

// OpenAIHealthHandler returns health check in OpenAI-compatible format
func (ctrl *APIController) OpenAIHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// AuthenticateHandler checks authentication status and initiates device auth if needed
func (ctrl *APIController) AuthenticateHandler(c *gin.Context) {
	// First check if user is already authenticated
	credentials, err := ctrl.proxyUseCase.CheckAuthentication()
	if err == nil && credentials != nil {
		// User is authenticated
		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"message":       "User is authenticated",
			"resource_url":  credentials.ResourceURL,
		})
		return
	}

	// User is not authenticated, initiate device authentication
	err = ctrl.proxyUseCase.AuthenticateManually()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"authenticated": false,
			"error": gin.H{
				"message": "Authentication failed",
				"type":    "authentication_error",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": false,
		"message":       "Device authentication initiated. Please complete the authentication process in your browser.",
		"status":        "authentication_initiated",
	})
}

// OpenAIModelsHandler returns models in OpenAI-compatible format
func (ctrl *APIController) OpenAIModelsHandler(c *gin.Context) {
	models := ctrl.proxyUseCase.GetModels()

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

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openAIModels,
	})
}

// OpenAICompletionsHandler handles OpenAI-style completions (non-chat)
func (ctrl *APIController) OpenAICompletionsHandler(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid JSON", "type": "invalid_request_error"}})
		return
	}

	// Convert legacy completion to chat completion format
	prompt, ok := body["prompt"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Missing prompt field", "type": "invalid_request_error"}})
		return
	}

	stream, _ := body["stream"].(bool)

	// Create chat completion body
	chatReq := &entities.ChatCompletionRequest{
		Model: "qwen3-coder-plus",
		Messages: []entities.ChatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: stream,
	}

	// Copy other relevant fields
	if maxTokens, ok := body["max_tokens"]; ok {
		if mt, ok := maxTokens.(float64); ok {
			chatReq.MaxTokens = int(mt)
		}
	}
	if temperature, ok := body["temperature"]; ok {
		if t, ok := temperature.(float64); ok {
			chatReq.Temperature = t
		}
	}
	if topP, ok := body["top_p"]; ok {
		if tp, ok := topP.(float64); ok {
			chatReq.TopP = tp
		}
	}

	if stream {
		ctrl.StreamChatCompletionsHandler(c, chatReq)
		return
	}

	// Handle non-streaming
	response, err := ctrl.proxyUseCase.ChatCompletions(chatReq)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}

	// Convert chat completion response to completion response format
	if len(response.Choices) > 0 {
		completionResponse := gin.H{
			"id":      response.ID,
			"object":  "text_completion",
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
		c.JSON(http.StatusOK, completionResponse)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Unexpected response format", "type": "internal_error"}})
	}
}

// ChatCompletionsHandler handles chat completion requests
func (ctrl *APIController) ChatCompletionsHandler(c *gin.Context) {
	var req entities.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Log the actual error for debugging
		ctrl.logger.Error("Failed to parse JSON in chat completions", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid JSON", "type": "invalid_request_error"}})
		return
	}

	if req.Stream {
		ctrl.StreamChatCompletionsHandler(c, &req)
		return
	}

	// Handle non-streaming
	response, err := ctrl.proxyUseCase.ChatCompletions(&req)
	if err != nil {
		ctrl.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// StreamChatCompletionsHandler handles streaming chat completion requests
func (ctrl *APIController) StreamChatCompletionsHandler(c *gin.Context, req *entities.ChatCompletionRequest) {
	err := ctrl.proxyUseCase.StreamChatCompletions(req, c.Writer)
	if err != nil {
		// For streaming, we can't send JSON error after headers are set
		// The error would have been logged in the use case
		return
	}
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
				if blockType, ok := blockMap["type"].(string); ok && blockType == "text" {
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

// handleError handles errors and returns appropriate HTTP responses
func (ctrl *APIController) handleError(c *gin.Context, err error) {
	// Map different error types to OpenAI-compatible error codes
	statusCode := http.StatusInternalServerError
	errorType := "internal_error"
	message := "An internal error occurred"

	// You can add more specific error handling here based on error types
	// For example:
	// if strings.Contains(err.Error(), "authentication") {
	//     statusCode = http.StatusUnauthorized
	//     errorType = "authentication_error"
	//     message = "Authentication failed"
	// }

	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errorType,
			"code":    statusCode,
		},
	})
}
