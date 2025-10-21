package services

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"

	"github.com/stretchr/testify/assert"
)

func TestNewOAuthService(t *testing.T) {
	baseURL := "https://api.example.com"
	service := NewOAuthService(baseURL)

	oauthService, ok := service.(*OAuthService)
	assert.True(t, ok)
	assert.Equal(t, baseURL, oauthService.baseURL)
	assert.Equal(t, baseURL+"/api/v1/oauth2/device/code", oauthService.deviceAuthURL)
	assert.Equal(t, baseURL+"/api/v1/oauth2/token", oauthService.tokenURL)
	assert.NotNil(t, oauthService.httpClient)
	assert.Equal(t, 30*time.Second, oauthService.httpClient.Timeout)
}

func TestOAuthService_RefreshToken_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/oauth2/token", r.URL.Path)

		// Check the request body
		body, _ := io.ReadAll(r.Body)
		expectedBody := "grant_type=refresh_token&refresh_token=refresh-token&client_id=client-id"
		assert.Equal(t, expectedBody, string(body))

		// Check headers
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Send a successful response
		response := map[string]interface{}{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewOAuthService(server.URL)

	creds, err := service.RefreshToken("refresh-token", "client-id")
	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", creds.AccessToken)
	assert.Equal(t, "Bearer", creds.TokenType)
	assert.Equal(t, "new-refresh-token", creds.RefreshToken)
	assert.NotZero(t, creds.ExpiryDate) // Should be set to current time + expires_in
}

func TestOAuthService_RefreshToken_HTTPError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	service := NewOAuthService(server.URL)

	_, err := service.RefreshToken("refresh-token", "client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token refresh failed with status 500")
}

func TestOAuthService_RefreshToken_RequestCreationError(t *testing.T) {
	// Test with invalid URL to trigger request creation error
	service := &OAuthService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "invalid://url",
	}

	_, err := service.RefreshToken("refresh-token", "client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send request")
}

func TestOAuthService_RefreshToken_InvalidJSON(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	service := NewOAuthService(server.URL)

	_, err := service.RefreshToken("refresh-token", "client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestOAuthService_RefreshToken_ResponseError(t *testing.T) {
	// Create a mock server that returns an error in the response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":             "invalid_request",
			"error_description": "Invalid refresh token",
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewOAuthService(server.URL)

	_, err := service.RefreshToken("refresh-token", "client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token refresh failed: invalid_request - Invalid refresh token")
}

func TestNewAIService(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}

	service := NewAIService(config)

	aiService, ok := service.(*AIService)
	assert.True(t, ok)
	assert.Equal(t, config, aiService.config)
	assert.NotNil(t, aiService.httpClient)
	assert.Equal(t, 300*time.Second, aiService.httpClient.Timeout)
}

func TestAIService_GetBaseURL(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}

	service := NewAIService(config).(*AIService)

	// Test with credentials that have a resource URL
	creds := &entities.Credentials{
		ResourceURL: "https://custom.example.com",
	}

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_WithCredentialsButNoResourceURL(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}

	service := NewAIService(config).(*AIService)

	// Test with credentials that don't have a resource URL
	creds := &entities.Credentials{
		ResourceURL: "", // Empty
	}

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_WithoutCredentials(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}

	service := NewAIService(config).(*AIService)

	// Test without credentials
	var creds *entities.Credentials

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_AddsHTTPS(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "api.example.com", // No scheme
	}

	service := NewAIService(config).(*AIService)

	// Test with a URL that doesn't have HTTPS
	var creds *entities.Credentials

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_AlreadyHasHTTPS(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com", // Already has HTTPS
	}

	service := NewAIService(config).(*AIService)

	// Test with a URL that already has HTTPS
	var creds *entities.Credentials

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_AddsV1Suffix(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com", // No /v1 suffix
	}

	service := NewAIService(config).(*AIService)

	var creds *entities.Credentials

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_GetBaseURL_AlreadyHasV1Suffix(t *testing.T) {
	config := &entities.Config{
		APIBaseURL: "https://api.example.com/v1", // Already has /v1 suffix
	}

	service := NewAIService(config).(*AIService)

	var creds *entities.Credentials

	baseURL, err := service.GetBaseURL(creds, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestAIService_ChatCompletions(t *testing.T) {
	// Create a mock server for the AI service
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Send a successful response
		response := `{
			"id": "test-id",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "test response"
					},
					"finish_reason": "stop"
				}
			]
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	config := &entities.Config{
		APIBaseURL: server.URL,
	}

	service := NewAIService(config)

	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
	}

	creds := &entities.Credentials{
		AccessToken: "test-token",
	}

	resp, err := service.ChatCompletions(req, creds)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Clean up
	resp.Body.Close()
}

func TestAIService_ChatCompletions_InvalidRequest(t *testing.T) {
	// Create a mock server that won't be called
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Mock server should not be called for invalid request")
	}))
	defer server.Close()

	config := &entities.Config{
		APIBaseURL: server.URL,
	}

	service := NewAIService(config)

	// Test with nil request
	_, err := service.ChatCompletions(nil, &entities.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestAIService_ChatCompletions_InvalidCredentials(t *testing.T) {
	// Create a mock server that expects a request without Authorization header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// No Authorization header should be present when credentials are nil
		assert.Equal(t, "", r.Header.Get("Authorization"))

		// Return a 401 Unauthorized response
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	config := &entities.Config{
		APIBaseURL: server.URL,
	}

	service := NewAIService(config)

	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
	}

	// Test with invalid credentials (nil)
	resp, err := service.ChatCompletions(req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Clean up
	if resp != nil {
		resp.Body.Close()
	}
}

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := generateCodeVerifier()
	assert.NoError(t, err)
	assert.NotEmpty(t, verifier)
	// Verify it's a proper base64-encoded string
	_, err = base64.RawURLEncoding.DecodeString(verifier)
	assert.NoError(t, err)
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier"
	challenge := generateCodeChallenge(verifier)
	assert.NotEmpty(t, challenge)

	// Verify it's a proper base64-encoded string
	_, err := base64.RawURLEncoding.DecodeString(challenge)
	assert.NoError(t, err)
}

func TestOAuthService_AuthenticateWithDeviceFlow(t *testing.T) {
	// Since this method involves user interaction and external API calls,
	// we can't easily test it directly. We'll focus on testing the error paths.

	// Create an OAuthService with a non-existent base URL to trigger an error
	service := &OAuthService{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:       "http://non-existent-url-for-test",
		deviceAuthURL: "http://non-existent-url-for-test/device",
		tokenURL:      "http://non-existent-url-for-test/token",
	}

	_, err := service.AuthenticateWithDeviceFlow("test-client", "test-scope")
	assert.Error(t, err)
}
