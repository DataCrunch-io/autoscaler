package datacrunch

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type VolumeService struct {
	client *Client
}

// Get retrieves all volumes
func (s *VolumeService) Get(ctx context.Context) ([]Volume, error) {
	return s.GetByStatus(ctx, "")
}

// GetByStatus retrieves volumes filtered by status, or all volumes if status is empty
func (s *VolumeService) GetByStatus(ctx context.Context, status string) ([]Volume, error) {
	path := "/volumes"
	if status != "" {
		params := url.Values{}
		params.Set("status", status)
		path += "?" + params.Encode()
	}

	volumes, _, err := getRequest[[]Volume](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

// GetByID fetches a specific volume by its ID
func (s *VolumeService) GetByID(ctx context.Context, id string) (*Volume, error) {
	path := fmt.Sprintf("/volumes/%s", id)
	volume, _, err := getRequest[Volume](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

// Create creates a new volume
func (s *VolumeService) Create(ctx context.Context, req VolumeCreateRequest) (string, error) {
	// Set default location if not provided
	if req.Location == "" {
		req.Location = LocationFIN01
	}

	// The API returns just the volume ID as plain text, not JSON
	// So we need to handle this manually like instances.Create
	return s.createWithPlainTextResponse(ctx, req)
}

// createWithPlainTextResponse handles the case where the API returns plain text instead of JSON
func (s *VolumeService) createWithPlainTextResponse(ctx context.Context, req VolumeCreateRequest) (string, error) {
	// Use the old method as a fallback for plain text responses
	resp, err := s.client.makeRequest("POST", "/volumes", req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error in defer
	}()

	// Check for error status codes first
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", s.client.handleResponse(resp, nil)
	}

	// The API returns just the volume ID as plain text, not JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	volumeID := strings.TrimSpace(string(body))
	return volumeID, nil
}

// Delete removes a volume
func (s *VolumeService) Delete(ctx context.Context, id string, force bool) error {
	path := fmt.Sprintf("/volumes/%s", id)
	if force {
		path += "?force=true"
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
