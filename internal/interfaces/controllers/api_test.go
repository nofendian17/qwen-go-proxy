package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/mocks"
)

// testLogger is a simple logger implementation for testing that does nothing
type testLogger struct{}

func (t *testLogger) Debug(msg string, args ...any) {}
func (t *testLogger) Info(msg string, args ...any)  {}
func (t *testLogger) Warn(msg string, args ...any)  {}
func (t *testLogger) Error(msg string, args ...any) {}

func TestNewAPIController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Verify controller is created
	assert.NotNil(t, controller)
	assert.Equal(t, proxyCtrl, controller.proxyUseCase)
	assert.Equal(t, logger, controller.logger)
}

func TestOpenAIHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.GET("/health", controller.OpenAIHealthHandler)

	// Create request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
}

func TestAuthenticateHandler_UserAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	credentials := &entities.Credentials{
		ResourceURL: "https://example.com/resource",
	}
	proxyCtrl.EXPECT().CheckAuthentication().Return(credentials, nil)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.GET("/auth", controller.AuthenticateHandler)

	// Create request
	req := httptest.NewRequest("GET", "/auth", nil)
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"authenticated":true`)
	assert.Contains(t, w.Body.String(), `"message":"User is authenticated"`)
	assert.Contains(t, w.Body.String(), `"resource_url":"https://example.com/resource"`)
}

func TestAuthenticateHandler_UserNotAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	proxyCtrl.EXPECT().CheckAuthentication().Return(nil, assert.AnError)
	proxyCtrl.EXPECT().AuthenticateManually().Return(nil)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.GET("/auth", controller.AuthenticateHandler)

	// Create request
	req := httptest.NewRequest("GET", "/auth", nil)
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"authenticated":false`)
	assert.Contains(t, w.Body.String(), `"message":"Device authentication initiated. Please complete the authentication process in your browser."`)
	assert.Contains(t, w.Body.String(), `"status":"authentication_initiated"`)
}

func TestAuthenticateHandler_AuthenticationFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	proxyCtrl.EXPECT().CheckAuthentication().Return(nil, assert.AnError)
	proxyCtrl.EXPECT().AuthenticateManually().Return(assert.AnError)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.GET("/auth", controller.AuthenticateHandler)

	// Create request
	req := httptest.NewRequest("GET", "/auth", nil)
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"authenticated":false`)
	assert.Contains(t, w.Body.String(), `"message":"Authentication failed"`)
	assert.Contains(t, w.Body.String(), `"type":"authentication_error"`)
}

func TestOpenAIModelsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	models := []*entities.ModelInfo{
		{
			ID:      "qwen3-coder-plus",
			Object:  "model",
			Created: 1754686206,
			OwnedBy: "qwen",
			Permission: []entities.ModelPermission{
				{
					ID:                 "modelperm-qwen3-coder-plus",
					Object:             "model_permission",
					Created:            1754686206,
					AllowCreateEngine:  false,
					AllowSampling:      true,
					AllowLogprobs:      true,
					AllowSearchIndices: false,
					AllowView:          true,
					AllowFineTuning:    false,
					Organization:       "*",
					Group:              nil,
					IsBlocking:         false,
				},
			},
		},
	}
	proxyCtrl.EXPECT().GetModels().Return(models)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.GET("/models", controller.OpenAIModelsHandler)

	// Create request
	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"object":"list"`)
	assert.Contains(t, w.Body.String(), `"id":"qwen3-coder-plus"`)
	assert.Contains(t, w.Body.String(), `"object":"model"`)
	assert.Contains(t, w.Body.String(), `"owned_by":"qwen"`)
}

func TestOpenAICompletionsHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/completions", controller.OpenAICompletionsHandler)

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/completions", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"Invalid JSON"`)
	assert.Contains(t, w.Body.String(), `"type":"invalid_request_error"`)
}

func TestOpenAICompletionsHandler_MissingPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/completions", controller.OpenAICompletionsHandler)

	// Create request without prompt
	reqBody := `{"model": "test-model"}`
	req := httptest.NewRequest("POST", "/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"Missing prompt field"`)
	assert.Contains(t, w.Body.String(), `"type":"invalid_request_error"`)
}

func TestOpenAICompletionsHandler_NonStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	response := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen3-coder-plus",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "Test response",
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
	proxyCtrl.EXPECT().ChatCompletions(gomock.Any()).Return(response, nil)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/completions", controller.OpenAICompletionsHandler)

	// Create request
	reqBody := `{"prompt": "Test prompt", "stream": false}`
	req := httptest.NewRequest("POST", "/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"test-id"`)
	assert.Contains(t, w.Body.String(), `"object":"text_completion"`)
	assert.Contains(t, w.Body.String(), `"text":"Test response"`)
	assert.Contains(t, w.Body.String(), `"finish_reason":"stop"`)
}

func TestChatCompletionsHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/chat/completions", controller.ChatCompletionsHandler)

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/chat/completions", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"Invalid JSON"`)
	assert.Contains(t, w.Body.String(), `"type":"invalid_request_error"`)
}

func TestChatCompletionsHandler_NonStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	response := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen3-coder-plus",
		Choices: []entities.ChatCompletionChoice{
			{
				Index: 0,
				Message: entities.ChatMessage{
					Role:    "assistant",
					Content: "Test response",
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
	proxyCtrl.EXPECT().ChatCompletions(gomock.Any()).Return(response, nil)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/chat/completions", controller.ChatCompletionsHandler)

	// Create request
	reqBody := `{
		"model": "qwen3-coder-plus",
		"messages": [{"role": "user", "content": "Test message"}],
		"stream": false
	}`
	req := httptest.NewRequest("POST", "/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"test-id"`)
	assert.Contains(t, w.Body.String(), `"object":"chat.completion"`)
	assert.Contains(t, w.Body.String(), `"content":"Test response"`)
	assert.Contains(t, w.Body.String(), `"finish_reason":"stop"`)
}

func TestChatCompletionsHandler_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	proxyCtrl := mocks.NewMockProxyUseCaseInterface(ctrl)
	logger := &testLogger{}

	// Setup mock expectations
	proxyCtrl.EXPECT().ChatCompletions(gomock.Any()).Return(nil, assert.AnError)

	// Create controller
	controller := NewAPIController(proxyCtrl, logger)

	// Create test router
	router := gin.New()
	router.POST("/chat/completions", controller.ChatCompletionsHandler)

	// Create request
	reqBody := `{
		"model": "qwen3-coder-plus",
		"messages": [{"role": "user", "content": "Test message"}],
		"stream": false
	}`
	req := httptest.NewRequest("POST", "/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"An internal error occurred"`)
	assert.Contains(t, w.Body.String(), `"type":"internal_error"`)
}

func TestExtractTextContent_String(t *testing.T) {
	content := "Test content"
	result := extractTextContent(content)
	assert.Equal(t, "Test content", result)
}

func TestExtractTextContent_ContentBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "Block 1",
		},
		map[string]interface{}{
			"type": "text",
			"text": "Block 2",
		},
	}
	result := extractTextContent(content)
	assert.Equal(t, "Block 1Block 2", result)
}

func TestExtractTextContent_Nil(t *testing.T) {
	result := extractTextContent(nil)
	assert.Equal(t, "", result)
}

func TestExtractString(t *testing.T) {
	value, ok := extractString("test")
	assert.True(t, ok)
	assert.Equal(t, "test", value)

	value, ok = extractString(123)
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestExtractFloat64(t *testing.T) {
	value, ok := extractFloat64(123.45)
	assert.True(t, ok)
	assert.Equal(t, 123.45, value)

	value, ok = extractFloat64("not a number")
	assert.False(t, ok)
	assert.Equal(t, 0.0, value)
}

func TestExtractBool(t *testing.T) {
	assert.True(t, extractBool(true))
	assert.False(t, extractBool(false))
	assert.False(t, extractBool("not a bool"))
	assert.False(t, extractBool(nil))
}
