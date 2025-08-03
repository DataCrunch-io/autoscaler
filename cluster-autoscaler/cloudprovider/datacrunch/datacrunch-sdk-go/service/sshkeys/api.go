package sshkeys

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// SSHKeyResponse represents an SSH key
type SSHKeyResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// CreateSSHKeyInput represents the input for creating a new SSH key
type CreateSSHKeyInput struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// DeleteSSHKeysInput represents the input for deleting multiple SSH keys
type DeleteSSHKeysInput struct {
	Keys []string `json:"keys"`
}

// ListSSHKeys lists all SSH keys
func (c *SSHKey) ListSSHKeys(ctx context.Context) ([]*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "ListSSHKeys",
		HTTPMethod: "GET",
		HTTPPath:   "/sshkeys",
	}

	var sshKeys []*SSHKeyResponse
	req := c.NewRequest(op, nil, &sshKeys)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return sshKeys, nil
}

// GetSSHKey gets a single SSH key by ID
func (c *SSHKey) GetSSHKey(ctx context.Context, id string) (*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "GetSSHKey",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/sshkeys/%s", id),
	}

	var sshKey SSHKeyResponse
	req := c.NewRequest(op, nil, &sshKey)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// CreateSSHKey creates a new SSH key
func (c *SSHKey) CreateSSHKey(ctx context.Context, input *CreateSSHKeyInput) (*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "CreateSSHKey",
		HTTPMethod: "POST",
		HTTPPath:   "/sshkeys",
	}

	var sshKey SSHKeyResponse
	req := c.NewRequest(op, input, &sshKey)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// DeleteSSHKeys deletes multiple SSH keys
func (c *SSHKey) DeleteSSHKeys(ctx context.Context, input *DeleteSSHKeysInput) error {
	op := &request.Operation{
		Name:       "DeleteSSHKeys",
		HTTPMethod: "DELETE",
		HTTPPath:   "/sshkeys",
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return err
	}

	return nil
}

// DeleteSSHKey deletes a single SSH key by ID
func (c *SSHKey) DeleteSSHKey(ctx context.Context, id string) error {
	op := &request.Operation{
		Name:       "DeleteSSHKey",
		HTTPMethod: "DELETE",
		HTTPPath:   fmt.Sprintf("/sshkeys/%s", id),
	}

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return err
	}

	return nil
}
