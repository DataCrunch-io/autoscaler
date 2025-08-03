package startscripts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

	var scripts []*StartScriptResponse
	req := c.NewRequest(op, nil, &scripts)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
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

	var script StartScriptResponse
	req := c.NewRequest(op, nil, &script)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
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

	var scriptID string
	req := c.NewRequest(op, input, &scriptID)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return "", err
	}

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

// DeleteStartScript deletes a single startup script by ID
func (c *StartScripts) DeleteStartScript(ctx context.Context, id string) error {
	op := &request.Operation{
		Name:       "DeleteStartScript",
		HTTPMethod: "DELETE",
		HTTPPath:   fmt.Sprintf("/scripts/%s", id),
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
