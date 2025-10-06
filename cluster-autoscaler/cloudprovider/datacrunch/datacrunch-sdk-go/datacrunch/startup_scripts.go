package datacrunch

import (
	"context"
	"fmt"
)

type StartupScriptService struct {
	client *Client
}

type CreateStartupScriptRequest struct {
	Name   string `json:"name"`
	Script string `json:"script"`
}

// Get retrieves all startup scripts
func (s *StartupScriptService) Get(ctx context.Context) ([]StartupScript, error) {
	scripts, _, err := getRequest[[]StartupScript](ctx, s.client, "/startup-scripts")
	if err != nil {
		return nil, err
	}
	return scripts, nil
}

// GetByID fetches a specific startup script by its ID
func (s *StartupScriptService) GetByID(ctx context.Context, id string) (*StartupScript, error) {
	path := fmt.Sprintf("/startup-scripts/%s", id)
	script, _, err := getRequest[StartupScript](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// Create creates a new startup script
func (s *StartupScriptService) Create(ctx context.Context, req CreateStartupScriptRequest) (*StartupScript, error) {
	script, _, err := postRequest[StartupScript](ctx, s.client, "/startup-scripts", req)
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// Delete removes a startup script
func (s *StartupScriptService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/startup-scripts/%s", id)
	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
