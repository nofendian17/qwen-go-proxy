package streaming

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewStreamingUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewStreamingUseCase(mockLogger)

	assert.NotNil(t, useCase)
	assert.Equal(t, mockLogger, useCase.logger)
}

func TestStreamingUseCase_ProcessStreamingResponse_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewStreamingUseCase(mockLogger)

	// Create a mock HTTP response with streaming data
	streamingData := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: [DONE]

`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(streamingData)),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()
	ctx := context.Background()

	// Mock logger calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug("Client disconnected during streaming, stopping response").Times(0)

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	assert.NoError(t, err)
	// Check that data was written to the response
	responseBody := writer.Body.String()
	assert.Contains(t, responseBody, "data:")
	assert.Contains(t, responseBody, "[DONE]")
}

func TestStreamingUseCase_ProcessStreamingResponse_ClientDisconnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewStreamingUseCase(mockLogger)

	// Create a response with data that will be processed
	streamingData := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: [DONE]

`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(streamingData)),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()

	// Create context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to simulate client disconnect

	// Mock logger calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	// Should return context.Canceled error
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStreamingUseCase_ProcessStreamingResponse_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	useCase := NewStreamingUseCase(mockLogger)

	// Create response with invalid JSON data
	streamingData := `data: {"invalid": json}

data: [DONE]

`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(streamingData)),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()
	ctx := context.Background()

	// Mock logger calls - expect Warn for malformed JSON
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	// Should handle invalid JSON gracefully (skip malformed chunks)
	assert.NoError(t, err)
	// Should still contain the DONE message
	responseBody := writer.Body.String()
	assert.Contains(t, responseBody, "[DONE]")
}
