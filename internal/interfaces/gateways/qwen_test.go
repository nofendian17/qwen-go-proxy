package gateways

import (
	"testing"
	"time"
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
