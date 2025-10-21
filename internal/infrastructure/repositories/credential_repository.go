// Package repositories contains infrastructure implementations of domain interfaces.
// This package provides concrete implementations of the repository interfaces
// defined in the domain layer.
package repositories

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"qwen-go-proxy/internal/domain/entities"
	"qwen-go-proxy/internal/domain/interfaces"
)

// FileCredentialRepository implements CredentialRepository using file storage.
// This infrastructure component provides persistence for credentials using the local filesystem.
type FileCredentialRepository struct {
	filePath string
}

// NewFileCredentialRepository creates a new file-based credential repository.
// It constructs the file path from the provided directory and ensures it can be used
// for credential storage operations.
func NewFileCredentialRepository(qwenDir string) interfaces.CredentialRepository {
	// Use current working directory as base path
	workDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get current working directory: %v", err))
	}
	return &FileCredentialRepository{
		filePath: filepath.Join(workDir, qwenDir, "oauth_creds.json"),
	}
}

// Load loads credentials from file.
// It reads the JSON-serialized credentials from the filesystem and returns them.
func (r *FileCredentialRepository) Load() (*entities.Credentials, error) {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read Qwen OAuth credentials: %w", err)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("failed to read Qwen OAuth credentials: permission denied. The credentials file exists but is not readable by the application user. Please ensure the file permissions allow read access")
		}
		return nil, fmt.Errorf("failed to read Qwen OAuth credentials: %w", err)
	}

	var creds entities.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen OAuth credentials: %w", err)
	}

	return &creds, nil
}

// Save saves credentials to file.
// It serializes credentials to JSON and writes them to the filesystem,
// creating any necessary directories.
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