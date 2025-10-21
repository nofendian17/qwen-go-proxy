package auth

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/infrastructure/repositories"
	"qwen-go-proxy/internal/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Test AuthUseCase

func TestNewAuthUseCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:      "test-client-id",
		QWENOAuthScope:         "test-scope",
		QWENOAuthBaseURL:       "https://oauth.example.com",
		QWENOAuthDeviceAuthURL: "https://oauth.example.com/device",
		TokenRefreshBuffer:     5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &mockCredentialRepository{}

	NewAuthUseCase(config, oauthService, repo, logger)
}

func TestAuthUseCase_EnsureAuthenticated_NoCredentials_DeviceFlowSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	testCreds := &entities.Credentials{
		AccessToken:  "device-flow-token",
		TokenType:    "Bearer",
		RefreshToken: "device-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Mock repo that returns error on load
	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.EnsureAuthenticated()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != testCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", testCreds.AccessToken, result.AccessToken)
	}
}

func TestAuthUseCase_EnsureAuthenticated_NoCredentials_DeviceFlowFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(nil, errors.New("device flow failed")).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.EnsureAuthenticated()

	if result != nil {
		t.Error("Expected nil result on failure")
	}
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "device authentication failed") {
		t.Errorf("Expected device auth error, got %v", err)
	}
}

func TestAuthUseCase_EnsureAuthenticated_ValidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		TokenRefreshBuffer: 5 * time.Minute,
	}

	validCreds := &entities.Credentials{
		AccessToken:  "valid-token",
		TokenType:    "Bearer",
		RefreshToken: "valid-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(), // Valid for another hour
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: validCreds,
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.EnsureAuthenticated()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != validCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", validCreds.AccessToken, result.AccessToken)
	}
	// Should not call device flow or refresh for valid credentials
}

func TestAuthUseCase_EnsureAuthenticated_ExpiredCredentials_RefreshSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	expiredCreds := &entities.Credentials{
		AccessToken:  "expired-token",
		TokenType:    "Bearer",
		RefreshToken: "valid-refresh",
		ExpiryDate:   time.Now().Add(-time.Hour).UnixMilli(), // Expired 1 hour ago
		ResourceURL:  "https://api.example.com",
	}

	refreshedCreds := &entities.Credentials{
		AccessToken:  "refreshed-token",
		TokenType:    "Bearer",
		RefreshToken: "new-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("valid-refresh", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.EnsureAuthenticated()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != refreshedCreds.AccessToken {
		t.Errorf("Expected refreshed access token %s, got %s", refreshedCreds.AccessToken, result.AccessToken)
	}
	if repo.saveCallCount != 1 {
		t.Errorf("Expected save to be called once, got %d", repo.saveCallCount)
	}
}

func TestAuthUseCase_EnsureAuthenticated_ExpiredCredentials_RefreshFailure_FallbackToDeviceFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	expiredCreds := &entities.Credentials{
		AccessToken:  "expired-token",
		TokenType:    "Bearer",
		RefreshToken: "invalid-refresh",
		ExpiryDate:   time.Now().Add(-time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	deviceFlowCreds := &entities.Credentials{
		AccessToken:  "device-token",
		TokenType:    "Bearer",
		RefreshToken: "device-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("invalid-refresh", "test-client-id").
		Return(nil, errors.New("refresh failed")).
		Times(1)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(deviceFlowCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.EnsureAuthenticated()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != deviceFlowCreds.AccessToken {
		t.Errorf("Expected device flow access token %s, got %s", deviceFlowCreds.AccessToken, result.AccessToken)
	}
}

func TestAuthUseCase_refreshAccessToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
	}

	credentials := &entities.Credentials{
		AccessToken:  "old-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		ExpiryDate:   time.Now().UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	refreshedCreds := &entities.Credentials{
		AccessToken:  "new-token",
		TokenType:    "Bearer",
		RefreshToken: "new-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("refresh-token", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	refreshed, err := useCase.refreshAccessToken(credentials)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if refreshed == nil {
		t.Fatal("Expected refreshed credentials")
	}
	if repo.saveCallCount != 1 {
		t.Error("Expected save to be called")
	}
	if repo.lastSavedCredentials.AccessToken != refreshedCreds.AccessToken {
		t.Errorf("Expected saved token %s, got %s", refreshedCreds.AccessToken, repo.lastSavedCredentials.AccessToken)
	}
	if repo.lastSavedCredentials.ResourceURL != credentials.ResourceURL {
		t.Error("Expected original resource URL to be preserved")
	}
	if refreshed.AccessToken != refreshedCreds.AccessToken {
		t.Errorf("Expected returned token %s, got %s", refreshedCreds.AccessToken, refreshed.AccessToken)
	}
}

func TestAuthUseCase_refreshAccessToken_NoRefreshToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
	}

	credentials := &entities.Credentials{
		AccessToken: "old-token",
		TokenType:   "Bearer",
		// No refresh token
		ExpiryDate: time.Now().UnixMilli(),
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	_, err := useCase.refreshAccessToken(credentials)

	if err == nil {
		t.Error("Expected error for missing refresh token")
	}
	if !strings.Contains(err.Error(), "no refresh token available") {
		t.Errorf("Expected specific error message, got %v", err)
	}
}

func TestAuthUseCase_refreshAccessToken_RefreshFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
	}

	credentials := &entities.Credentials{
		AccessToken:  "old-token",
		TokenType:    "Bearer",
		RefreshToken: "invalid-refresh-token",
		ExpiryDate:   time.Now().UnixMilli(),
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("invalid-refresh-token", "test-client-id").
		Return(nil, errors.New("refresh failed")).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	_, err := useCase.refreshAccessToken(credentials)

	if err == nil {
		t.Error("Expected error for refresh failure")
	}
	if !strings.Contains(err.Error(), "failed to refresh token") {
		t.Errorf("Expected refresh error, got %v", err)
	}
}

func TestAuthUseCase_authenticateWithDeviceFlow_Success(t *testing.T) {
	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
		QWENOAuthScope:    "test-scope",
	}

	testCreds := &entities.Credentials{
		AccessToken:  "device-token",
		TokenType:    "Bearer",
		RefreshToken: "device-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.authenticateWithDeviceFlow()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != testCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", testCreds.AccessToken, result.AccessToken)
	}
	if repo.saveCallCount != 1 {
		t.Error("Expected save to be called")
	}
}

func TestAuthUseCase_authenticateWithDeviceFlow_Failure(t *testing.T) {
	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
		QWENOAuthScope:    "test-scope",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(nil, errors.New("device flow failed")).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.authenticateWithDeviceFlow()

	if result != nil {
		t.Error("Expected nil result on failure")
	}
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "device authentication failed") {
		t.Errorf("Expected device auth error, got %v", err)
	}
	if repo.saveCallCount != 0 {
		t.Error("Expected save not to be called on failure")
	}
}

func TestAuthUseCase_AuthenticateManually_Success(t *testing.T) {
	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
		QWENOAuthScope:    "test-scope",
	}

	testCreds := &entities.Credentials{
		AccessToken: "manual-token",
		TokenType:   "Bearer",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	err := useCase.AuthenticateManually()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthUseCase_CheckAuthentication(t *testing.T) {
	config := &entities.Config{}
	testCreds := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiryDate:  time.Now().Add(time.Hour).UnixMilli(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: testCreds,
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.CheckAuthentication()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected credentials")
	}
	if result.AccessToken != testCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", testCreds.AccessToken, result.AccessToken)
	}
}

// Mock credential repository for testing
type mockCredentialRepository struct {
	loadCredentials      *entities.Credentials
	loadError            error
	saveCallCount        int
	lastSavedCredentials *entities.Credentials
	saveError            error
	savedCredentials     *entities.Credentials // Tracks the most recently saved credentials
}

func (m *mockCredentialRepository) Load() (*entities.Credentials, error) {
	if m.savedCredentials != nil {
		return m.savedCredentials, m.loadError
	}
	return m.loadCredentials, m.loadError
}

func (m *mockCredentialRepository) Save(credentials *entities.Credentials) error {
	m.saveCallCount++
	m.lastSavedCredentials = credentials
	m.savedCredentials = credentials // Update what Load() returns
	return m.saveError
}

// Test concurrency with mutexes
func TestAuthUseCase_EnsureAuthenticated_ConcurrentAccess(t *testing.T) {
	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	expiredCreds := &entities.Credentials{
		AccessToken:  "expired-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		ExpiryDate:   time.Now().Add(-time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	refreshedCreds := &entities.Credentials{
		AccessToken:  "refreshed-token",
		TokenType:    "Bearer",
		RefreshToken: "new-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("refresh-token", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	// Run multiple concurrent calls on the same useCase instance
	var wg sync.WaitGroup
	results := make([]*entities.Credentials, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errors[index] = useCase.EnsureAuthenticated()
		}(i)
	}

	wg.Wait()

	// Verify all calls succeeded
	for i := 0; i < 10; i++ {
		if errors[i] != nil {
			t.Errorf("Concurrent call %d failed: %v", i, errors[i])
		}
		if results[i] == nil {
			t.Errorf("Concurrent call %d returned nil credentials", i)
		} else if results[i].AccessToken != refreshedCreds.AccessToken {
			t.Errorf("Concurrent call %d got wrong token: %s", i, results[i].AccessToken)
		}
	}

	// Should only refresh once due to mutex
}

// Negative Test Cases

func TestNewAuthUseCase_NilConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &mockCredentialRepository{}

	// Test with nil config - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewAuthUseCase(nil, oauthService, repo, logger)
	})
}

func TestNewAuthUseCase_NilOAuthService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &mockCredentialRepository{}

	// Test with nil oauth service - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewAuthUseCase(config, nil, repo, logger)
	})
}

func TestNewAuthUseCase_NilRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Test with nil repository - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewAuthUseCase(config, oauthService, nil, logger)
	})
}

func TestNewAuthUseCase_NilLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	repo := repositories.NewFileCredentialRepository("/tmp/test")

	// Test with nil logger - should panic (documenting current behavior)
	assert.Panics(t, func() {
		NewAuthUseCase(config, oauthService, repo, nil)
	})
}

func TestAuthUseCase_EnsureAuthenticated_ConfigWithEmptyClientID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "", // Empty client ID
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	oauthService.EXPECT().AuthenticateWithDeviceFlow("", "test-scope").Return(nil, fmt.Errorf("empty client ID"))

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	_, err := useCase.EnsureAuthenticated()

	// Should handle empty client ID gracefully
	assert.Error(t, err)
}

func TestAuthUseCase_EnsureAuthenticated_ConfigWithEmptyScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "", // Empty scope
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	oauthService.EXPECT().AuthenticateWithDeviceFlow("test-client-id", "").Return(nil, fmt.Errorf("empty scope"))

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	_, err := useCase.EnsureAuthenticated()

	// Should handle empty scope gracefully
	assert.Error(t, err)
}

func TestAuthUseCase_EnsureAuthenticated_RepositoryLoadPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Repository that panics on Load
	repo := &panickingRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	// Should handle repository panic gracefully
	assert.Panics(t, func() {
		useCase.EnsureAuthenticated()
	})
}

func TestAuthUseCase_EnsureAuthenticated_RepositorySavePanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID:  "test-client-id",
		QWENOAuthScope:     "test-scope",
		TokenRefreshBuffer: 5 * time.Minute,
	}

	testCreds := &entities.Credentials{
		AccessToken:  "device-flow-token",
		TokenType:    "Bearer",
		RefreshToken: "device-refresh",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Repository that panics on Save
	repo := &panickingSaveRepository{}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	// Should handle repository save panic gracefully
	assert.Panics(t, func() {
		useCase.EnsureAuthenticated()
	})
}

func TestAuthUseCase_refreshAccessToken_RepositorySaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{
		QWENOAuthClientID: "test-client-id",
	}

	credentials := &entities.Credentials{
		AccessToken:  "old-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		ExpiryDate:   time.Now().UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	refreshedCreds := &entities.Credentials{
		AccessToken:  "new-token",
		TokenType:    "Bearer",
		RefreshToken: "new-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	oauthService := mocks.NewMockOAuthService(ctrl)
	oauthService.EXPECT().
		RefreshToken("refresh-token", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Repository that fails on Save
	repo := &mockCredentialRepository{
		saveError: fmt.Errorf("save failed"),
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	refreshed, err := useCase.refreshAccessToken(credentials)

	// Should handle save error gracefully
	assert.Error(t, err)
	assert.Nil(t, refreshed)
	assert.Contains(t, err.Error(), "failed to save refreshed credentials")
}

func TestAuthUseCase_CheckAuthentication_RepositoryLoadError(t *testing.T) {
	config := &entities.Config{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oauthService := mocks.NewMockOAuthService(ctrl)
	// Since CheckAuthentication calls EnsureAuthenticated which tries to authenticate on load failure,
	// we need to set up the mock to return an error
	oauthService.EXPECT().AuthenticateWithDeviceFlow(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("auth failed")).AnyTimes()
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Repository that fails on Load
	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("load failed"),
	}

	useCase := NewAuthUseCase(config, oauthService, repo, logger)

	result, err := useCase.CheckAuthentication()

	// Should handle load error gracefully (by attempting authentication, which fails)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "device authentication failed")
}

// Mock repository that panics for testing
type panickingRepository struct{}

func (p *panickingRepository) Load() (*entities.Credentials, error) {
	panic("load panic")
}

func (p *panickingRepository) Save(credentials *entities.Credentials) error {
	panic("save panic")
}

// Mock repository that panics only on Save
type panickingSaveRepository struct{}

func (p *panickingSaveRepository) Load() (*entities.Credentials, error) {
	return nil, fmt.Errorf("no credentials") // Return error to force authentication
}

func (p *panickingSaveRepository) Save(credentials *entities.Credentials) error {
	panic("save panic")
}
