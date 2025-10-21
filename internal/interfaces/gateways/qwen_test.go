package gateways

import (
	"crypto/sha256"
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

func TestQwenAPIGatewayImpl_ChatCompletions(t *testing.T) {
	// Create a test server to mock the Qwen API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Send a successful response
		response := map[string]interface{}{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "test response",
					},
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create a Qwen API gateway with the test server URL
	config := &entities.Config{
		APIBaseURL: server.URL,
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Create a chat completion request
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
	}

	// Create credentials with access token
	credentials := &entities.Credentials{
		AccessToken: "test-token",
	}

	// Call the ChatCompletions method
	httpResp, err := gateway.ChatCompletions(req, credentials)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	// Read and verify the response body
	body, err := io.ReadAll(httpResp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "test response")

	// Clean up
	httpResp.Body.Close()
}

func TestQwenAPIGatewayImpl_ChatCompletions_Error(t *testing.T) {
	// Create a Qwen API gateway with an invalid URL
	config := &entities.Config{
		APIBaseURL: "http://invalid-url-that-does-not-exist",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Create a chat completion request
	req := &entities.ChatCompletionRequest{
		Model: "test-model",
		Messages: []entities.ChatMessage{
			{
				Role:    "user",
				Content: "test message",
			},
		},
	}

	// Create credentials with access token
	credentials := &entities.Credentials{
		AccessToken: "test-token",
	}

	// Call the ChatCompletions method - should return an error
	_, err := gateway.ChatCompletions(req, credentials)
	assert.Error(t, err)
	// The error should be a network error since the URL is invalid
	assert.Contains(t, err.Error(), "invalid-url-that-does-not-exist")
}

func TestQwenAPIGatewayImpl_GetBaseURL(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with credentials that have a resource URL
	credentials := &entities.Credentials{
		ResourceURL: "https://custom.example.com",
	}

	baseURL, err := gateway.GetBaseURL(credentials, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com/v1", baseURL)
}

func TestQwenAPIGatewayImpl_GetBaseURL_NoCredentials(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with nil credentials
	baseURL, err := gateway.GetBaseURL(nil, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestQwenAPIGatewayImpl_GetBaseURL_NoResourceURL(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with credentials that don't have a resource URL
	credentials := &entities.Credentials{
		AccessToken: "test-token",
		// No ResourceURL
	}

	baseURL, err := gateway.GetBaseURL(credentials, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestQwenAPIGatewayImpl_GetBaseURL_InvalidResourceURL(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "https://api.example.com",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with credentials that have an invalid resource URL
	credentials := &entities.Credentials{
		ResourceURL: "://invalid-url",
	}

	_, err := gateway.GetBaseURL(credentials, config.APIBaseURL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse resource URL")
}

func TestQwenAPIGatewayImpl_GetBaseURL_EmptyURL(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with empty default URL and no credentials
	_, err := gateway.GetBaseURL(nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base URL cannot be empty")
}

func TestQwenAPIGatewayImpl_GetBaseURL_MissingScheme(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "api.example.com",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with URL that doesn't have a scheme
	baseURL, err := gateway.GetBaseURL(nil, config.APIBaseURL)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", baseURL)
}

func TestQwenAPIGatewayImpl_GetBaseURL_MissingHost(t *testing.T) {
	// Create a Qwen API gateway
	config := &entities.Config{
		APIBaseURL: "https://",
	}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with URL that has no host
	_, err := gateway.GetBaseURL(nil, config.APIBaseURL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing host")
}

func TestOAuthGatewayImpl_RefreshToken(t *testing.T) {
	// Create a test server to mock the OAuth token refresh endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/oauth2/token", r.URL.Path)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Read the request body to verify the form data
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, "grant_type=refresh_token")
		assert.Contains(t, bodyStr, "refresh_token=test-refresh-token")
		assert.Contains(t, bodyStr, "client_id=test-client-id")

		// Send a successful response
		response := map[string]interface{}{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create an OAuth gateway with the test server URL
	gateway := NewOAuthGateway(server.URL).(*OAuthGatewayImpl)

	// Call the RefreshToken method
	creds, err := gateway.RefreshToken("test-refresh-token", "test-client-id")
	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", creds.AccessToken)
	assert.Equal(t, "Bearer", creds.TokenType)
	assert.Equal(t, "new-refresh-token", creds.RefreshToken)
	// Verify that the expiry date is approximately correct (within 10 seconds)
	expectedExpiry := time.Now().UnixMilli() + 3600000         // 3600 seconds = 1 hour
	assert.InDelta(t, expectedExpiry, creds.ExpiryDate, 10000) // 10 second tolerance
}

func TestOAuthGatewayImpl_RefreshToken_Error(t *testing.T) {
	// Create a test server to return an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errorResponse := map[string]string{
			"error":             "invalid_grant",
			"error_description": "The provided authorization grant is invalid",
		}
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer server.Close()

	// Create an OAuth gateway with the test server URL
	gateway := NewOAuthGateway(server.URL).(*OAuthGatewayImpl)

	// Call the RefreshToken method - should return an error
	_, err := gateway.RefreshToken("invalid-refresh-token", "test-client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token refresh failed with status 400")
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestOAuthGatewayImpl_RefreshToken_NetworkError(t *testing.T) {
	// Create an OAuth gateway with an invalid URL
	gateway := NewOAuthGateway("http://invalid-url-that-does-not-exist").(*OAuthGatewayImpl)

	// Call the RefreshToken method - should return a network error
	_, err := gateway.RefreshToken("test-refresh-token", "test-client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid-url-that-does-not-exist")
}

func TestOAuthGatewayImpl_RefreshToken_ResponseDecodeError(t *testing.T) {
	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Send invalid JSON
		w.Write([]byte("{ invalid json }"))
	}))
	defer server.Close()

	// Create an OAuth gateway with the test server URL
	gateway := NewOAuthGateway(server.URL).(*OAuthGatewayImpl)

	// Call the RefreshToken method - should return a decode error
	_, err := gateway.RefreshToken("test-refresh-token", "test-client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestOAuthGatewayImpl_RefreshToken_ErrorInResponse(t *testing.T) {
	// Create a test server that returns an error in the response body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Send response with error
		errorResponse := map[string]string{
			"error":             "invalid_request",
			"error_description": "The request is missing a required parameter",
		}
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer server.Close()

	// Create an OAuth gateway with the test server URL
	gateway := NewOAuthGateway(server.URL).(*OAuthGatewayImpl)

	// Call the RefreshToken method - should return an error from response body
	_, err := gateway.RefreshToken("test-refresh-token", "test-client-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token refresh failed: invalid_request")
	assert.Contains(t, err.Error(), "The request is missing a required parameter")
}

func TestGenerateCodeVerifier(t *testing.T) {
	// Test that generateCodeVerifier creates a valid code verifier
	verifier, err := generateCodeVerifier()
	assert.NoError(t, err)
	assert.NotEmpty(t, verifier)
	// Verify that it's a valid base64-encoded string
	_, err = base64.RawURLEncoding.DecodeString(verifier)
	assert.NoError(t, err)
	// Verify it has the expected length (32 bytes -> base64 encoded)
	assert.Equal(t, 43, len(verifier)) // 32 bytes encoded with base64 raw URL encoding
}

func TestGenerateCodeChallenge(t *testing.T) {
	// Test that generateCodeChallenge creates a valid code challenge from a verifier
	codeVerifier := "test-code-verifier"
	challenge := generateCodeChallenge(codeVerifier)

	// Verify it's a valid base64-encoded string
	_, err := base64.RawURLEncoding.DecodeString(challenge)
	assert.NoError(t, err)
	// The challenge should be 43 characters (32 bytes SHA256 hash encoded in base64 raw URL)
	assert.Equal(t, 43, len(challenge))

	// Verify that it's the SHA256 hash of the code verifier
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	assert.Equal(t, expectedChallenge, challenge)
}
