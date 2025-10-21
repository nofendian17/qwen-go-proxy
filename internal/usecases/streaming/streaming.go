package streaming

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"qwen-go-proxy/internal/infrastructure/logging"
	"time"
)

// StreamingUseCase defines the streaming use case with simplified architecture
type StreamingUseCase struct {
	logger logging.LoggerInterface
}

// NewStreamingUseCase creates a new streaming use case
func NewStreamingUseCase(logger logging.LoggerInterface) *StreamingUseCase {
	return &StreamingUseCase{
		logger: logger,
	}
}

// ProcessStreamingResponse handles streaming responses with simplified architecture
func (uc *StreamingUseCase) ProcessStreamingResponse(
	ctx context.Context,
	resp *http.Response,
	writer http.ResponseWriter,
) error {
	uc.logger.Info("Starting streaming response processing",
		"response_status", resp.StatusCode,
		"response_headers", resp.Header)

	for name, values := range resp.Header {
		for _, value := range values {
			writer.Header().Add(name, value)
		}
	}
	writer.WriteHeader(resp.StatusCode)

	// Wrap the writer
	wrappedWriter := &responseWriterWrapper{ResponseWriter: writer}

	// Create stream processor
	processor := NewStreamProcessor(wrappedWriter, ctx, uc.logger)

	// Process the stream
	reader := bufio.NewReader(resp.Body)
	uc.logger.Debug("Starting stream processing with reader")

	// Read and process lines
	for {
		select {
		case <-ctx.Done():
			uc.logger.Debug("Context cancelled, stopping stream processing")
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				uc.logger.Error("Error reading from upstream", "error", err)
				return err
			}
			break
		}

		if err := processor.ProcessLine(line); err != nil {
			if errors.Is(err, context.Canceled) {
				uc.logger.Debug("Client disconnected, stopping stream processing")
				return nil
			}
			uc.logger.Error("Error processing stream line", "error", err)
			return err
		}

		if processor.state.Current == StateTerminating {
			break
		}
	}

	// Log final statistics
	uc.logger.Info("Streaming completed",
		"chunks_processed", processor.state.ChunkCount,
		"errors", processor.state.ErrorCount,
		"duration", time.Since(processor.state.StartTime))

	return nil
}

// StreamingUseCaseInterface defines the interface for streaming operations
type StreamingUseCaseInterface interface {
	ProcessStreamingResponse(ctx context.Context, resp *http.Response, writer http.ResponseWriter) error
}
