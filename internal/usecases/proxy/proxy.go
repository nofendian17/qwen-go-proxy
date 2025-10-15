package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/interfaces/gateways"
	"qwen-go-proxy/internal/usecases/auth"
)

// ProxyUseCase defines the API proxy use case
type ProxyUseCase struct {
	authUseCase  *auth.AuthUseCase
	qwenGateway  gateways.QwenAPIGateway
	logger       *logging.Logger
	defaultModel string
}

// NewProxyUseCase creates a new proxy use case
func NewProxyUseCase(authUseCase *auth.AuthUseCase, qwenGateway gateways.QwenAPIGateway, logger *logging.Logger) *ProxyUseCase {
	return &ProxyUseCase{
		authUseCase:  authUseCase,
		qwenGateway:  qwenGateway,
		logger:       logger,
		defaultModel: "qwen3-coder-plus",
	}
}

// ChatCompletions handles chat completion requests
func (uc *ProxyUseCase) ChatCompletions(req *entities.ChatCompletionRequest) (*entities.ChatCompletionResponse, error) {
	credentials, err := uc.authUseCase.EnsureAuthenticated()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Set default model if not provided
	if req.Model == "" {
		req.Model = uc.defaultModel
	}

	// Strip Qwen-specific fields to maintain OpenAI compatibility
	// These fields are not part of OpenAI's Chat Completion API specification
	req.ReasoningEffort = ""
	req.IncludeReasoning = false

	resp, err := uc.qwenGateway.ChatCompletions(req, credentials)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		errorMsg := string(body)
		if err != nil {
			errorMsg = fmt.Sprintf("Failed to read error response: %v", err)
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// Read the entire response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response entities.ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		// Log the raw response for debugging
		uc.logger.Error("Failed to decode Qwen response", "error", err, "raw_response", string(bodyBytes))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert Qwen response format to OpenAI format if needed
	uc.convertQwenToOpenAIResponse(&response)

	return &response, nil
}

// StreamChatCompletions handles streaming chat completion requests
func (uc *ProxyUseCase) StreamChatCompletions(req *entities.ChatCompletionRequest, writer http.ResponseWriter) error {
	credentials, err := uc.authUseCase.EnsureAuthenticated()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set default model if not provided
	if req.Model == "" {
		req.Model = uc.defaultModel
	}

	// Strip Qwen-specific fields to maintain OpenAI compatibility
	// These fields are not part of OpenAI's Chat Completion API specification
	req.ReasoningEffort = ""
	req.IncludeReasoning = false

	resp, err := uc.qwenGateway.ChatCompletions(req, credentials)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		errorMsg := string(body)
		if err != nil {
			errorMsg = fmt.Sprintf("Failed to read error response: %v", err)
		}
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// Set headers for SSE
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	writer.WriteHeader(http.StatusOK)

	// Stream the response
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE format: data: {"choices":[{"delta":{"content":"..."}}],...}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle special SSE messages
			if data == "[DONE]" {
				// End of stream - send OpenAI done message
				writer.Write([]byte("data: [DONE]\n\n"))
				break
			}

			// Parse JSON response
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(data), &response); err != nil {
				uc.logger.Error("Failed to parse streaming response", "error", err)
				continue
			}

			// Convert Qwen format to OpenAI chat completion chunk format
			if id, ok := response["id"].(string); ok {
				if created, ok := response["created"].(float64); ok {
					if choices, ok := response["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							// Create OpenAI format chunk
							chunk := map[string]interface{}{
								"id":      id,
								"object":  "chat.completion.chunk",
								"created": created,
								"model":   response["model"],
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
								uc.logger.Error("Failed to marshal chunk", "error", err)
								continue
							}

							// Write as SSE data
							writer.Write([]byte("data: " + string(chunkJSON) + "\n\n"))
						}
					}
				}
			}
		}

		// Flush after each message
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// Ensure stream ends properly
	writer.Write([]byte("data: [DONE]\n\n"))
	return nil
}

// convertQwenToOpenAIResponse converts Qwen API response format to OpenAI format
func (uc *ProxyUseCase) convertQwenToOpenAIResponse(response *entities.ChatCompletionResponse) {
	for i := range response.Choices {
		choice := &response.Choices[i]

		// Handle tool calls - Qwen might return them in different formats
		if choice.Message.ToolCalls != nil {
			// Ensure tool calls are in OpenAI format
			for j := range choice.Message.ToolCalls {
				toolCall := &choice.Message.ToolCalls[j]
				// Ensure type is set
				if toolCall.Type == "" {
					toolCall.Type = "function"
				}
				// Ensure ID is set if missing
				if toolCall.ID == "" {
					toolCall.ID = fmt.Sprintf("call_%d_%d", i, j)
				}
				// Validate function structure
				if toolCall.Function.Name == "" {
					uc.logger.Warn("Tool call missing function name", "choice_index", i, "tool_index", j)
				}
			}
			// If there are tool calls, content should be null
			if len(choice.Message.ToolCalls) > 0 {
				choice.Message.Content = nil
			}
		}

		// Handle content - ensure it's properly formatted
		if choice.Message.Content == nil && len(choice.Message.ToolCalls) == 0 {
			// If no content and no tool calls, this might indicate an issue
			uc.logger.Warn("Response choice has no content and no tool calls", "choice_index", i)
		}

		// Ensure finish_reason is set appropriately for tool calls
		if len(choice.Message.ToolCalls) > 0 && choice.FinishReason == "" {
			choice.FinishReason = "tool_calls"
		}
	}
}

// GetModels returns available models
func (uc *ProxyUseCase) GetModels() []*entities.ModelInfo {
	return []*entities.ModelInfo{
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
		{
			ID:      "qwen3-coder-flash",
			Object:  "model",
			Created: 1754686206,
			OwnedBy: "qwen",
			Permission: []entities.ModelPermission{
				{
					ID:                 "modelperm-qwen3-coder-flash",
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
		{
			ID:      "vision-model",
			Object:  "model",
			Created: 1754686206,
			OwnedBy: "qwen",
			Permission: []entities.ModelPermission{
				{
					ID:                 "modelperm-vision-model",
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
}

// AuthenticateManually triggers manual OAuth2 device flow authentication
func (uc *ProxyUseCase) AuthenticateManually() error {
	return uc.authUseCase.AuthenticateManually()
}

// CheckAuthentication checks if user is currently authenticated
func (uc *ProxyUseCase) CheckAuthentication() (*entities.Credentials, error) {
	return uc.authUseCase.EnsureAuthenticated()
}
