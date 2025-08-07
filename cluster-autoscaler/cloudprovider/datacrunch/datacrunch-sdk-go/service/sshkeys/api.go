package sshkeys

import (
	"fmt"

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
func (c *SSHKey) ListSSHKeys() ([]*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "ListSSHKeys",
		HTTPMethod: "GET",
		HTTPPath:   "/sshkeys",
	}

	var sshKeys []*SSHKeyResponse
	req := c.newRequest(op, nil, &sshKeys)

	return sshKeys, req.Send()
}

// GetSSHKey gets a single SSH key by ID
func (c *SSHKey) GetSSHKey(id string) (*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "GetSSHKey",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/sshkeys/%s", id),
	}

	var sshKey SSHKeyResponse
	req := c.newRequest(op, nil, &sshKey)

	return &sshKey, req.Send()
}

// CreateSSHKey creates a new SSH key
func (c *SSHKey) CreateSSHKey(input *CreateSSHKeyInput) (*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "CreateSSHKey",
		HTTPMethod: "POST",
		HTTPPath:   "/sshkeys",
	}

	var sshKey SSHKeyResponse
	req := c.newRequest(op, input, &sshKey)

	return &sshKey, req.Send()
}

// DeleteSSHKeys deletes multiple SSH keys
func (c *SSHKey) DeleteSSHKeys(input *DeleteSSHKeysInput) error {
	op := &request.Operation{
		Name:       "DeleteSSHKeys",
		HTTPMethod: "DELETE",
		HTTPPath:   "/sshkeys",
	}

	req := c.newRequest(op, input, nil)

	if err := req.Send(); err != nil {
		return err
	}

	return req.Send()
}

// DeleteSSHKey deletes a single SSH key by ID
func (c *SSHKey) DeleteSSHKey(id string) error {
	op := &request.Operation{
		Name:       "DeleteSSHKey",
		HTTPMethod: "DELETE",
		HTTPPath:   fmt.Sprintf("/sshkeys/%s", id),
	}

	req := c.newRequest(op, nil, nil)

	return req.Send()
}
