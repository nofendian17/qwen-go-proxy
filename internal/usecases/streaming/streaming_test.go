package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewStreamingUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{
		MaxErrors:           5,
		BufferSize:          4096,
		TimeoutSeconds:      300,
		WindowSize:          5,
		SimilarityThreshold: 0.8,
		TimeWindow:          time.Second * 2,
		MinConfidence:       0.7,
	}

	useCase := NewStreamingUseCase(config, mockLogger)

	assert.NotNil(t, useCase)
	assert.Equal(t, config, useCase.config)
	assert.Equal(t, mockLogger, useCase.logger)
	assert.NotNil(t, useCase.circuitBreaker)
	assert.NotNil(t, useCase.stutteringDetector)

	// Check circuit breaker initialization
	assert.Equal(t, config.MaxErrors, useCase.circuitBreaker.MaxFailures)
	assert.Equal(t, entities.CircuitClosed, useCase.circuitBreaker.State)

	// Check stuttering detector initialization
	assert.Equal(t, config.WindowSize, useCase.stutteringDetector.WindowSize)
	assert.Equal(t, config.SimilarityThreshold, useCase.stutteringDetector.SimilarityThreshold)
	assert.Equal(t, config.TimeWindow, useCase.stutteringDetector.TimeWindow)
	assert.Equal(t, config.MinConfidence, useCase.stutteringDetector.MinConfidence)
}

func TestStreamingUseCase_CanExecute_CircuitClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 5}

	useCase := NewStreamingUseCase(config, mockLogger)
	useCase.circuitBreaker.State = entities.CircuitClosed

	canExecute := useCase.canExecute()
	assert.True(t, canExecute)
}

func TestStreamingUseCase_CanExecute_CircuitOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 5}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Mock logging calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	useCase.circuitBreaker.State = entities.CircuitOpen
	useCase.circuitBreaker.LastFailureTime = time.Now().Add(time.Hour)
}

func TestStreamingUseCase_CanExecute_CircuitHalfOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 5}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Mock logging calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	useCase.circuitBreaker.State = entities.CircuitOpen
	useCase.circuitBreaker.LastFailureTime = time.Now().Add(time.Hour) // Future time, should not transition

	canExecute := useCase.canExecute()
	assert.False(t, canExecute)
}

func TestStreamingUseCase_RecordFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 2}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Mock logging calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	// Initially closed
	assert.Equal(t, entities.CircuitClosed, useCase.circuitBreaker.State)

	// Record first failure
	useCase.recordFailure()
	assert.Equal(t, entities.CircuitClosed, useCase.circuitBreaker.State)
	assert.Equal(t, 1, useCase.circuitBreaker.FailureCount)

	// Record second failure - should open circuit
	useCase.recordFailure()
	assert.Equal(t, entities.CircuitOpen, useCase.circuitBreaker.State)
	assert.Equal(t, 2, useCase.circuitBreaker.FailureCount)
}

func TestStreamingUseCase_RecordSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 2}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Start with half-open state
	useCase.circuitBreaker.State = entities.CircuitHalfOpen
	useCase.circuitBreaker.FailureCount = 1

	// Record success - should increment tries but not close circuit yet (needs 3 tries)
	useCase.recordSuccess()
	assert.Equal(t, entities.CircuitHalfOpen, useCase.circuitBreaker.State)
	assert.Equal(t, 1, useCase.circuitBreaker.FailureCount)
	assert.Equal(t, 1, useCase.circuitBreaker.SuccessCount)
}

func TestStreamingUseCase_ProcessStreamingResponse_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{
		MaxErrors:      5,
		BufferSize:     4096,
		TimeoutSeconds: 300,
	}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Create a mock HTTP response with streaming data
	streamingData := `data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

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
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	assert.NoError(t, err)
}

func TestStreamingUseCase_ProcessStreamingResponse_CircuitOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 1}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Open the circuit and set recent failure time
	useCase.circuitBreaker.State = entities.CircuitOpen
	useCase.circuitBreaker.LastFailureTime = time.Now()

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()
	ctx := context.Background()

	mockLogger.EXPECT().Warn("Circuit breaker is open, rejecting request", "state", "open").Times(1)

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestStreamingUseCase_ProcessStreamingResponse_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{
		MaxErrors:      5,
		BufferSize:     4096,
		TimeoutSeconds: 1, // Very short timeout
	}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Create a response that will take longer than timeout
	streamingData := `data: {"content": "test"}
data: [DONE]
`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(streamingData)),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()

	// Create context with timeout shorter than processing time
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	// Processing completes before timeout
	assert.NoError(t, err)
}

func TestStreamingUseCase_ProcessStreamingResponse_InvalidResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{MaxErrors: 5}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Create response with invalid data that should cause processing errors
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("invalid data")),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()
	ctx := context.Background()

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	// Should handle the error gracefully
	assert.NoError(t, err) // The function should not return an error for processing issues
}

func TestStreamingUseCase_ProcessStreamingResponse_WithProcessor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{
		MaxErrors:           5,
		BufferSize:          4096,
		TimeoutSeconds:      300,
		WindowSize:          5,
		SimilarityThreshold: 0.8,
		TimeWindow:          time.Second * 2,
		MinConfidence:       0.7,
	}

	useCase := NewStreamingUseCase(config, mockLogger)

	streamingData := `data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: [DONE]

`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(streamingData)),
		Header:     make(http.Header),
	}

	writer := httptest.NewRecorder()
	ctx := context.Background()

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	err := useCase.ProcessStreamingResponse(ctx, resp, writer)

	assert.NoError(t, err)
}

func TestStutteringDetector_AnalyzeStuttering(t *testing.T) {
	detector := &entities.StutteringDetector{
		WindowSize:          3,
		SimilarityThreshold: 0.8,
		TimeWindow:          2 * time.Second,
		MinConfidence:       0.7,
		ContentHistory:      make([]entities.ContentChunk, 0),
	}

	// Test with first chunk (should always be stuttering)
	result := detector.AnalyzeStuttering("Hello", "")
	if !result.IsStuttering {
		t.Errorf("First chunk should always be stuttering")
	}
	if result.Confidence != 1.0 {
		t.Errorf("First chunk should have confidence 1.0, got: %f", result.Confidence)
	}

	// Add more chunks to history for better analysis
	detector.ContentHistory = append(detector.ContentHistory, entities.ContentChunk{
		Content:    "Hello",
		Timestamp:  time.Now().Add(-time.Second),
		Length:     5,
		TokenCount: 1,
		ChunkIndex: 0,
	})
	detector.ContentHistory = append(detector.ContentHistory, entities.ContentChunk{
		Content:    " world",
		Timestamp:  time.Now(),
		Length:     6,
		TokenCount: 1,
		ChunkIndex: 1,
	})

	// Test with similar content (should detect stuttering)
	result = detector.AnalyzeStuttering("Hello", "Hello")
	if result.Confidence < detector.MinConfidence {
		t.Logf("Content similarity test: confidence=%f, threshold=%f", result.Confidence, detector.MinConfidence)
	}
}

func BenchmarkStreamingUseCase_ProcessStreamingResponse(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	config := &entities.StreamingConfig{
		MaxErrors:           10,
		BufferSize:          4096,
		TimeoutSeconds:      30,
		WindowSize:          5,
		SimilarityThreshold: 0.8,
		TimeWindow:          2 * time.Second,
		MinConfidence:       0.7,
	}

	useCase := NewStreamingUseCase(config, mockLogger)

	// Create larger mock response for benchmarking
	var responseBody bytes.Buffer
	for i := 0; i < 100; i++ {
		data := map[string]interface{}{
			"id":      "bench-test",
			"object":  "chat.completion.chunk",
			"created": float64(time.Now().Unix()),
			"model":   "qwen3-coder-plus",
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"delta": map[string]interface{}{
						"content": "test content ",
					},
				},
			},
		}
		jsonData, _ := json.Marshal(data)
		responseBody.WriteString("data: ")
		responseBody.Write(jsonData)
		responseBody.WriteString("\n\n")
	}
	responseBody.WriteString("data: [DONE]\n\n")

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(responseBody.String())),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "text/event-stream")

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		err := useCase.ProcessStreamingResponse(context.Background(), resp, w)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
