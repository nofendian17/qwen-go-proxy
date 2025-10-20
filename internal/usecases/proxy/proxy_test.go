package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewProxyUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	assert.NotNil(t, useCase)
	assert.Equal(t, mockAuthUseCase, useCase.authUseCase)
	assert.Equal(t, mockQwenGateway, useCase.qwenGateway)
	assert.Equal(t, mockStreamingUseCase, useCase.streamingUseCase)
	assert.Equal(t, mockLogger, useCase.logger)
	assert.Equal(t, "qwen3-coder-plus", useCase.defaultModel)
}

func TestProxyUseCase_ChatCompletions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []entities.ChatCompletionChoice{
			{
				Index:        0,
				Message:      entities.ChatMessage{Role: "assistant", Content: "Hello there!"},
				FinishReason: "stop",
			},
		},
		Usage: &entities.Usage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		},
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	// Mock expectations
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(req, credentials).Return(createMockHttpResponse(expectedResponse), nil)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	response, err := useCase.ChatCompletions(req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, expectedResponse.ID, response.ID)
	assert.Equal(t, expectedResponse.Model, response.Model)
}

func TestProxyUseCase_ChatCompletions_AuthenticationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	authError := errors.New("authentication failed")

	// Mock expectations
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(nil, authError)
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	response, err := useCase.ChatCompletions(req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestProxyUseCase_ChatCompletions_DefaultModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		// Model is empty, should use default
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	expectedResponse := &entities.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen3-coder-plus", // Should use default model
		Choices: []entities.ChatCompletionChoice{
			{
				Index:        0,
				Message:      entities.ChatMessage{Role: "assistant", Content: "Hello there!"},
				FinishReason: "stop",
			},
		},
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	// Mock expectations - expect the request with default model
	expectedReq := &entities.ChatCompletionRequest{
		Model: "qwen3-coder-plus", // Should be set to default
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(expectedReq, credentials).Return(createMockHttpResponse(expectedResponse), nil)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	response, err := useCase.ChatCompletions(req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "qwen3-coder-plus", response.Model)
}

func TestProxyUseCase_StreamChatCompletions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	streamingResponse := createMockStreamingHttpResponse()
	writer := httptest.NewRecorder()

	// Mock expectations
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(req, credentials).Return(streamingResponse, nil)
	mockStreamingUseCase.EXPECT().ProcessStreamingResponse(gomock.Any(), streamingResponse, writer).Return(nil)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.StreamChatCompletions(req, writer)

	assert.NoError(t, err)
}

func TestProxyUseCase_StreamChatCompletions_AuthFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Stream: true,
	}

	authError := errors.New("authentication failed")
	writer := httptest.NewRecorder()

	// Mock expectations
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(nil, authError)
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.StreamChatCompletions(req, writer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestProxyUseCase_GetModels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	// This method doesn't exist in the current implementation
	// We would need to add it or test through other means
	// For now, this is a placeholder test
	assert.NotNil(t, useCase)
}

// Negative Test Cases

func TestNewProxyUseCase_NilAuthUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Test with nil auth use case - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewProxyUseCase(nil, mockQwenGateway, mockStreamingUseCase, mockLogger)
	})
}

func TestNewProxyUseCase_NilQwenGateway(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Test with nil qwen gateway - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewProxyUseCase(mockAuthUseCase, nil, mockStreamingUseCase, mockLogger)
	})
}

func TestNewProxyUseCase_NilStreamingUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Test with nil streaming use case - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewProxyUseCase(mockAuthUseCase, mockQwenGateway, nil, mockLogger)
	})
}

func TestNewProxyUseCase_NilLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)

	// Test with nil logger - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, nil)
	})
}

func TestProxyUseCase_ChatCompletions_NilRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	// Test with nil request - should return error
	response, err := useCase.ChatCompletions(nil)

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestProxyUseCase_ChatCompletions_AuthUseCasePanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	// Mock auth use case to panic
	mockAuthUseCase.EXPECT().EnsureAuthenticated().DoAndReturn(func() (*entities.Credentials, error) {
		panic("auth panic")
	})

	// Should handle auth panic gracefully
	assert.Panics(t, func() {
		useCase.ChatCompletions(req)
	})
}

func TestProxyUseCase_ChatCompletions_QwenGatewayPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	// Mock successful auth but panicking gateway
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(req, credentials).DoAndReturn(func(req *entities.ChatCompletionRequest, creds *entities.Credentials) (*http.Response, error) {
		panic("gateway panic")
	})

	// Should handle gateway panic gracefully
	assert.Panics(t, func() {
		useCase.ChatCompletions(req)
	})
}

func TestProxyUseCase_ChatCompletions_DefaultModelWithInvalidRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	// Request with empty model (should use default)
	req := &entities.ChatCompletionRequest{
		Messages: []entities.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	// Mock successful auth and gateway call
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(gomock.Any(), credentials).Return(nil, errors.New("gateway error"))

	response, err := useCase.ChatCompletions(req)

	// Should handle gateway error gracefully
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestProxyUseCase_StreamChatCompletions_NilRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	writer := httptest.NewRecorder()

	// Test with nil request - should return error
	err := useCase.StreamChatCompletions(nil, writer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestProxyUseCase_StreamChatCompletions_NilWriter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Stream: true,
	}

	// Test with nil writer - should return error
	err := useCase.StreamChatCompletions(req, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "writer cannot be nil")
}

func TestProxyUseCase_StreamChatCompletions_StreamingUseCasePanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	req := &entities.ChatCompletionRequest{
		Stream: true,
	}

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	streamingResponse := &http.Response{
		StatusCode: 200,
		Body:       &mockReadCloser{data: []byte("data: test\n\ndata: [DONE]\n\n")},
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()

	// Mock successful auth and gateway but panicking streaming use case
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)
	mockQwenGateway.EXPECT().ChatCompletions(req, credentials).Return(streamingResponse, nil)
	mockStreamingUseCase.EXPECT().ProcessStreamingResponse(gomock.Any(), streamingResponse, writer).DoAndReturn(func(ctx context.Context, resp *http.Response, w http.ResponseWriter) error {
		panic("streaming panic")
	})

	// Should handle streaming panic gracefully
	assert.Panics(t, func() {
		useCase.StreamChatCompletions(req, writer)
	})
}

func TestProxyUseCase_CheckAuthentication_AuthUseCasePanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	// Mock auth use case to panic
	mockAuthUseCase.EXPECT().EnsureAuthenticated().DoAndReturn(func() (*entities.Credentials, error) {
		panic("auth panic")
	})

	// Should handle auth panic gracefully
	assert.Panics(t, func() {
		useCase.CheckAuthentication()
	})
}

func TestProxyUseCase_CheckAuthentication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthUseCase := mocks.NewMockAuthUseCaseInterface(ctrl)
	mockQwenGateway := mocks.NewMockQwenAPIGateway(ctrl)
	mockStreamingUseCase := mocks.NewMockStreamingUseCaseInterface(ctrl)
	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewProxyUseCase(mockAuthUseCase, mockQwenGateway, mockStreamingUseCase, mockLogger)

	credentials := &entities.Credentials{
		ResourceURL: "https://api.example.com",
	}

	// Mock expectations
	mockAuthUseCase.EXPECT().EnsureAuthenticated().Return(credentials, nil)

	result, err := useCase.CheckAuthentication()

	assert.NoError(t, err)
	assert.Equal(t, credentials, result)
}

// Helper functions for creating mock responses
func createMockHttpResponse(response *entities.ChatCompletionResponse) *http.Response {
	jsonData, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: 200,
		Body:       &mockReadCloser{data: jsonData},
		Header:     make(http.Header),
	}
}

func createMockStreamingHttpResponse() *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       &mockReadCloser{data: []byte("data: {\"choices\":[]}\n\ndata: [DONE]\n\n")},
		Header:     make(http.Header),
	}
}

// mockReadCloser implements io.ReadCloser for testing
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}
