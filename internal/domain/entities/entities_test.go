package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCredentialsSanitize(t *testing.T) {
	creds := &Credentials{
		AccessToken:  "secret-access-token",
		TokenType:    "Bearer",
		RefreshToken: "secret-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	sanitized := creds.Sanitize()

	// Check that sensitive data is not exposed
	assert.NotEqual(t, creds.AccessToken, sanitized["access_token"])
	assert.NotEqual(t, creds.RefreshToken, sanitized["refresh_token"])

	// Check that non-sensitive data is preserved
	assert.Equal(t, creds.TokenType, sanitized["token_type"])
	assert.Equal(t, creds.ExpiryDate, sanitized["expiry_date"])
	assert.Equal(t, creds.ResourceURL, sanitized["resource_url"])
	assert.Equal(t, true, sanitized["has_token"])
}

func TestCredentialsSanitize_Nil(t *testing.T) {
	var creds *Credentials
	sanitized := creds.Sanitize()
	assert.Nil(t, sanitized)
}

func TestCredentialsSanitize_Empty(t *testing.T) {
	creds := &Credentials{}
	sanitized := creds.Sanitize()

	assert.Equal(t, creds.TokenType, sanitized["token_type"])
	assert.Equal(t, creds.ExpiryDate, sanitized["expiry_date"])
	assert.Equal(t, creds.ResourceURL, sanitized["resource_url"])
	assert.Equal(t, false, sanitized["has_token"])
}

func TestConfigGetServerAddress(t *testing.T) {
	config := &Config{
		ServerHost: "localhost",
		ServerPort: 8080,
	}

	address := config.GetServerAddress()
	assert.Equal(t, "localhost:8080", address)
}

func TestConfigGetServerAddress_Nil(t *testing.T) {
	var config *Config
	address := config.GetServerAddress()
	assert.Equal(t, ":8080", address)
}

func TestConfigGetServerAddress_Default(t *testing.T) {
	config := &Config{
		ServerPort: 8080, // Default port is set by env tag
	}
	address := config.GetServerAddress()
	assert.Equal(t, ":8080", address)
}

func TestConfigGetServerAddress_Custom(t *testing.T) {
	config := &Config{
		ServerHost: "0.0.0.0",
		ServerPort: 3000,
	}

	address := config.GetServerAddress()
	assert.Equal(t, "0.0.0.0:3000", address)
}

func TestCompletionRequestValidation(t *testing.T) {
	// Test valid request
	req := &CompletionRequest{
		Model:       "test-model",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.5,
	}

	// Since we don't have a validation function exposed, we'll just test that the struct can be created
	assert.Equal(t, "test-model", req.Model)
	assert.Equal(t, "test prompt", req.Prompt)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, 0.5, req.Temperature)
}

func TestChatCompletionRequestValidation(t *testing.T) {
	// Test valid request
	req := &ChatCompletionRequest{
		Model:       "test-model",
		Messages:    []ChatMessage{{Role: "user", Content: "test message"}},
		MaxTokens:   100,
		Temperature: 0.5,
	}

	// Since we don't have a validation function exposed, we'll just test that the struct can be created
	assert.Equal(t, "test-model", req.Model)
	assert.Equal(t, 1, len(req.Messages))
	assert.Equal(t, "user", req.Messages[0].Role)
	assert.Equal(t, "test message", req.Messages[0].Content)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, 0.5, req.Temperature)
}

func TestChatMessageValidation(t *testing.T) {
	// Test valid message
	msg := ChatMessage{
		Role:    "user",
		Content: "test message",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "test message", msg.Content)
}

func TestChatMessageInvalidRole(t *testing.T) {
	// Test invalid role - struct allows it but validation would catch it
	msg := ChatMessage{
		Role:    "invalid-role",
		Content: "test message",
	}

	assert.Equal(t, "invalid-role", msg.Role)
	// Note: The struct itself doesn't validate, validation would happen in a separate validator
}

func TestChatMessageValidRoles(t *testing.T) {
	validRoles := []string{"system", "user", "assistant", "tool"}

	for _, role := range validRoles {
		msg := ChatMessage{
			Role:    role,
			Content: "test message",
		}
		assert.Equal(t, role, msg.Role)
		assert.Equal(t, "test message", msg.Content)
	}
}

func TestResponseFormat(t *testing.T) {
	format := &ResponseFormat{
		Type:       "json_object",
		JSONSchema: map[string]interface{}{"type": "object"},
	}

	assert.Equal(t, "json_object", format.Type)
	assert.NotNil(t, format.JSONSchema)
}

func TestTool(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: Function{
			Name:        "test_function",
			Description: "A test function",
			Parameters:  map[string]interface{}{"type": "object"},
		},
	}

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "test_function", tool.Function.Name)
	assert.Equal(t, "A test function", tool.Function.Description)
	assert.NotNil(t, tool.Function.Parameters)
}

func TestToolChoice(t *testing.T) {
	choice := ToolChoice{
		Type: "function",
		Function: Function{
			Name: "test_function",
		},
	}

	assert.Equal(t, "function", choice.Type)
	assert.Equal(t, "test_function", choice.Function.Name)
}

func TestStreamOptions(t *testing.T) {
	options := &StreamOptions{
		IncludeUsage: true,
	}

	assert.True(t, options.IncludeUsage)
}

func TestCompletionResponse(t *testing.T) {
	response := CompletionResponse{
		ID:      "test-id",
		Object:  "test-object",
		Created: time.Now().Unix(),
		Model:   "test-model",
		Choices: []CompletionChoice{
			{
				Text:         "test text",
				Index:        0,
				FinishReason: "stop",
			},
		},
	}

	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "test-object", response.Object)
	assert.Equal(t, "test-model", response.Model)
	assert.Equal(t, 1, len(response.Choices))
	assert.Equal(t, "test text", response.Choices[0].Text)
}

func TestChatCompletionResponse(t *testing.T) {
	response := ChatCompletionResponse{
		ID:      "test-id",
		Object:  "test-object",
		Created: time.Now().Unix(),
		Model:   "test-model",
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			},
		},
	}

	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "test-object", response.Object)
	assert.Equal(t, "test-model", response.Model)
	assert.Equal(t, 1, len(response.Choices))
}
