package streaming

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
)

// StreamingUseCase defines the streaming use case with advanced features
type StreamingUseCase struct {
	config             *entities.StreamingConfig
	logger             logging.LoggerInterface
	circuitBreaker     *entities.CircuitBreaker
	stutteringDetector *entities.StutteringDetector
}

// NewStreamingUseCase creates a new streaming use case
func NewStreamingUseCase(config *entities.StreamingConfig, logger logging.LoggerInterface) *StreamingUseCase {
	return &StreamingUseCase{
		config: config,
		logger: logger,
		circuitBreaker: &entities.CircuitBreaker{
			MaxFailures:      config.MaxErrors,
			ResetTimeout:     30 * time.Second, // Configurable in future
			State:            entities.CircuitClosed,
			HalfOpenMaxTries: 3,
		},
		stutteringDetector: &entities.StutteringDetector{
			WindowSize:          config.WindowSize,
			SimilarityThreshold: config.SimilarityThreshold,
			ContentHistory:      make([]entities.ContentChunk, 0),
			TimeWindow:          config.TimeWindow,
			MinConfidence:       config.MinConfidence,
		},
	}
}

// ProcessStreamingResponse handles streaming responses with advanced features
func (uc *StreamingUseCase) ProcessStreamingResponse(
	ctx context.Context,
	resp *http.Response,
	writer http.ResponseWriter,
) error {
	uc.logger.Info("Starting streaming response processing",
		"response_status", resp.StatusCode,
		"response_headers", resp.Header,
		"circuit_breaker_state", uc.circuitBreaker.State.String())

	// Check circuit breaker
	if !uc.canExecute() {
		uc.logger.Warn("Circuit breaker is open, rejecting request",
			"state", uc.circuitBreaker.State.String(),
			"failure_count", uc.circuitBreaker.FailureCount)
		return fmt.Errorf("circuit breaker is open, operation not allowed")
	}

	// Set headers for SSE
	uc.logger.Debug("Setting SSE headers")
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)
	uc.logger.Debug("SSE headers set and status code sent")

	// Create stream processor
	processor := NewStreamProcessor(uc.config, uc.stutteringDetector, uc.logger)

	// Process the stream
	reader := bufio.NewReader(resp.Body)
	uc.logger.Debug("Starting stream processing with reader")
	metrics, err := processor.ProcessStream(ctx, reader, writer)

	// Update circuit breaker based on result
	if err != nil {
		uc.logger.Error("Stream processing failed",
			"error", err,
			"chunks_processed", metrics.ChunksProcessed,
			"errors_encountered", metrics.ErrorsEncountered)
		uc.recordFailure()
		return err
	}

	uc.logger.Debug("Stream processing completed successfully")
	uc.recordSuccess()

	// Log metrics
	uc.logger.Info("Streaming completed",
		"chunks_processed", metrics.ChunksProcessed,
		"errors", metrics.ErrorsEncountered,
		"duration", metrics.Duration,
		"state_transitions", len(metrics.StateTransitions),
		"stuttering_events", len(metrics.StutteringEvents))

	return nil
}

// canExecute checks if the circuit breaker allows execution
func (uc *StreamingUseCase) canExecute() bool {
	switch uc.circuitBreaker.State {
	case entities.CircuitClosed:
		return true
	case entities.CircuitOpen:
		if time.Since(uc.circuitBreaker.LastFailureTime) > uc.circuitBreaker.ResetTimeout {
			uc.circuitBreaker.State = entities.CircuitHalfOpen
			uc.circuitBreaker.HalfOpenTries = 0
			uc.logger.Debug("Circuit breaker transitioning to half-open state", "state", "transitioning")
			return true
		}
		return false
	case entities.CircuitHalfOpen:
		return uc.circuitBreaker.HalfOpenTries < uc.circuitBreaker.HalfOpenMaxTries
	default:
		return false
	}
}

// recordFailure records a failure in the circuit breaker
func (uc *StreamingUseCase) recordFailure() {
	uc.circuitBreaker.FailureCount++
	uc.circuitBreaker.LastFailureTime = time.Now()

	if uc.circuitBreaker.FailureCount >= uc.circuitBreaker.MaxFailures {
		uc.circuitBreaker.State = entities.CircuitOpen
		uc.logger.Warn("Circuit breaker opened due to too many failures", "failures", uc.circuitBreaker.FailureCount)
	}
}

// recordSuccess records a success in the circuit breaker
func (uc *StreamingUseCase) recordSuccess() {
	uc.circuitBreaker.SuccessCount++
	uc.circuitBreaker.LastSuccessTime = time.Now()

	if uc.circuitBreaker.State == entities.CircuitHalfOpen {
		uc.circuitBreaker.HalfOpenTries++
		if uc.circuitBreaker.HalfOpenTries >= uc.circuitBreaker.HalfOpenMaxTries {
			uc.circuitBreaker.State = entities.CircuitClosed
			uc.circuitBreaker.FailureCount = 0
			uc.logger.Info("Circuit breaker closed after successful half-open attempts")
		}
	} else if uc.circuitBreaker.State == entities.CircuitClosed {
		// Reset failure count on success
		uc.circuitBreaker.FailureCount = 0
	}
}

// GetCircuitBreakerStatus returns the current circuit breaker status
func (uc *StreamingUseCase) GetCircuitBreakerStatus() entities.CircuitBreaker {
	return *uc.circuitBreaker
}

// StreamingUseCaseInterface defines the interface for streaming operations
type StreamingUseCaseInterface interface {
	ProcessStreamingResponse(ctx context.Context, resp *http.Response, writer http.ResponseWriter) error
}
