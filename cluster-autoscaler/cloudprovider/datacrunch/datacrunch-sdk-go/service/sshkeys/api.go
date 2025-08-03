package sshkeys

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
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

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return nil, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return nil, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Parse response
	var sshKeys []*SSHKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&sshKeys); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
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

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return nil, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return nil, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Parse response
	var sshKey SSHKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&sshKey); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
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

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Set the request body
	body, err := json.Marshal(input)
	if err != nil {
		return nil, dcerr.New("SerializationError", "failed to marshal request body", err)
	}
	req.SetBufferBody(body)

	// Set content type header
	req.HTTPRequest.Header.Set("Content-Type", "application/json")

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Log the request URL and payload
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())
	log.Printf("Request payload: %s", string(body))
	log.Printf("Request headers: %v", req.HTTPRequest.Header)

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return nil, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return nil, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Parse response
	var sshKey SSHKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&sshKey); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
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

	// Set the request body
	body, err := json.Marshal(input)
	if err != nil {
		return dcerr.New("SerializationError", "failed to marshal request body", err)
	}
	req.SetBufferBody(body)

	// Set content type header
	req.HTTPRequest.Header.Set("Content-Type", "application/json")

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Log the request URL and payload
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())
	log.Printf("Request payload: %s", string(body))
	log.Printf("Request headers: %v", req.HTTPRequest.Header)

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
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

	// Set content type header
	req.HTTPRequest.Header.Set("Content-Type", "text/plain;charset=UTF-8")

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	return nil
}
