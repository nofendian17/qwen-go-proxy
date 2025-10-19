package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/mocks"

	"go.uber.org/mock/gomock"
)

// Mock implementations for testing

func setupTestDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "qwen-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// Test FileCredentialRepository

func TestNewFileCredentialRepository(t *testing.T) {
	qwenDir := "data"
	repo := NewFileCredentialRepository(qwenDir)

	fileRepo, ok := repo.(*FileCredentialRepository)
	if !ok {
		t.Fatal("Expected FileCredentialRepository")
	}

	// Note: We can't easily test the exact path since it uses os.Getwd()
	// but we can verify it's initialized
	if fileRepo.filePath == "" {
		t.Error("Expected filePath to be set")
	}
	if !strings.Contains(fileRepo.filePath, "oauth_creds.json") {
		t.Errorf("Expected filePath to contain oauth_creds.json, got %s", fileRepo.filePath)
	}
}

func TestFileCredentialRepository_Load_Success(t *testing.T) {
	testDir := setupTestDir(t)
	qwenDir := "qwen-data"
	filePath := filepath.Join(testDir, qwenDir, "oauth_creds.json")

	// Create directory and file
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testCreds := &entities.Credentials{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	data, err := json.MarshalIndent(testCreds, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test credentials: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test credentials: %v", err)
	}

	// Create repository with test directory
	oldWd, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(oldWd)

	repo := NewFileCredentialRepository(qwenDir)
	loadedCreds, err := repo.Load()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if loadedCreds == nil {
		t.Fatal("Expected credentials to be loaded")
	}
	if loadedCreds.AccessToken != testCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", testCreds.AccessToken, loadedCreds.AccessToken)
	}
	if loadedCreds.TokenType != testCreds.TokenType {
		t.Errorf("Expected token type %s, got %s", testCreds.TokenType, loadedCreds.TokenType)
	}
	if loadedCreds.RefreshToken != testCreds.RefreshToken {
		t.Errorf("Expected refresh token %s, got %s", testCreds.RefreshToken, loadedCreds.RefreshToken)
	}
	if loadedCreds.ResourceURL != testCreds.ResourceURL {
		t.Errorf("Expected resource URL %s, got %s", testCreds.ResourceURL, loadedCreds.ResourceURL)
	}
}

func TestFileCredentialRepository_Load_FileNotFound(t *testing.T) {
	testDir := setupTestDir(t)
	qwenDir := "qwen-data"

	oldWd, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(oldWd)

	repo := NewFileCredentialRepository(qwenDir)
	creds, err := repo.Load()

	if creds != nil {
		t.Error("Expected nil credentials for non-existent file")
	}
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read Qwen OAuth credentials") {
		t.Errorf("Expected specific error message, got %v", err)
	}
}

func TestFileCredentialRepository_Load_InvalidJSON(t *testing.T) {
	testDir := setupTestDir(t)
	qwenDir := "qwen-data"
	filePath := filepath.Join(testDir, qwenDir, "oauth_creds.json")

	// Create directory and invalid JSON file
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	err = os.WriteFile(filePath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(oldWd)

	repo := NewFileCredentialRepository(qwenDir)
	creds, err := repo.Load()

	if creds != nil {
		t.Error("Expected nil credentials for invalid JSON")
	}
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse Qwen OAuth credentials") {
		t.Errorf("Expected parse error message, got %v", err)
	}
}

func TestFileCredentialRepository_Save_Success(t *testing.T) {
	testDir := setupTestDir(t)
	qwenDir := "qwen-data"
	filePath := filepath.Join(testDir, qwenDir, "oauth_creds.json")

	oldWd, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(oldWd)

	repo := NewFileCredentialRepository(qwenDir)

	testCreds := &entities.Credentials{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		ExpiryDate:   time.Now().Add(time.Hour).UnixMilli(),
		ResourceURL:  "https://api.example.com",
	}

	err := repo.Save(testCreds)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify file was created and contains correct data
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Expected credentials file to be created")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	var savedCreds entities.Credentials
	err = json.Unmarshal(data, &savedCreds)
	if err != nil {
		t.Fatalf("Failed to unmarshal saved data: %v", err)
	}

	if savedCreds.AccessToken != testCreds.AccessToken {
		t.Errorf("Expected access token %s, got %s", testCreds.AccessToken, savedCreds.AccessToken)
	}
}

func TestFileCredentialRepository_Save_DirectoryCreationFails(t *testing.T) {
	// Use a read-only directory or invalid path that can't be created
	repo := &FileCredentialRepository{
		filePath: "/root/readonly/path/oauth_creds.json", // Assuming /root is not writable
	}

	testCreds := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}

	err := repo.Save(testCreds)
	if err == nil {
		t.Error("Expected error when directory creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("Expected directory creation error, got %v", err)
	}
}

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &FileCredentialRepository{filePath: "/tmp/test"}

	NewAuthUseCase(config, oauthGateway, repo, logger)
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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	// Mock repo that returns error on load
	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(nil, errors.New("device flow failed")).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadError: fmt.Errorf("file not found"),
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: validCreds,
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		RefreshToken("valid-refresh", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		RefreshToken("invalid-refresh", "test-client-id").
		Return(nil, errors.New("refresh failed")).
		Times(1)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(deviceFlowCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		RefreshToken("refresh-token", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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
		ExpiryDate:  time.Now().UnixMilli(),
	}

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		RefreshToken("invalid-refresh-token", "test-client-id").
		Return(nil, errors.New("refresh failed")).
		Times(1)

	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(nil, errors.New("device flow failed")).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

func TestAuthUseCase_GetBaseURL_WithResourceURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{}
	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

	credentials := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ResourceURL: "https://custom.api.com",
	}

	result, err := useCase.GetBaseURL(credentials)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "https://custom.api.com" {
		t.Errorf("Expected base URL 'https://custom.api.com', got %s", result)
	}
}

func TestAuthUseCase_GetBaseURL_NoResourceURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &entities.Config{}
	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})
	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

	credentials := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		// No ResourceURL
	}

	result, err := useCase.GetBaseURL(credentials)

	if err == nil {
		t.Error("Expected error for missing resource URL")
	}
	if !strings.Contains(err.Error(), "no resource URL available") {
		t.Errorf("Expected specific error message, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		AuthenticateWithDeviceFlow("test-client-id", "test-scope").
		Return(testCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: testCreds,
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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

	oauthGateway := mocks.NewMockOAuthGateway(ctrl)
	oauthGateway.EXPECT().
		RefreshToken("refresh-token", "test-client-id").
		Return(refreshedCreds, nil).
		Times(1)
	logger := logging.NewLoggerFromConfig(&entities.Config{LogLevel: "error"})

	repo := &mockCredentialRepository{
		loadCredentials: expiredCreds,
	}

	useCase := NewAuthUseCase(config, oauthGateway, repo, logger)

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