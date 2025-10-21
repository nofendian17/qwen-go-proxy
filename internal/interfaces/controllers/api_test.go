package controllers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Add the missing tests to the existing test file
func TestStreamChatCompletionsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Test successful streaming
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
		Stream: true,
	}

	// Mock the StreamChatCompletions call to return no error
	mockProxy.EXPECT().StreamChatCompletions(req, gomock.Any()).Return(nil).Times(1)

	// Create a request to the handler
	httpReq := httptest.NewRequest("POST", "/chat/completions", nil)
	rec := httptest.NewRecorder()

	// Call the handler directly (it requires the request and the chat completion request)
	controller.StreamChatCompletionsHandler(rec, httpReq, req)

	// The response should be handled by the proxy use case in a streaming fashion
	// For this test, we're mainly checking that the function can be called without error
	// The important part is that the mock expectation was met.
}

func TestStreamChatCompletionsHandler_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
		Stream: true,
	}

	// Mock the StreamChatCompletions call to return an error
	mockProxy.EXPECT().StreamChatCompletions(req, gomock.Any()).Return(assert.AnError).Times(1)

	// Create a request to the handler
	httpReq := httptest.NewRequest("POST", "/chat/completions", nil)
	rec := httptest.NewRecorder()

	// Call the handler directly
	controller.StreamChatCompletionsHandler(rec, httpReq, req)

	// The response status code will depend on how the proxy handles the streaming.
	// The important part is that the mock expectation was met and error was logged.
}

func TestExtractTextContent(t *testing.T) {
	// Test with string content
	strContent := "Hello, world!"
	result := extractTextContent(strContent)
	assert.Equal(t, "Hello, world!", result)

	// Test with nil content
	result = extractTextContent(nil)
	assert.Equal(t, "", result)

	// Test with empty interface{}
	var empty interface{}
	result = extractTextContent(empty)
	assert.Equal(t, "", result)
}

func TestExtractTextContent_ContentBlocks(t *testing.T) {
	// Test with content blocks array
	contentBlocks := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "Hello",
		},
		map[string]interface{}{
			"type": "image_url",
			"url":  "http://example.com/image.jpg",
		},
		map[string]interface{}{
			"type": "text",
			"text": " world",
		},
	}

	result := extractTextContent(contentBlocks)
	assert.Equal(t, "Hello world", result)
}

func TestExtractTextContent_InvalidContentBlocks(t *testing.T) {
	// Test with invalid content blocks (not maps)
	invalidBlocks := []interface{}{
		"just a string",
		42,
		map[string]interface{}{
			"type": "text",
			"text": "valid text",
		},
	}

	result := extractTextContent(invalidBlocks)
	assert.Equal(t, "valid text", result)
}

func TestExtractFloat64(t *testing.T) {
	// Test with valid float64
	val, ok := extractFloat64(3.14)
	assert.True(t, ok)
	assert.Equal(t, 3.14, val)

	// Test with invalid type
	val, ok = extractFloat64("not a float")
	assert.False(t, ok)
	assert.Equal(t, 0.0, val)

	// Test with integer (should be converted to float64 by JSON unmarshaling)
	val, ok = extractFloat64(float64(42))
	assert.True(t, ok)
	assert.Equal(t, 42.0, val)
}

func TestExtractString(t *testing.T) {
	// Test with valid string
	str, ok := extractString("hello")
	assert.True(t, ok)
	assert.Equal(t, "hello", str)

	// Test with invalid type
	str, ok = extractString(123)
	assert.False(t, ok)
	assert.Equal(t, "", str)

	// Test with empty string
	str, ok = extractString("")
	assert.True(t, ok)
	assert.Equal(t, "", str)
}

func TestExtractBool(t *testing.T) {
	// Test with valid bool true
	b := extractBool(true)
	assert.True(t, b)

	// Test with valid bool false
	b = extractBool(false)
	assert.False(t, b)

	// Test with invalid type (should default to false)
	b = extractBool("not a bool")
	assert.False(t, b)

	// Test with nil
	b = extractBool(nil)
	assert.False(t, b)
}

func TestCopyCompletionParameters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a request body with various parameters
	body := map[string]interface{}{
		"max_tokens":  100.0,  // float64 from JSON
		"temperature": 0.7,    // float64 from JSON
		"top_p":       0.9,    // float64 from JSON
		"model":       "test", // string, should be ignored by copyCompletionParameters
	}

	chatReq := &entities.ChatCompletionRequest{
		Model: "default",
	}

	controller.copyCompletionParameters(body, chatReq)

	// Check that the parameters were copied correctly
	assert.Equal(t, 100, chatReq.MaxTokens)
	assert.Equal(t, 0.7, chatReq.Temperature)
	assert.Equal(t, 0.9, chatReq.TopP)
}

func TestBuildChatRequestFromCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	body := map[string]interface{}{
		"max_tokens":  200.0,
		"temperature": 0.5,
	}

	prompt := "Write a story about a robot"
	stream := false

	chatReq := controller.buildChatRequestFromCompletion(body, prompt, stream)

	// Check that the request was built correctly
	assert.Equal(t, DefaultModel, chatReq.Model)
	assert.Len(t, chatReq.Messages, 1)
	assert.Equal(t, "user", chatReq.Messages[0].Role)
	assert.Equal(t, prompt, chatReq.Messages[0].Content)
	assert.Equal(t, 200, chatReq.MaxTokens)
	assert.Equal(t, 0.5, chatReq.Temperature)
	assert.Equal(t, false, chatReq.Stream)
}

func TestBuildCompletionResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a sample chat completion response
	chatResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "Hello, this is a test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	response := controller.buildCompletionResponse(chatResponse)

	// Check the structure of the completion response
	assert.Equal(t, "test-id", response["id"])
	assert.Equal(t, ObjectTextCompletion, response["object"])
	assert.Equal(t, int64(1234567890), response["created"])
	assert.Equal(t, "test-model", response["model"])

	// Check choices
	choices := response["choices"].([]map[string]interface{})
	assert.Len(t, choices, 1)
	assert.Equal(t, "Hello, this is a test response", choices[0]["text"])
	assert.Equal(t, 0, choices[0]["index"])
	assert.Equal(t, "stop", choices[0]["finish_reason"])

	// Check usage
	usage := response["usage"].(*entities.Usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)
	assert.Equal(t, 30, usage.TotalTokens)
}

func TestSendErrorResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	controller.sendErrorResponse(rec, req, 400, "invalid_request", "Invalid request")

	assert.Equal(t, 400, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
	assert.Contains(t, rec.Body.String(), "Invalid request")
}

func TestSendValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	controller.sendValidationError(rec, req, "Validation error")

	assert.Equal(t, 400, rec.Code) // StatusBadRequest
	assert.Contains(t, rec.Body.String(), "invalid_request_error")
	assert.Contains(t, rec.Body.String(), "Validation error")
}

func TestSendInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	controller.sendInternalError(rec, req, assert.AnError)

	assert.Equal(t, 500, rec.Code) // StatusInternalServerError
	assert.Contains(t, rec.Body.String(), "internal_error")
	assert.Contains(t, rec.Body.String(), "An internal error occurred")
}

func TestValidateJSONRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Test valid JSON
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"valid": "json"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	var target map[string]interface{}
	result := controller.validateJSONRequest(rec, req, &target)

	assert.True(t, result)
	assert.Equal(t, "json", target["valid"])
}

func TestValidateJSONRequest_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"invalid": json}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	var target map[string]interface{}
	result := controller.validateJSONRequest(rec, req, &target)

	assert.False(t, result)
	assert.Equal(t, 400, rec.Code) // Should return bad request
}

func TestOpenAIHealthHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	controller.OpenAIHealthHandler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "healthy")
}

func TestAuthenticateHandler_UserAuthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	// Mock the CheckAuthentication to return valid credentials
	mockProxy.EXPECT().CheckAuthentication().Return(&entities.Credentials{
		ResourceURL: "https://api.example.com",
	}, nil)

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/auth", nil)
	rec := httptest.NewRecorder()

	controller.AuthenticateHandler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "authenticated")
	assert.Contains(t, rec.Body.String(), "true")
}

func TestOpenAIModelsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	// Mock the GetModels to return some models
	expectedModels := []*entities.ModelInfo{
		{
			ID:      "model-1",
			Object:  "model",
			Created: 1234567890,
			OwnedBy: "test",
		},
	}
	mockProxy.EXPECT().GetModels().Return(expectedModels, nil)

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/models", nil)
	rec := httptest.NewRecorder()

	controller.OpenAIModelsHandler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "model-1")
	assert.Contains(t, rec.Body.String(), ObjectList)
}

func TestOpenAIModelsHandler_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	// Mock the GetModels to return an error
	mockProxy.EXPECT().GetModels().Return(nil, assert.AnError)

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	req := httptest.NewRequest("GET", "/models", nil)
	rec := httptest.NewRecorder()

	controller.OpenAIModelsHandler(rec, req)

	assert.Equal(t, 500, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrorTypeInternal)
	assert.Contains(t, rec.Body.String(), ErrMsgInternalError)
}

func TestOpenAICompletionsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Valid request body
	jsonBody := `{
		"prompt": "Write a test",
		"model": "test-model"
	}`

	// Mock the ChatCompletions call to return a response
	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "text_completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mockProxy.EXPECT().ChatCompletions(gomock.Any()).Return(expectedResponse, nil)

	req := httptest.NewRequest("POST", "/completions", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.OpenAICompletionsHandler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "test response")
}

func TestOpenAICompletionsHandler_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Invalid JSON request body
	invalidJSON := `{"invalid": json}`

	req := httptest.NewRequest("POST", "/completions", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.OpenAICompletionsHandler(rec, req)

	// Should return 400 Bad Request for invalid JSON
	assert.Equal(t, 400, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrMsgInvalidJSON)
}

func TestOpenAICompletionsHandler_MissingPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Valid JSON but missing prompt
	jsonBody := `{"model": "test-model"}`

	req := httptest.NewRequest("POST", "/completions", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.OpenAICompletionsHandler(rec, req)

	// Should return 400 Bad Request for missing prompt
	assert.Equal(t, 400, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrMsgMissingPrompt)
}

func TestHandleNonStreamingCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a chat completion request
	chatReq := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test prompt",
			},
		},
		Stream: false,
	}

	// Mock the ChatCompletions call to return a response
	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mockProxy.EXPECT().ChatCompletions(chatReq).Return(expectedResponse, nil)

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	controller.handleNonStreamingCompletion(rec, req, chatReq)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "test response")
}

func TestHandleNonStreamingCompletion_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a chat completion request
	chatReq := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test prompt",
			},
		},
		Stream: false,
	}

	// Mock the ChatCompletions call to return an error
	mockProxy.EXPECT().ChatCompletions(chatReq).Return(nil, assert.AnError)

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	controller.handleNonStreamingCompletion(rec, req, chatReq)

	assert.Equal(t, 500, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrorTypeInternal)
	assert.Contains(t, rec.Body.String(), ErrMsgInternalError)
}

func TestHandleNonStreamingCompletion_EmptyChoices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a chat completion request
	chatReq := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test prompt",
			},
		},
		Stream: false,
	}

	// Mock the ChatCompletions call to return a response with empty choices
	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{}, // Empty choices
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mockProxy.EXPECT().ChatCompletions(chatReq).Return(expectedResponse, nil)

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	controller.handleNonStreamingCompletion(rec, req, chatReq)

	// Should return 500 Internal Server Error for empty choices
	assert.Equal(t, 500, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrorTypeInternal)
	assert.Contains(t, rec.Body.String(), ErrMsgUnexpectedFormat)
}

func TestChatCompletionsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Valid request body
	jsonBody := `{
		"model": "test-model",
		"messages": [
			{
				"role": "user",
				"content": "test message"
			}
		],
		"stream": false
	}`

	// Mock the ChatCompletions call to return a response
	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mockProxy.EXPECT().ChatCompletions(gomock.Any()).Return(expectedResponse, nil)

	req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.ChatCompletionsHandler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "test response")
}

func TestChatCompletionsHandler_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Invalid JSON request body
	invalidJSON := `{"invalid": json}`

	req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.ChatCompletionsHandler(rec, req)

	// Should return 400 Bad Request for invalid JSON
	assert.Equal(t, 400, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrMsgInvalidJSON)
}

func TestHandleNonStreamingChatCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a chat completion request
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
		Stream: false,
	}

	// Mock the ChatCompletions call to return a response
	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mockProxy.EXPECT().ChatCompletions(req).Return(expectedResponse, nil)

	httpReq := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	controller.handleNonStreamingChatCompletion(rec, httpReq, req)

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "test response")
}

func TestHandleNonStreamingChatCompletion_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProxy := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := logging.NewLogger("info")

	controller := NewAPIController(mockProxy, &logging.Logger{Logger: logger})

	// Create a chat completion request
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
		Stream: false,
	}

	// Mock the ChatCompletions call to return an error
	mockProxy.EXPECT().ChatCompletions(req).Return(nil, assert.AnError)

	httpReq := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	controller.handleNonStreamingChatCompletion(rec, httpReq, req)

	assert.Equal(t, 500, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrorTypeInternal)
	assert.Contains(t, rec.Body.String(), ErrMsgInternalError)
}
