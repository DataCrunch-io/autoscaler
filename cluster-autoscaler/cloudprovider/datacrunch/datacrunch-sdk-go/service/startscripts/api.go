package startscripts

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

// StartScriptResponse represents a startup script
type StartScriptResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Script string `json:"script"`
}

// CreateStartScriptInput represents the input for creating a new startup script
type CreateStartScriptInput struct {
	Name   string `json:"name"`
	Script string `json:"script"`
}

// DeleteStartScriptsInput represents the input for deleting multiple startup scripts
type DeleteStartScriptsInput struct {
	Scripts []string `json:"scripts"`
}

// ListStartScripts lists all startup scripts
func (c *StartScripts) ListStartScripts(ctx context.Context) ([]*StartScriptResponse, error) {
	op := &request.Operation{
		Name:       "ListStartScripts",
		HTTPMethod: "GET",
		HTTPPath:   "/scripts",
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

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
	var scripts []*StartScriptResponse
	if err := json.NewDecoder(resp.Body).Decode(&scripts); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return scripts, nil
}

// GetStartScript gets a single startup script by ID
func (c *StartScripts) GetStartScript(ctx context.Context, id string) (*StartScriptResponse, error) {
	op := &request.Operation{
		Name:       "GetStartScript",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/scripts/%s", id),
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
	var script StartScriptResponse
	if err := json.NewDecoder(resp.Body).Decode(&script); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return &script, nil
}

// CreateStartScript creates a new startup script
func (c *StartScripts) CreateStartScript(ctx context.Context, input *CreateStartScriptInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateStartScript",
		HTTPMethod: "POST",
		HTTPPath:   "/scripts",
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Set the request body
	body, err := json.Marshal(input)
	if err != nil {
		return "", dcerr.New("SerializationError", "failed to marshal request body", err)
	}
	req.SetBufferBody(body)

	// Set content type header
	req.HTTPRequest.Header.Set("Content-Type", "application/json")

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return "", req.Error
	}

	// Log the request URL and payload
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())
	log.Printf("Request payload: %s", string(body))
	log.Printf("Request headers: %v", req.HTTPRequest.Header)

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return "", dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return "", req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return "", dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Read the response body which should be the script ID
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", dcerr.New("SerializationError", "failed to read response body", err)
	}

	scriptID := string(body)
	log.Printf("Successfully created startup script: %s", scriptID)
	return scriptID, nil
}

// DeleteStartScripts deletes multiple startup scripts
func (c *StartScripts) DeleteStartScripts(ctx context.Context, input *DeleteStartScriptsInput) error {
	op := &request.Operation{
		Name:       "DeleteStartScripts",
		HTTPMethod: "DELETE",
		HTTPPath:   "/scripts",
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

// DeleteStartScript deletes a single startup script by ID
func (c *StartScripts) DeleteStartScript(ctx context.Context, id string) error {
	op := &request.Operation{
		Name:       "DeleteStartScript",
		HTTPMethod: "DELETE",
		HTTPPath:   fmt.Sprintf("/scripts/%s", id),
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
