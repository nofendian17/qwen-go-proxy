package streaming

import (
	"context"
	"net/http/httptest"
	"testing"

	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewStreamProcessor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)
	writer := &responseWriterWrapper{ResponseWriter: httptest.NewRecorder()}
	ctx := context.Background()

	processor := NewStreamProcessor(writer, ctx, mockLogger)

	assert.NotNil(t, processor)
	assert.Equal(t, writer, processor.writer)
	assert.Equal(t, ctx, processor.ctx)
	assert.Equal(t, mockLogger, processor.logger)
	assert.NotNil(t, processor.state)
	assert.NotNil(t, processor.parser)
	assert.NotNil(t, processor.recovery)
}

func TestStreamProcessor_ProcessLine_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Mock logger calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	writer := &responseWriterWrapper{ResponseWriter: httptest.NewRecorder()}
	ctx := context.Background()

	processor := NewStreamProcessor(writer, ctx, mockLogger)

	// Test with a valid SSE line containing content
	line := `data: {"choices":[{"delta":{"content":"Hello"}}]}`

	err := processor.ProcessLine(line)

	assert.NoError(t, err)
	assert.Equal(t, StateStuttering, processor.state.Current)
	assert.Equal(t, 1, processor.state.ChunkCount)
}

func TestStreamProcessor_ProcessLine_Done(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Mock logger calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	writer := &responseWriterWrapper{ResponseWriter: httptest.NewRecorder()}
	ctx := context.Background()

	processor := NewStreamProcessor(writer, ctx, mockLogger)

	// Test with DONE message
	line := `data: [DONE]`

	err := processor.ProcessLine(line)

	assert.NoError(t, err)
	assert.Equal(t, StateTerminating, processor.state.Current)
	assert.Equal(t, 1, processor.state.ChunkCount)
}

func TestStreamProcessor_ProcessLine_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Mock logger calls - expect Warn for malformed JSON
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

	writer := &responseWriterWrapper{ResponseWriter: httptest.NewRecorder()}
	ctx := context.Background()

	processor := NewStreamProcessor(writer, ctx, mockLogger)

	// Test with invalid JSON
	line := `data: {"invalid": json}`

	err := processor.ProcessLine(line)

	// Should handle malformed JSON gracefully (skip and continue)
	assert.NoError(t, err)
	assert.Equal(t, StateInitial, processor.state.Current) // Error chunks don't increment counter
	assert.Equal(t, 0, processor.state.ChunkCount)
	assert.Equal(t, 1, processor.state.ErrorCount)
}

func TestStreamProcessor_ProcessLine_ClientDisconnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLoggerInterface(ctrl)

	// Mock logger calls
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	writer := &responseWriterWrapper{ResponseWriter: httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context to simulate client disconnect

	processor := NewStreamProcessor(writer, ctx, mockLogger)

	line := `data: {"choices":[{"delta":{"content":"Hello"}}]}`

	err := processor.ProcessLine(line)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, StateTerminating, processor.state.Current)
}
