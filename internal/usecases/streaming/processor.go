package streaming

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
)

// StreamProcessor handles the processing of streaming data with advanced features
type StreamProcessor struct {
	config             *entities.StreamingConfig
	stutteringDetector *entities.StutteringDetector
	logger             logging.LoggerInterface
	state              *entities.StreamState
	metrics            *entities.StreamingMetrics
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(config *entities.StreamingConfig, detector *entities.StutteringDetector, logger logging.LoggerInterface) *StreamProcessor {
	return &StreamProcessor{
		config:             config,
		stutteringDetector: detector,
		logger:             logger,
		state:              &entities.StreamState{},
		metrics:            &entities.StreamingMetrics{},
	}
}

// ProcessStream processes the streaming response
func (sp *StreamProcessor) ProcessStream(ctx context.Context, reader *bufio.Reader, writer http.ResponseWriter) (*entities.StreamingMetrics, error) {
	sp.initializeState()

	for {
		select {
		case <-ctx.Done():
			sp.logger.Debug("Client disconnected, stopping stream processing")
			sp.transitionToState(entities.StateTerminating, "client disconnected")
			return sp.metrics, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				sp.logger.Error("Error reading from upstream", "error", err)
				sp.recordError()
			}
			break
		}

		if err := sp.processLine(line, writer); err != nil {
			sp.recordError()
			if sp.state.Current == entities.StateTerminating {
				break
			}
		}

		if sp.state.Current == entities.StateTerminating {
			break
		}
	}

	sp.finalizeMetrics()
	return sp.metrics, nil
}

// processLine processes a single line from the stream
func (sp *StreamProcessor) processLine(line string, writer http.ResponseWriter) error {
	chunk := sp.parseChunk(line)
	sp.metrics.ChunksProcessed++

	switch sp.state.Current {
	case entities.StateInitial:
		return sp.handleInitialState(chunk, writer)
	case entities.StateStuttering:
		return sp.handleStutteringState(chunk, writer)
	case entities.StateNormalFlow:
		return sp.handleNormalFlowState(chunk, writer)
	case entities.StateRecovering:
		return sp.handleRecoveringState(chunk, writer)
	default:
		return fmt.Errorf("unknown state: %s", sp.state.Current.String())
	}
}

// parseChunk parses a raw line into a ParsedChunk
func (sp *StreamProcessor) parseChunk(rawLine string) *entities.ParsedChunk {
	chunk := &entities.ParsedChunk{
		RawLine:  rawLine,
		ParsedAt: time.Now(),
		Metadata: make(map[string]interface{}),
	}

	trimmed := strings.TrimSpace(rawLine)
	if len(trimmed) == 0 {
		chunk.Type = entities.ChunkTypeEmpty
		chunk.IsValid = false
		return chunk
	}

	if !strings.HasPrefix(rawLine, "data: ") {
		chunk.Type = entities.ChunkTypeUnknown
		chunk.Content = rawLine
		chunk.IsValid = true
		return chunk
	}

	data := strings.TrimPrefix(rawLine, "data: ")
	data = strings.TrimRight(data, "\n")
	chunk.Content = data

	if data == "[DONE]" {
		chunk.Type = entities.ChunkTypeDone
		chunk.IsValid = true
		return chunk
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		chunk.Type = entities.ChunkTypeMalformed
		chunk.Error = fmt.Errorf("failed to parse JSON: %w", err)
		chunk.IsValid = false
		return chunk
	}

	if content, hasContent := sp.extractContentFromJSON(jsonData); hasContent {
		chunk.Type = entities.ChunkTypeData
		chunk.IsValid = true
		chunk.HasContent = true
		chunk.ContentText = content
		chunk.Metadata = jsonData
	} else {
		chunk.Type = entities.ChunkTypeData
		chunk.IsValid = true
		chunk.HasContent = false
		chunk.Metadata = jsonData
	}

	return chunk
}

// extractContentFromJSON extracts content from JSON response
func (sp *StreamProcessor) extractContentFromJSON(jsonData map[string]interface{}) (string, bool) {
	choices, ok := jsonData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", false
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", false
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return "", false
	}

	content, ok := delta["content"].(string)
	return content, ok
}

// handleInitialState handles the initial state
func (sp *StreamProcessor) handleInitialState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	if chunk.Type == entities.ChunkTypeData && chunk.HasContent {
		// Analyze for stuttering
		result := sp.analyzeStuttering(chunk.ContentText, "")

		if result.IsStuttering {
			sp.transitionToState(entities.StateStuttering, "initial stuttering detected")
			sp.state.Buffer = chunk.ContentText
			sp.metrics.StutteringEvents = append(sp.metrics.StutteringEvents, entities.StutteringEvent{
				Detected:   true,
				Confidence: result.Confidence,
				Reason:     result.Reason,
				Timestamp:  time.Now(),
			})
			return nil
		}
	}

	sp.transitionToState(entities.StateNormalFlow, "normal flow started")
	return sp.forwardChunk(chunk, writer)
}

// handleStutteringState handles the stuttering state
func (sp *StreamProcessor) handleStutteringState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	if chunk.Type != entities.ChunkTypeData || !chunk.HasContent {
		return sp.forwardChunk(chunk, writer)
	}

	result := sp.analyzeStuttering(chunk.ContentText, sp.state.Buffer)

	if result.ShouldFlush {
		sp.transitionToState(entities.StateNormalFlow, "stuttering resolved")
		// Flush buffer and current chunk
		combinedContent := sp.state.Buffer + chunk.ContentText
		combinedChunk := &entities.ParsedChunk{
			Type:        entities.ChunkTypeData,
			Content:     combinedContent,
			IsValid:     true,
			HasContent:  true,
			ContentText: combinedContent,
			Metadata:    chunk.Metadata,
		}
		sp.state.Buffer = ""
		return sp.forwardChunk(combinedChunk, writer)
	}

	if result.ShouldBuffer {
		sp.state.Buffer += chunk.ContentText
		return nil
	}

	return sp.forwardChunk(chunk, writer)
}

// handleNormalFlowState handles the normal flow state
func (sp *StreamProcessor) handleNormalFlowState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	if chunk.Type == entities.ChunkTypeDone {
		sp.transitionToState(entities.StateTerminating, "stream completed")
	}

	return sp.forwardChunk(chunk, writer)
}

// handleRecoveringState handles the recovering state
func (sp *StreamProcessor) handleRecoveringState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	// Simplified recovery logic - transition back to normal flow
	sp.transitionToState(entities.StateNormalFlow, "recovered from error")
	return sp.forwardChunk(chunk, writer)
}

// forwardChunk forwards a chunk to the client
func (sp *StreamProcessor) forwardChunk(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	var data string
	switch chunk.Type {
	case entities.ChunkTypeData:
		if chunk.Metadata != nil {
			// Convert Qwen format to OpenAI chat completion chunk format
			openAIChunk, err := sp.convertQwenToOpenAI(chunk.Metadata)
			if err != nil {
				sp.logger.Error("Failed to convert chunk format", "error", err)
				// Fallback to original data
				jsonData, _ := json.Marshal(chunk.Metadata)
				data = "data: " + string(jsonData) + "\n\n"
			} else {
				data = "data: " + openAIChunk + "\n\n"
			}
		}
	case entities.ChunkTypeDone:
		data = "data: [DONE]\n\n"
	case entities.ChunkTypeUnknown:
		data = chunk.Content + "\n"
	}

	if data != "" {
		if _, err := writer.Write([]byte(data)); err != nil {
			return fmt.Errorf("failed to write to client: %w", err)
		}
		sp.state.IncrementChunk()
	}

	return nil
}

// analyzeStuttering performs stuttering analysis
func (sp *StreamProcessor) analyzeStuttering(current, previous string) entities.StutteringResult {
	return sp.stutteringDetector.AnalyzeStuttering(current, previous)
}

// transitionToState changes the state and logs the transition
func (sp *StreamProcessor) transitionToState(newState entities.StreamingState, reason string) {
	oldState := sp.state.Current
	sp.state.Current = newState

	transition := entities.StateTransition{
		From:      oldState,
		To:        newState,
		Reason:    reason,
		Timestamp: time.Now(),
	}
	sp.metrics.StateTransitions = append(sp.metrics.StateTransitions, transition)

	sp.logger.Debug("State transition",
		"from", oldState.String(),
		"to", newState.String(),
		"reason", reason)
}

// recordError records an error
func (sp *StreamProcessor) recordError() {
	sp.state.ErrorCount++
	sp.metrics.ErrorsEncountered++

	if sp.state.ErrorCount >= sp.config.MaxErrors {
		sp.transitionToState(entities.StateTerminating, "too many errors")
	}
}

// initializeState initializes the stream state
func (sp *StreamProcessor) initializeState() {
	sp.state.Current = entities.StateInitial
	sp.state.StartTime = time.Now()
	sp.metrics = &entities.StreamingMetrics{
		StateTransitions: make([]entities.StateTransition, 0),
		StutteringEvents: make([]entities.StutteringEvent, 0),
	}
}

// finalizeMetrics finalizes the metrics
func (sp *StreamProcessor) finalizeMetrics() {
	sp.metrics.Duration = time.Since(sp.state.StartTime)
}

// convertQwenToOpenAI converts Qwen streaming format to OpenAI chat completion chunk format
func (sp *StreamProcessor) convertQwenToOpenAI(qwenData map[string]interface{}) (string, error) {
	// Extract required fields from Qwen format
	id, ok := qwenData["id"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid id field")
	}

	created, ok := qwenData["created"].(float64)
	if !ok {
		return "", fmt.Errorf("missing or invalid created field")
	}

	model, _ := qwenData["model"].(string)
	if model == "" {
		model = "qwen3-coder-plus" // default model
	}

	choices, ok := qwenData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("missing or invalid choices field")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format")
	}

	// Create OpenAI format chunk
	chunk := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": choice["delta"],
			},
		},
		"usage": nil,
	}

	// Handle tool calls in delta
	if delta, ok := choice["delta"].(map[string]interface{}); ok {
		if toolCalls, exists := delta["tool_calls"]; exists {
			// Ensure tool calls are properly formatted
			if toolCallsSlice, ok := toolCalls.([]interface{}); ok {
				for i, tc := range toolCallsSlice {
					if tcMap, ok := tc.(map[string]interface{}); ok {
						// Ensure type is set
						if _, hasType := tcMap["type"]; !hasType {
							tcMap["type"] = "function"
						}
						// Ensure index is set for streaming
						tcMap["index"] = i
					}
				}
			}
			chunk["choices"].([]map[string]interface{})[0]["delta"] = delta
		}
	}

	// Add finish_reason if present
	if finishReason, exists := choice["finish_reason"]; exists {
		chunk["choices"].([]map[string]interface{})[0]["finish_reason"] = finishReason
	}

	// Marshal to JSON
	chunkJSON, err := json.Marshal(chunk)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI chunk: %w", err)
	}

	return string(chunkJSON), nil
}
