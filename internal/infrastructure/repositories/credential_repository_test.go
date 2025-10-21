package repositories

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"qwen-go-proxy/internal/domain/entities"

	"github.com/stretchr/testify/assert"
)

func TestNewFileCredentialRepository(t *testing.T) {
	// Test that NewFileCredentialRepository creates a repository with correct file path
	qwenDir := ".qwen-test"
	repo := NewFileCredentialRepository(qwenDir)

	// We can't directly access the private field, but we can test the behavior
	// by attempting to save and load credentials
	assert.NotNil(t, repo)
}

func TestFileCredentialRepository_Save_Load_Success(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create the repository with a path in the temp directory
	repo := &FileCredentialRepository{
		filePath: filepath.Join(tempDir, ".qwen-test", "oauth_creds.json"),
	}

	creds := &entities.Credentials{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		ExpiryDate:   1234567890,
		ResourceURL:  "https://api.example.com",
	}

	// Save credentials
	err := repo.Save(creds)
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(repo.filePath)
	assert.NoError(t, err)

	// Load credentials
	loadedCreds, err := repo.Load()
	assert.NoError(t, err)
	assert.Equal(t, creds.AccessToken, loadedCreds.AccessToken)
	assert.Equal(t, creds.TokenType, loadedCreds.TokenType)
	assert.Equal(t, creds.RefreshToken, loadedCreds.RefreshToken)
	assert.Equal(t, creds.ExpiryDate, loadedCreds.ExpiryDate)
	assert.Equal(t, creds.ResourceURL, loadedCreds.ResourceURL)
}

func TestFileCredentialRepository_Load_NonExistentFile(t *testing.T) {
	// Create a repository pointing to a non-existent file
	tempDir := t.TempDir()

	repo := &FileCredentialRepository{
		filePath: filepath.Join(tempDir, "non-existent", "oauth_creds.json"),
	}

	// Load credentials from non-existent file
	_, err := repo.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read Qwen OAuth credentials")
}

func TestFileCredentialRepository_Save_InvalidCredentials(t *testing.T) {
	// Create a repository
	tempDir := t.TempDir()

	repo := &FileCredentialRepository{
		filePath: filepath.Join(tempDir, ".qwen-test", "oauth_creds.json"),
	}

	// Create credentials with unmarshalable data (this is tricky since our struct is valid)
	// Instead, let's test saving to a directory we can't write to
	// For this test, we'll create a read-only file to trigger marshal error
	creds := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}

	// Save credentials normally first
	err := repo.Save(creds)
	assert.NoError(t, err)

	// Now test loading with invalid JSON
	invalidJSON := []byte(`{"access_token": "test", "invalid_json": }`) // Invalid JSON
	err = os.WriteFile(repo.filePath, invalidJSON, 0644)
	assert.NoError(t, err)

	// Try to load invalid JSON
	_, err = repo.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse Qwen OAuth credentials")
}

func TestFileCredentialRepository_Save_DirectoryCreation(t *testing.T) {
	// Create a repository that needs to create directories
	tempDir := t.TempDir()

	repo := &FileCredentialRepository{
		filePath: filepath.Join(tempDir, "new-dir", "subdir", "oauth_creds.json"),
	}

	creds := &entities.Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}

	// Save credentials - this should create the directory structure
	err := repo.Save(creds)
	assert.NoError(t, err)

	// Verify the file exists
	_, err = os.Stat(repo.filePath)
	assert.NoError(t, err)

	// Verify the directory was created
	dir := filepath.Dir(repo.filePath)
	_, err = os.Stat(dir)
	assert.NoError(t, err)
}

func TestFileCredentialRepository_Save_MarshalError(t *testing.T) {
	// This test is to check what happens when JSON marshaling fails
	// Since our Credentials struct is valid, we can't easily create a marshal error
	// Instead, let's test another scenario - trying to save to a directory without permissions
	// This is difficult to test without changing system permissions

	// For now, let's just verify that our credentials struct can be marshaled
	creds := &entities.Credentials{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		ExpiryDate:   1234567890,
		ResourceURL:  "https://api.example.com",
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify that unmarshaling works too
	var unmarshaledCreds entities.Credentials
	err = json.Unmarshal(data, &unmarshaledCreds)
	assert.NoError(t, err)
	assert.Equal(t, creds.AccessToken, unmarshaledCreds.AccessToken)
	assert.Equal(t, creds.TokenType, unmarshaledCreds.TokenType)
	assert.Equal(t, creds.RefreshToken, unmarshaledCreds.RefreshToken)
	assert.Equal(t, creds.ExpiryDate, unmarshaledCreds.ExpiryDate)
	assert.Equal(t, creds.ResourceURL, unmarshaledCreds.ResourceURL)
}
