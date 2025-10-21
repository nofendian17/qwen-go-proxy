package auth

import (
	"fmt"
	"sync"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/domain/interfaces"
	"qwen-go-proxy/internal/infrastructure/logging"
)

// AuthUseCase defines the authentication use case
type AuthUseCase struct {
	config         *entities.Config
	oauthService   interfaces.OAuthService
	credentialRepo interfaces.CredentialRepository
	logger         *logging.Logger
	tokenMutex     sync.RWMutex
	refreshMutex   sync.Mutex
}

// NewAuthUseCase creates a new authentication use case
func NewAuthUseCase(config *entities.Config, oauthService interfaces.OAuthService, credentialRepo interfaces.CredentialRepository, logger *logging.Logger) *AuthUseCase {
	if config == nil {
		panic("config cannot be nil")
	}
	if oauthService == nil {
		panic("oauthService cannot be nil")
	}
	if credentialRepo == nil {
		panic("credentialRepo cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	return &AuthUseCase{
		config:         config,
		oauthService:   oauthService,
		credentialRepo: credentialRepo,
		logger:         logger,
	}
}

// EnsureAuthenticated ensures valid credentials are available, performing device auth if needed
func (uc *AuthUseCase) EnsureAuthenticated() (*entities.Credentials, error) {
	uc.tokenMutex.RLock()
	credentials, err := uc.credentialRepo.Load()
	uc.tokenMutex.RUnlock()

	if err != nil {
		// No credentials found, perform device authentication
		uc.logger.Info("No credentials found, initiating device authentication")
		return uc.authenticateWithDeviceFlow()
	}

	uc.tokenMutex.RLock()
	// Check if token is expired or close to expiring
	now := time.Now().UnixMilli()
	timeUntilExpiry := credentials.ExpiryDate - now
	bufferInMillis := uc.config.TokenRefreshBuffer.Milliseconds()
	isExpired := timeUntilExpiry < bufferInMillis
	uc.tokenMutex.RUnlock()

	if isExpired {
		// Use write lock for refresh operation to prevent concurrent refreshes
		uc.refreshMutex.Lock()
		defer uc.refreshMutex.Unlock()

		// Double-check after acquiring lock - reload credentials in case another goroutine refreshed
		uc.tokenMutex.RLock()
		credentials, err = uc.credentialRepo.Load()
		if err != nil {
			uc.tokenMutex.RUnlock()
			return nil, fmt.Errorf("failed to reload credentials: %w", err)
		}
		now = time.Now().UnixMilli()
		timeUntilExpiry = credentials.ExpiryDate - now
		isStillExpired := timeUntilExpiry < bufferInMillis
		uc.tokenMutex.RUnlock()

		if isStillExpired {
			uc.logger.Info("Qwen token expired or close to expiring, refreshing")
			newCredentials, err := uc.refreshAccessToken(credentials)
			if err != nil {
				uc.logger.Warn("Failed to refresh token, falling back to device authentication", "error", err)
				return uc.authenticateWithDeviceFlow()
			}
			return newCredentials, nil
		} else {
			// Another goroutine already refreshed, reload the new credentials
			uc.tokenMutex.RLock()
			defer uc.tokenMutex.RUnlock()
			return uc.credentialRepo.Load()
		}
	}

	return credentials, nil
}

// refreshAccessToken refreshes the access token
func (uc *AuthUseCase) refreshAccessToken(credentials *entities.Credentials) (*entities.Credentials, error) {
	if credentials.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available in credentials")
	}

	newCredentials, err := uc.oauthService.RefreshToken(credentials.RefreshToken, uc.config.QWENOAuthClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Keep existing resource URL
	newCredentials.ResourceURL = credentials.ResourceURL

	uc.tokenMutex.Lock()
	defer uc.tokenMutex.Unlock()

	if err := uc.credentialRepo.Save(newCredentials); err != nil {
		return nil, fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	return newCredentials, nil
}

// authenticateWithDeviceFlow performs OAuth2 device authorization flow
func (uc *AuthUseCase) authenticateWithDeviceFlow() (*entities.Credentials, error) {
	credentials, err := uc.oauthService.AuthenticateWithDeviceFlow(uc.config.QWENOAuthClientID, uc.config.QWENOAuthScope)
	if err != nil {
		return nil, fmt.Errorf("device authentication failed: %w", err)
	}

	// Save the credentials
	uc.tokenMutex.Lock()
	defer uc.tokenMutex.Unlock()

	if err := uc.credentialRepo.Save(credentials); err != nil {
		return nil, fmt.Errorf("failed to save credentials: %w", err)
	}

	uc.logger.Info("Device authentication successful, credentials saved")
	return credentials, nil
}

// AuthenticateManually triggers the OAuth2 device flow authentication manually
func (uc *AuthUseCase) AuthenticateManually() error {
	uc.logger.Info("Manual authentication requested")
	_, err := uc.authenticateWithDeviceFlow()
	return err
}

// CheckAuthentication checks if authentication is available without performing device flow
func (uc *AuthUseCase) CheckAuthentication() (*entities.Credentials, error) {
	return uc.EnsureAuthenticated()
}

// AuthUseCaseInterface defines the interface for authentication operations
type AuthUseCaseInterface interface {
	EnsureAuthenticated() (*entities.Credentials, error)
	AuthenticateManually() error
	CheckAuthentication() (*entities.Credentials, error)
}
