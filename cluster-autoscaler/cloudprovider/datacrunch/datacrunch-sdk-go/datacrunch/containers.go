package datacrunch

import (
	"context"
	"fmt"
)

type ContainerService struct {
	client *Client
}

type CreateContainerRequest struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Environment map[string]string `json:"environment,omitempty"`
}

// Get retrieves all containers
func (s *ContainerService) Get(ctx context.Context) ([]Container, error) {
	containers, _, err := getRequest[[]Container](ctx, s.client, "/containers")
	if err != nil {
		return nil, err
	}
	return containers, nil
}

// GetByID fetches a specific container by its ID
func (s *ContainerService) GetByID(ctx context.Context, id string) (*Container, error) {
	path := fmt.Sprintf("/containers/%s", id)
	container, _, err := getRequest[Container](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// Create creates a new container
func (s *ContainerService) Create(ctx context.Context, req CreateContainerRequest) (*Container, error) {
	container, _, err := postRequest[Container](ctx, s.client, "/containers", req)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// Delete removes a container
func (s *ContainerService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s", id)
	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
