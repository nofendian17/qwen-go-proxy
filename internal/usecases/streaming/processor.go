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
	seenChunks         map[string]time.Time // Track recently seen chunks for deduplication
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
	sp.logger.Info("Starting stream processing")
	sp.initializeState()

	lineCount := 0
	for {
		select {
		case <-ctx.Done():
			sp.logger.Debug("Client disconnected, stopping stream processing")
			sp.transitionToState(entities.StateTerminating, "client disconnected")
			return sp.metrics, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		lineCount++
		if err != nil {
			if err != io.EOF {
				sp.logger.Error("Error reading from upstream",
					"error", err,
					"line_count", lineCount,
					"current_state", sp.state.Current.String())
				sp.recordError()
			} else {
				sp.logger.Debug("Reached EOF from upstream", "line_count", lineCount)
			}
			break
		}

		sp.logger.Debug("Processing line",
			"line_number", lineCount,
			"line_content", line,
			"line_length", len(line),
			"current_state", sp.state.Current.String())

		if err := sp.processLine(line, writer); err != nil {
			sp.logger.Error("Error processing line",
				"error", err,
				"line_number", lineCount,
				"current_state", sp.state.Current.String())
			sp.recordError()
			if sp.state.Current == entities.StateTerminating {
				sp.logger.Debug("State is terminating, breaking processing loop")
				break
			}
		}

		if sp.state.Current == entities.StateTerminating {
			sp.logger.Debug("Stream processing terminating", "line_count", lineCount)
			break
		}
	}

	sp.logger.Info("Stream processing completed",
		"line_count", lineCount,
		"chunks_processed", sp.metrics.ChunksProcessed,
		"errors", sp.metrics.ErrorsEncountered,
		"final_state", sp.state.Current.String())

	sp.finalizeMetrics()
	return sp.metrics, nil
}

// processLine processes a single line from the stream
func (sp *StreamProcessor) processLine(line string, writer http.ResponseWriter) error {
	chunk := sp.parseChunk(line)
	sp.metrics.ChunksProcessed++

	// Check for immediate duplicates before state processing
	if sp.isDuplicateChunk(chunk) {
		sp.logger.Debug("Duplicate chunk filtered out",
			"chunk_type", chunk.Type.String(),
			"content_length", len(chunk.Content))
		return nil // Skip processing this duplicate
	}

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

	sp.logger.Debug("Parsing chunk", "raw_line", rawLine, "line_length", len(rawLine))

	trimmed := strings.TrimSpace(rawLine)
	if len(trimmed) == 0 {
		sp.logger.Debug("Chunk is empty after trimming")
		chunk.Type = entities.ChunkTypeEmpty
		chunk.IsValid = false
		return chunk
	}

	if !strings.HasPrefix(rawLine, "data: ") {
		sp.logger.Debug("Chunk does not start with 'data: ', treating as unknown type")
		chunk.Type = entities.ChunkTypeUnknown
		chunk.Content = rawLine
		chunk.IsValid = true
		return chunk
	}

	data := strings.TrimPrefix(rawLine, "data: ")
	data = strings.TrimRight(data, "\n")
	chunk.Content = data

	sp.logger.Debug("Extracted data content", "data", data, "data_length", len(data))

	if data == "[DONE]" {
		sp.logger.Debug("Chunk is DONE marker")
		chunk.Type = entities.ChunkTypeDone
		chunk.IsValid = true
		return chunk
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		sp.logger.Error("Failed to parse JSON data",
			"error", err,
			"data", data,
			"data_bytes", []byte(data))
		chunk.Type = entities.ChunkTypeMalformed
		chunk.Error = fmt.Errorf("failed to parse JSON: %w", err)
		chunk.IsValid = false
		return chunk
	}

	sp.logger.Debug("Successfully parsed JSON", "json_keys", getKeys(jsonData))

	if content, hasContent := sp.extractContentFromJSON(jsonData); hasContent {
		sp.logger.Debug("Chunk has content",
			"content_text", content,
			"content_length", len(content))
		chunk.Type = entities.ChunkTypeData
		chunk.IsValid = true
		chunk.HasContent = true
		chunk.ContentText = content
		chunk.Metadata = jsonData
	} else {
		sp.logger.Debug("Chunk has no content")
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
	sp.logger.Debug("Processing initial chunk",
		"chunk_type", chunk.Type.String(),
		"has_content", chunk.HasContent,
		"is_valid", chunk.IsValid,
		"content_length", len(chunk.Content),
		"raw_line", chunk.RawLine)

	// Handle different types of initial chunks more robustly
	if chunk.Type == entities.ChunkTypeData && chunk.HasContent && chunk.ContentText != "" {
		sp.logger.Debug("Initial chunk has content, analyzing for stuttering",
			"content_text", chunk.ContentText,
			"content_length", len(chunk.ContentText))

		// Analyze for stuttering
		result := sp.analyzeStuttering(chunk.ContentText, "")

		sp.logger.Debug("Stuttering analysis result",
			"is_stuttering", result.IsStuttering,
			"confidence", result.Confidence,
			"reason", result.Reason,
			"should_buffer", result.ShouldBuffer,
			"should_flush", result.ShouldFlush)

		if result.IsStuttering {
			sp.transitionToState(entities.StateStuttering, "initial stuttering detected")
			sp.state.Buffer = chunk.ContentText
			sp.metrics.StutteringEvents = append(sp.metrics.StutteringEvents, entities.StutteringEvent{
				Detected:   true,
				Confidence: result.Confidence,
				Reason:     result.Reason,
				Timestamp:  time.Now(),
			})
			sp.logger.Debug("Transitioned to stuttering state", "buffer_content", sp.state.Buffer)
			return nil
		}
	} else if chunk.Type == entities.ChunkTypeData && chunk.IsValid {
		// Valid data chunk but no content - still forward it
		sp.logger.Debug("Initial chunk is valid data but has no content, forwarding directly")
	} else if chunk.Type == entities.ChunkTypeEmpty {
		sp.logger.Debug("Initial chunk is empty, skipping")
		return nil
	} else if !chunk.IsValid {
		sp.logger.Debug("Initial chunk is invalid, skipping")
		return nil
	} else {
		sp.logger.Debug("Initial chunk is not data type or has no content, forwarding as-is",
			"chunk_type", chunk.Type.String(),
			"has_content", chunk.HasContent)
	}

	sp.logger.Debug("Transitioning to normal flow state")
	sp.transitionToState(entities.StateNormalFlow, "normal flow started")
	return sp.forwardChunk(chunk, writer)
}

// handleStutteringState handles the stuttering state
func (sp *StreamProcessor) handleStutteringState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	if chunk.Type != entities.ChunkTypeData || !chunk.HasContent {
		return sp.forwardChunk(chunk, writer)
	}

	result := sp.analyzeStuttering(chunk.ContentText, sp.state.Buffer)

	sp.logger.Debug("Stuttering state analysis",
		"chunk_content", chunk.ContentText,
		"buffer_length", len(sp.state.Buffer),
		"is_stuttering", result.IsStuttering,
		"confidence", result.Confidence,
		"should_flush", result.ShouldFlush,
		"should_buffer", result.ShouldBuffer,
		"reason", result.Reason)

	// Check for buffer timeout - if we've been buffering for too long, flush anyway
	bufferAge := time.Since(sp.state.LastValidChunk)
	bufferTimeout := 10 * time.Second // Configurable timeout

	if sp.state.Buffer != "" && bufferAge > bufferTimeout {
		sp.logger.Warn("Buffer timeout reached, forcing flush",
			"buffer_age", bufferAge,
			"buffer_length", len(sp.state.Buffer))
		sp.transitionToState(entities.StateNormalFlow, "buffer timeout")

		// Flush buffer only (without current chunk)
		bufferChunk := &entities.ParsedChunk{
			Type:        entities.ChunkTypeData,
			Content:     sp.state.Buffer,
			IsValid:     true,
			HasContent:  true,
			ContentText: sp.state.Buffer,
			Metadata:    chunk.Metadata,
		}
		sp.state.Buffer = ""
		if err := sp.forwardChunk(bufferChunk, writer); err != nil {
			return err
		}
	}

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
		sp.logger.Debug("Flushing combined content", "combined_length", len(combinedContent))
		return sp.forwardChunk(combinedChunk, writer)
	}

	if result.ShouldBuffer {
		sp.state.Buffer += chunk.ContentText
		sp.logger.Debug("Buffering chunk", "new_buffer_length", len(sp.state.Buffer))
		return nil
	}

	// If not buffering and not flushing, forward the chunk
	sp.logger.Debug("Forwarding chunk in stuttering state")
	return sp.forwardChunk(chunk, writer)
}

// handleNormalFlowState handles the normal flow state
func (sp *StreamProcessor) handleNormalFlowState(chunk *entities.ParsedChunk, writer http.ResponseWriter) error {
	if chunk.Type == entities.ChunkTypeDone {
		sp.transitionToState(entities.StateTerminating, "stream completed")
		return sp.forwardChunk(chunk, writer)
	}

	// Quick duplicate detection in normal flow
	if sp.state.LastChunkContent != "" && sp.state.LastChunkContent == chunk.ContentText && chunk.HasContent {
		sp.logger.Debug("Duplicate chunk detected in normal flow, transitioning to stuttering",
			"duplicate_content", chunk.ContentText)

		sp.transitionToState(entities.StateStuttering, "immediate duplicate detected")
		sp.state.Buffer = chunk.ContentText

		// Don't forward this duplicate chunk
		return nil
	}

	// Store this chunk for duplicate comparison
	if chunk.HasContent {
		sp.state.LastChunkContent = chunk.ContentText
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
	sp.logger.Debug("Forwarding chunk to client",
		"chunk_type", chunk.Type.String(),
		"has_metadata", chunk.Metadata != nil,
		"content_length", len(chunk.Content))

	var data string
	switch chunk.Type {
	case entities.ChunkTypeData:
		if chunk.Metadata != nil {
			sp.logger.Debug("Converting Qwen format to OpenAI format", "metadata_keys", getKeys(chunk.Metadata))

			// Convert Qwen format to OpenAI chat completion chunk format
			openAIChunk, err := sp.convertQwenToOpenAI(chunk.Metadata)
			if err != nil {
				sp.logger.Error("Failed to convert chunk format", "error", err, "metadata", chunk.Metadata)
				// Fallback to original data
				jsonData, _ := json.Marshal(chunk.Metadata)
				data = "data: " + string(jsonData) + "\n\n"
				sp.logger.Debug("Using fallback format", "fallback_data", data)
			} else {
				data = "data: " + openAIChunk + "\n\n"
				sp.logger.Debug("Successfully converted to OpenAI format", "converted_data", data)
			}
		} else {
			sp.logger.Debug("Chunk has no metadata, skipping conversion")
		}
	case entities.ChunkTypeDone:
		data = "data: [DONE]\n\n"
		sp.logger.Debug("Forwarding DONE chunk")
	case entities.ChunkTypeUnknown:
		data = chunk.Content + "\n"
		sp.logger.Debug("Forwarding unknown chunk type", "content", chunk.Content)
	}

	if data != "" {
		sp.logger.Debug("Writing data to client", "data_length", len(data), "current_state", sp.state.Current.String())

		if _, err := writer.Write([]byte(data)); err != nil {
			sp.logger.Error("Failed to write to client", "error", err, "data_length", len(data))
			return fmt.Errorf("failed to write to client: %w", err)
		}

		// Ensure data is flushed immediately for streaming
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
			sp.logger.Debug("Flushed response writer")
		} else {
			sp.logger.Debug("Response writer does not support flushing")
		}

		sp.state.IncrementChunk()
		sp.logger.Debug("Successfully forwarded chunk", "chunk_count", sp.state.ChunkCount)
	} else {
		sp.logger.Debug("No data to forward")
	}

	return nil
}

// getKeys is a helper function to extract keys from a map for logging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
	sp.seenChunks = make(map[string]time.Time) // Initialize deduplication map
	sp.metrics = &entities.StreamingMetrics{
		StateTransitions: make([]entities.StateTransition, 0),
		StutteringEvents: make([]entities.StutteringEvent, 0),
	}
}

// finalizeMetrics finalizes the metrics
func (sp *StreamProcessor) finalizeMetrics() {
	sp.metrics.Duration = time.Since(sp.state.StartTime)
}

// isDuplicateChunk checks if a chunk is a duplicate of a recently seen chunk
func (sp *StreamProcessor) isDuplicateChunk(chunk *entities.ParsedChunk) bool {
	if chunk.Type != entities.ChunkTypeData || !chunk.IsValid {
		return false // Only deduplicate valid data chunks
	}

	// Create a signature for this chunk based on its content and metadata
	chunkSignature := sp.createChunkSignature(chunk)

	// Check if we've seen this chunk recently (within last 30 seconds)
	if lastSeen, exists := sp.seenChunks[chunkSignature]; exists {
		timeSince := time.Since(lastSeen)
		if timeSince < 30*time.Second {
			return true // This is a duplicate
		}
	}

	// Update the seen chunks map
	sp.seenChunks[chunkSignature] = time.Now()

	// Clean up old entries (older than 60 seconds)
	sp.cleanupSeenChunks()

	return false
}

// createChunkSignature creates a unique signature for a chunk for deduplication
func (sp *StreamProcessor) createChunkSignature(chunk *entities.ParsedChunk) string {
	// For data chunks, use content text and key metadata for signature
	if chunk.HasContent && chunk.ContentText != "" {
		// Include content and key identifiers for uniqueness
		if id, ok := chunk.Metadata["id"].(string); ok {
			return fmt.Sprintf("%s:%s", chunk.ContentText, id)
		}
		return chunk.ContentText
	}

	// For non-content chunks, use the raw content
	return chunk.Content
}

// cleanupSeenChunks removes old entries from the seen chunks map
func (sp *StreamProcessor) cleanupSeenChunks() {
	cutoff := time.Now().Add(-60 * time.Second)
	for signature, timestamp := range sp.seenChunks {
		if timestamp.Before(cutoff) {
			delete(sp.seenChunks, signature)
		}
	}
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
