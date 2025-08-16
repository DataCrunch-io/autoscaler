package sshkeys

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/restjson"
)

// CreateSSHKeyInput represents the input for creating a new SSH key
type CreateSSHKeyInput struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// DeleteSSHKeysInput represents the input for deleting multiple SSH keys
type DeleteSSHKeysInput struct {
	Keys []string `json:"keys"`
}

type SSHKeyResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
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

type GetSSHKeyInput struct {
	ID string `location:"uri" locationName:"id"`
}

// GetSSHKey gets a single SSH key by ID
func (c *SSHKey) GetSSHKey(id string) ([]*SSHKeyResponse, error) {
	op := &request.Operation{
		Name:       "GetSSHKey",
		HTTPMethod: "GET",
		HTTPPath:   "/sshkeys/{id}",
	}

	var sshKey []*SSHKeyResponse
	req := c.newRequest(op, &GetSSHKeyInput{ID: id}, &sshKey)

	return sshKey, req.Send()
}

// CreateSSHKey creates a new SSH key
func (c *SSHKey) CreateSSHKey(input *CreateSSHKeyInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateSSHKey",
		HTTPMethod: "POST",
		HTTPPath:   "/sshkeys",
	}

	var sshKey string
	req := c.newRequest(op, input, &sshKey)

	req.Handlers.Unmarshal.RemoveByName("datacrunchsdk.restjson.Unmarshal")
	req.Handlers.Unmarshal.PushBackNamed(restjson.StringUnmarshalHandler)

	return sshKey, req.Send()
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

type DeleteSSHKeyInput struct {
	ID string `location:"uri" locationName:"id"`
}

// DeleteSSHKey deletes a single SSH key by ID
func (c *SSHKey) DeleteSSHKey(id string) error {
	op := &request.Operation{
		Name:       "DeleteSSHKey",
		HTTPMethod: "DELETE",
		HTTPPath:   "/sshkeys/{id}",
	}

	input := &DeleteSSHKeyInput{
		ID: id,
	}

	req := c.newRequest(op, input, nil)

	return req.Send()
}
