package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/interfaces/gateways"
)

// CredentialRepository defines the interface for credential storage
type CredentialRepository interface {
	Load() (*entities.Credentials, error)
	Save(credentials *entities.Credentials) error
}

// FileCredentialRepository implements CredentialRepository using file storage
type FileCredentialRepository struct {
	filePath string
}

// NewFileCredentialRepository creates a new file-based credential repository
func NewFileCredentialRepository(qwenDir string) CredentialRepository {
	// Use current working directory as base path
	workDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get current working directory: %v", err))
	}
	return &FileCredentialRepository{
		filePath: filepath.Join(workDir, qwenDir, "oauth_creds.json"),
	}
}

// Load loads credentials from file
func (r *FileCredentialRepository) Load() (*entities.Credentials, error) {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read Qwen OAuth credentials: %w", err)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("failed to read Qwen OAuth credentials: permission denied. The credentials file exists but is not readable by the application user. Please ensure the file permissions allow read access.")
		}
		return nil, fmt.Errorf("failed to read Qwen OAuth credentials: %w", err)
	}

	var creds entities.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen OAuth credentials: %w", err)
	}

	return &creds, nil
}

// Save saves credentials to file
func (r *FileCredentialRepository) Save(credentials *entities.Credentials) error {
	// Ensure directory exists
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	return os.WriteFile(r.filePath, data, 0644)
}

// AuthUseCase defines the authentication use case
type AuthUseCase struct {
	config         *entities.Config
	oauthGateway   gateways.OAuthGateway
	credentialRepo CredentialRepository
	logger         *logging.Logger
	tokenMutex     sync.RWMutex
	refreshMutex   sync.Mutex
}

// NewAuthUseCase creates a new authentication use case
func NewAuthUseCase(config *entities.Config, oauthGateway gateways.OAuthGateway, credentialRepo CredentialRepository, logger *logging.Logger) *AuthUseCase {
	return &AuthUseCase{
		config:         config,
		oauthGateway:   oauthGateway,
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

		// Double-check after acquiring lock
		uc.tokenMutex.RLock()
		now = time.Now().UnixMilli()
		timeUntilExpiry = credentials.ExpiryDate - now
		isStillExpired := timeUntilExpiry < bufferInMillis
		uc.tokenMutex.RUnlock()

		if isStillExpired {
			uc.logger.Info("Qwen token expired or close to expiring, refreshing")
			if err := uc.refreshAccessToken(credentials); err != nil {
				uc.logger.Warn("Failed to refresh token, falling back to device authentication", "error", err)
				return uc.authenticateWithDeviceFlow()
			}
		}
	}

	return credentials, nil
}

// refreshAccessToken refreshes the access token
func (uc *AuthUseCase) refreshAccessToken(credentials *entities.Credentials) error {
	if credentials.RefreshToken == "" {
		return fmt.Errorf("no refresh token available in credentials")
	}

	newCredentials, err := uc.oauthGateway.RefreshToken(credentials.RefreshToken, uc.config.QWENOAuthClientID)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Keep existing resource URL
	newCredentials.ResourceURL = credentials.ResourceURL

	uc.tokenMutex.Lock()
	defer uc.tokenMutex.Unlock()

	return uc.credentialRepo.Save(newCredentials)
}

// authenticateWithDeviceFlow performs OAuth2 device authorization flow
func (uc *AuthUseCase) authenticateWithDeviceFlow() (*entities.Credentials, error) {
	credentials, err := uc.oauthGateway.AuthenticateWithDeviceFlow(uc.config.QWENOAuthClientID, uc.config.QWENOAuthScope)
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

// GetBaseURL returns the base URL for API calls
func (uc *AuthUseCase) GetBaseURL(credentials *entities.Credentials) (string, error) {
	baseURL := credentials.ResourceURL
	if baseURL == "" {
		return "", fmt.Errorf("no resource URL available in credentials")
	}

	return baseURL, nil
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
