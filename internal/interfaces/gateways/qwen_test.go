package gateways

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestOAuthGatewayImpl_GenerateCodeVerifier(t *testing.T) {
	// Test the generateCodeVerifier function directly
	verifier1, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("Failed to generate code verifier: %v", err)
	}

	verifier2, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("Failed to generate code verifier: %v", err)
	}

	// Verifiers should be different (random)
	if verifier1 == verifier2 {
		t.Error("Code verifiers should be different")
	}

	// Should be base64 URL encoded (no padding, URL safe chars)
	if len(verifier1) != 43 { // 32 bytes = 43 chars in base64url
		t.Errorf("Expected verifier length 43, got %d", len(verifier1))
	}
}

func TestOAuthGatewayImpl_GenerateCodeChallenge(t *testing.T) {
	testVerifier := "test_verifier_string"
	challenge := generateCodeChallenge(testVerifier)

	// Just verify it's not empty and has expected length for SHA-256 base64url
	if len(challenge) == 0 {
		t.Error("Code challenge should not be empty")
	}

	// SHA-256 hash encoded as base64url should be 43 characters
	if len(challenge) != 43 {
		t.Errorf("Expected challenge length 43, got %d", len(challenge))
	}

	// Should only contain URL-safe characters
	for _, char := range challenge {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			t.Errorf("Invalid character in challenge: %c", char)
		}
	}
}

func TestOAuthGatewayImpl_NewOAuthGateway(t *testing.T) {
	baseURL := "https://chat.qwen.ai"
	gateway := NewOAuthGateway(baseURL).(*OAuthGatewayImpl)

	if gateway.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, gateway.baseURL)
	}

	expectedDeviceURL := baseURL + "/api/v1/oauth2/device/code"
	if gateway.deviceAuthURL != expectedDeviceURL {
		t.Errorf("Expected deviceAuthURL %s, got %s", expectedDeviceURL, gateway.deviceAuthURL)
	}

	expectedTokenURL := baseURL + "/api/v1/oauth2/token"
	if gateway.tokenURL != expectedTokenURL {
		t.Errorf("Expected tokenURL %s, got %s", expectedTokenURL, gateway.tokenURL)
	}

	if gateway.httpClient == nil {
		t.Error("HTTP client should not be nil")
	}

	if gateway.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", gateway.httpClient.Timeout)
	}
}

// Negative Test Cases

func TestOAuthGatewayImpl_NewOAuthGateway_EmptyBaseURL(t *testing.T) {
	// Test with empty base URL - should not panic but create gateway with empty URLs
	gateway := NewOAuthGateway("").(*OAuthGatewayImpl)

	assert.Equal(t, "", gateway.baseURL)
	assert.Equal(t, "/api/v1/oauth2/device/code", gateway.deviceAuthURL)
	assert.Equal(t, "/api/v1/oauth2/token", gateway.tokenURL)
	assert.NotNil(t, gateway.httpClient)
}

func TestOAuthGatewayImpl_NewOAuthGateway_InvalidBaseURL(t *testing.T) {
	// Test with invalid base URL - should not panic
	gateway := NewOAuthGateway("not-a-valid-url").(*OAuthGatewayImpl)

	assert.Equal(t, "not-a-valid-url", gateway.baseURL)
	assert.Equal(t, "not-a-valid-url/api/v1/oauth2/device/code", gateway.deviceAuthURL)
	assert.Equal(t, "not-a-valid-url/api/v1/oauth2/token", gateway.tokenURL)
	assert.NotNil(t, gateway.httpClient)
}

func TestOAuthGatewayImpl_GenerateCodeVerifier_InvalidLength(t *testing.T) {
	// Test that verifier generation doesn't produce unexpected lengths
	verifier, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("Failed to generate code verifier: %v", err)
	}

	// Should be exactly 43 characters (32 bytes base64url encoded)
	assert.Equal(t, 43, len(verifier))
}

func TestOAuthGatewayImpl_GenerateCodeVerifier_Consistency(t *testing.T) {
	// Test that multiple calls produce different verifiers (randomness)
	verifiers := make(map[string]bool)
	for i := 0; i < 10; i++ {
		verifier, err := generateCodeVerifier()
		if err != nil {
			t.Fatalf("Failed to generate code verifier: %v", err)
		}
		verifiers[verifier] = true
	}

	// Should have 10 unique verifiers (extremely unlikely to have collisions)
	assert.Equal(t, 10, len(verifiers))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_EmptyVerifier(t *testing.T) {
	// Test with empty verifier
	challenge := generateCodeChallenge("")
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_LongVerifier(t *testing.T) {
	// Test with very long verifier
	longVerifier := strings.Repeat("a", 1000)
	challenge := generateCodeChallenge(longVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_SpecialCharacters(t *testing.T) {
	// Test with verifier containing special characters
	specialVerifier := "!@#$%^&*()_+{}|:<>?[]\\;',./"
	challenge := generateCodeChallenge(specialVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_UnicodeVerifier(t *testing.T) {
	// Test with verifier containing unicode characters
	unicodeVerifier := "测试verifier🔥"
	challenge := generateCodeChallenge(unicodeVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_VerifierWithSpaces(t *testing.T) {
	// Test with verifier containing spaces
	spacedVerifier := "test verifier with spaces"
	challenge := generateCodeChallenge(spacedVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_VerifierWithNewlines(t *testing.T) {
	// Test with verifier containing newlines
	newlineVerifier := "test\nverifier\r\nwith\nnewlines"
	challenge := generateCodeChallenge(newlineVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

func TestOAuthGatewayImpl_GenerateCodeChallenge_VerifierWithBinaryData(t *testing.T) {
	// Test with verifier containing binary-like data
	binaryVerifier := string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD})
	challenge := generateCodeChallenge(binaryVerifier)
	assert.NotEmpty(t, challenge)
	assert.Equal(t, 43, len(challenge))
}

// New tests for QwenAPIGateway and OAuth Refresh using gomock and httptest where appropriate
func TestQwenAPIGatewayImpl_NewQwenAPIGateway(t *testing.T) {
	config := &entities.Config{APIBaseURL: "https://api.example.com"}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	assert.NotNil(t, gateway.httpClient)
	assert.Equal(t, config, gateway.config)
	assert.Equal(t, 300*time.Second, gateway.httpClient.Timeout)
}

func TestQwenAPIGatewayImpl_GetBaseURL(t *testing.T) {
	config := &entities.Config{APIBaseURL: "https://api.example.com"}
	gateway := NewQwenAPIGateway(config).(*QwenAPIGatewayImpl)

	// Test with default
	url, err := gateway.GetBaseURL(nil, "https://default.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://default.com/v1", url)

	// Test with credentials
	creds := &entities.Credentials{ResourceURL: "https://custom.com"}
	url, err = gateway.GetBaseURL(creds, "https://default.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.com/v1", url)

	// Test without https
	creds2 := &entities.Credentials{ResourceURL: "api.example.com"}
	url, err = gateway.GetBaseURL(creds2, "https://default.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", url)

	// Test with v1 already
	creds3 := &entities.Credentials{ResourceURL: "https://api.example.com/v1"}
	url, err = gateway.GetBaseURL(creds3, "https://default.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", url)
}

func TestQwenAPIGateway_ChatCompletions_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGateway := mocks.NewMockQwenAPIGateway(ctrl)

	// prepare fake response
	body := `{"id":"test","object":"chat.completion"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	// Expect ChatCompletions to be called with any request and credentials and return our fake response
	mockGateway.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(resp, nil)

	// call the mock
	req := &entities.ChatCompletionRequest{}
	creds := &entities.Credentials{AccessToken: "test_token"}
	gotResp, err := mockGateway.ChatCompletions(req, creds)
	if err != nil {
		t.Fatalf("unexpected error from mock: %v", err)
	}
	assert.Equal(t, http.StatusOK, gotResp.StatusCode)
}

func TestOAuthGatewayImpl_RefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/oauth2/token" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected content type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("Expected grant_type refresh_token, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old_refresh_token" {
			t.Errorf("Expected refresh_token old_refresh_token, got %s", r.FormValue("refresh_token"))
		}
		if r.FormValue("client_id") != "client_id" {
			t.Errorf("Expected client_id client_id, got %s", r.FormValue("client_id"))
		}
		// Mock response
		response := `{
			"access_token": "new_access_token",
			"token_type": "Bearer",
			"refresh_token": "new_refresh_token",
			"expires_in": 3600
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	gateway := NewOAuthGateway(server.URL).(*OAuthGatewayImpl)
	creds, err := gateway.RefreshToken("old_refresh_token", "client_id")
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	assert.Equal(t, "new_access_token", creds.AccessToken)
	assert.Equal(t, "Bearer", creds.TokenType)
	assert.Equal(t, "new_refresh_token", creds.RefreshToken)
	assert.True(t, creds.ExpiryDate > time.Now().UnixMilli())
}
