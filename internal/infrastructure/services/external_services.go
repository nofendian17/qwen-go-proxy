// Package services contains infrastructure implementations of external service interfaces.
// This package provides concrete implementations for communicating with external APIs.
package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/domain/interfaces"

	"golang.org/x/oauth2"
)

// OAuthService implements the OAuthService interface for OAuth2 operations.
type OAuthService struct {
	httpClient    *http.Client
	baseURL       string
	deviceAuthURL string
	tokenURL      string
}

// NewOAuthService creates a new OAuth service implementation.
func NewOAuthService(baseURL string) interfaces.OAuthService {
	return &OAuthService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // TokenRefreshTimeout
		},
		baseURL:       baseURL,
		deviceAuthURL: baseURL + "/api/v1/oauth2/device/code",
		tokenURL:      baseURL + "/api/v1/oauth2/token",
	}
}

// RefreshToken refreshes the access token using the refresh token.
func (s *OAuthService) RefreshToken(refreshToken, clientID string) (*entities.Credentials, error) {
	// Prepare form data
	data := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s",
		refreshToken, clientID)

	// Create request
	req, err := http.NewRequest("POST", s.baseURL+"/api/v1/oauth2/token", strings.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		errorMsg := string(body)
		if err != nil {
			errorMsg = fmt.Sprintf("Failed to read error response: %v", err)
		}
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	var tokenData struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error,omitempty"`
		ErrorDesc    string `json:"error_description,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenData.Error != "" {
		return nil, fmt.Errorf("token refresh failed: %s - %s", tokenData.Error, tokenData.ErrorDesc)
	}

	return &entities.Credentials{
		AccessToken:  tokenData.AccessToken,
		TokenType:    tokenData.TokenType,
		RefreshToken: tokenData.RefreshToken,
		ExpiryDate:   time.Now().UnixMilli() + int64(tokenData.ExpiresIn*1000),
	}, nil
}

// AuthenticateWithDeviceFlow performs OAuth2 device authorization flow with PKCE.
func (s *OAuthService) AuthenticateWithDeviceFlow(clientID, scope string) (*entities.Credentials, error) {
	conf := &oauth2.Config{
		ClientID: clientID,
		Scopes:   []string{scope},
		Endpoint: oauth2.Endpoint{
			TokenURL:      s.tokenURL,
			DeviceAuthURL: s.deviceAuthURL,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	deviceAuthResponse, err := conf.DeviceAuth(ctx,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start device auth flow: %w", err)
	}

	// Construct verification URL with user code and client parameter
	var verificationURL string
	if deviceAuthResponse.VerificationURIComplete != "" {
		verificationURL = deviceAuthResponse.VerificationURIComplete
	} else {
		verificationURL = fmt.Sprintf("%s?user_code=%s&client=qwen-code", deviceAuthResponse.VerificationURI, deviceAuthResponse.UserCode)
	}

	// Try to open the verification URI in the browser
	if err := openBrowser(verificationURL); err != nil {
		fmt.Printf("Failed to open browser automatically: %v. Please open the URL manually.\n", err)
	}

	fmt.Printf("\n=== Qwen OAuth Authentication ===\n")
	fmt.Printf("If your browser didn't open, please go to: %s\n", verificationURL)
	fmt.Printf("And enter this code: %s\n\n", deviceAuthResponse.UserCode)
	fmt.Println("Waiting for authorization...")

	token, err := conf.DeviceAccessToken(ctx, deviceAuthResponse, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	creds := &entities.Credentials{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		ExpiryDate:   token.Expiry.UnixMilli(),
	}
	if resourceURL, ok := token.Extra("resource_url").(string); ok {
		creds.ResourceURL = resourceURL
	}

	fmt.Println("Authentication successful! Credentials obtained.")
	return creds, nil
}

// AIService implements the AIService interface for AI API operations.
type AIService struct {
	httpClient *http.Client
	config     *entities.Config
}

// NewAIService creates a new AI service implementation.
func NewAIService(config *entities.Config) interfaces.AIService {
	return &AIService{
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // DefaultHTTPTimeout
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		config: config,
	}
}

// ChatCompletions makes a chat completion request to the AI API.
func (s *AIService) ChatCompletions(req *entities.ChatCompletionRequest, credentials *entities.Credentials) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	baseURL, err := s.GetBaseURL(credentials, s.config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get base URL: %w", err)
	}

	qwenURL := baseURL + "/chat/completions"

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", qwenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if credentials != nil {
		httpReq.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	}

	return s.httpClient.Do(httpReq)
}

// GetBaseURL returns the base URL for API calls.
func (s *AIService) GetBaseURL(credentials *entities.Credentials, defaultURL string) (string, error) {
	baseURL := defaultURL
	if credentials != nil && credentials.ResourceURL != "" {
		baseURL = credentials.ResourceURL
	}

	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	return baseURL, nil
}

// Helper functions

// openBrowser opens the default browser with the given URL.
func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

// generateCodeVerifier generates a random code verifier for PKCE.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateCodeChallenge generates a code challenge from a code verifier using SHA-256.
func generateCodeChallenge(codeVerifier string) string {
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
