package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/interfaces/gateways"
	"qwen-go-proxy/internal/usecases/auth"
	"qwen-go-proxy/internal/usecases/streaming"
)

// ProxyUseCaseInterface defines the interface for proxy use case operations
type ProxyUseCaseInterface interface {
	ChatCompletions(req *entities.ChatCompletionRequest) (*entities.ChatCompletionResponse, error)
	StreamChatCompletions(req *entities.ChatCompletionRequest, writer http.ResponseWriter) error
	GetModels() ([]*entities.ModelInfo, error)
	AuthenticateManually() error
	CheckAuthentication() (*entities.Credentials, error)
}

// ProxyUseCase defines the API proxy use case
type ProxyUseCase struct {
	authUseCase      auth.AuthUseCaseInterface
	qwenGateway      gateways.QwenAPIGateway
	streamingUseCase streaming.StreamingUseCaseInterface
	logger           logging.LoggerInterface
	defaultModel     string
}

// NewProxyUseCase creates a new proxy use case
func NewProxyUseCase(authUseCase auth.AuthUseCaseInterface, qwenGateway gateways.QwenAPIGateway, streamingUseCase streaming.StreamingUseCaseInterface, logger logging.LoggerInterface) *ProxyUseCase {
	if authUseCase == nil {
		panic("authUseCase cannot be nil")
	}
	if qwenGateway == nil {
		panic("qwenGateway cannot be nil")
	}
	if streamingUseCase == nil {
		panic("streamingUseCase cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	return &ProxyUseCase{
		authUseCase:      authUseCase,
		qwenGateway:      qwenGateway,
		streamingUseCase: streamingUseCase,
		logger:           logger,
		defaultModel:     "qwen3-coder-plus",
	}
}

// ChatCompletions handles chat completion requests
func (uc *ProxyUseCase) ChatCompletions(req *entities.ChatCompletionRequest) (*entities.ChatCompletionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
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

// StreamChatCompletions handles streaming chat completion requests with advanced features
func (uc *ProxyUseCase) StreamChatCompletions(req *entities.ChatCompletionRequest, writer http.ResponseWriter) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}
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

	// Use the advanced streaming usecase for processing
	return uc.streamingUseCase.ProcessStreamingResponse(context.Background(), resp, writer)
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
func (uc *ProxyUseCase) GetModels() ([]*entities.ModelInfo, error) {
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
	}, nil
}

// AuthenticateManually triggers manual OAuth2 device flow authentication
func (uc *ProxyUseCase) AuthenticateManually() error {
	return uc.authUseCase.AuthenticateManually()
}

// CheckAuthentication checks if user is currently authenticated
func (uc *ProxyUseCase) CheckAuthentication() (*entities.Credentials, error) {
	return uc.authUseCase.EnsureAuthenticated()
}
