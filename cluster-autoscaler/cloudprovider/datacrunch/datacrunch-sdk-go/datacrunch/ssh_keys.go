package datacrunch

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type SSHKeyService struct {
	client *Client
}

type CreateSSHKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"key"`
}

// Get retrieves all SSH keys
func (s *SSHKeyService) Get(ctx context.Context) ([]SSHKey, error) {
	keys, _, err := getRequest[[]SSHKey](ctx, s.client, "/sshkeys")
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// GetByID fetches a specific SSH key by its ID
func (s *SSHKeyService) GetByID(ctx context.Context, id string) (*SSHKey, error) {
	path := fmt.Sprintf("/sshkeys/%s", id)

	// The API returns an array with one key
	keys, _, err := getRequest[[]SSHKey](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("SSH key with ID %s not found", id)
	}

	return &keys[0], nil
}

// Create creates a new SSH key
func (s *SSHKeyService) Create(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	// The API returns just the ID as plain text, not JSON
	// So we need to handle this manually
	return s.createWithPlainTextResponse(ctx, req)
}

// createWithPlainTextResponse handles the case where the API returns plain text instead of JSON
func (s *SSHKeyService) createWithPlainTextResponse(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	resp, err := s.client.makeRequest("POST", "/sshkeys", req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error in defer
	}()

	// Check for error status codes first
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Re-read the body for error handling
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// The API returns just the ID as plain text, not JSON
	// Read the ID from the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	keyID := strings.TrimSpace(string(body))

	// Now fetch the full key details
	return s.GetByID(ctx, keyID)
}

// Delete removes an SSH key
func (s *SSHKeyService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/sshkeys/%s", id)
	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
