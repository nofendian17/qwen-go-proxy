// Package interfaces defines the core domain interfaces that must be implemented
// by outer layers. These interfaces represent the contracts that our domain
// business logic depends on, following the Dependency Inversion Principle.
package interfaces

import (
	"net/http"

	"qwen-go-proxy/internal/domain/entities"
)

// CredentialRepository defines the interface for credential storage and retrieval.
// This is a domain interface that represents the contract for storing authentication
// credentials. Implementations should be provided by the infrastructure layer.
type CredentialRepository interface {
	// Load retrieves stored credentials from the repository
	Load() (*entities.Credentials, error)

	// Save persists credentials to the repository
	Save(credentials *entities.Credentials) error
}

// OAuthService defines the interface for OAuth authentication operations.
// This interface represents the contract for OAuth-related functionality
// that our domain needs, abstracting the external OAuth provider.
type OAuthService interface {
	// RefreshToken refreshes an existing access token using the refresh token
	RefreshToken(refreshToken, clientID string) (*entities.Credentials, error)

	// AuthenticateWithDeviceFlow performs OAuth2 device authorization flow
	AuthenticateWithDeviceFlow(clientID, scope string) (*entities.Credentials, error)
}

// AIService defines the interface for AI model interactions.
// This interface abstracts the external AI API that our application uses.
type AIService interface {
	// ChatCompletions sends a chat completion request to the AI service
	ChatCompletions(req *entities.ChatCompletionRequest, credentials *entities.Credentials) (*http.Response, error)

	// GetBaseURL returns the appropriate base URL for API calls
	GetBaseURL(credentials *entities.Credentials, defaultURL string) (string, error)
}

// StreamingService defines the interface for handling streaming responses.
// This interface abstracts the processing of streaming AI responses.
type StreamingService interface {
	// ProcessStreamingResponse processes a streaming response from the AI service
	ProcessStreamingResponse(response any, writer any) error
}