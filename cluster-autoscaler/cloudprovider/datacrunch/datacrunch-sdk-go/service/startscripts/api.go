package startscripts

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/restjson"
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

type GetStartScriptInput struct {
	ID string `location:"uri" locationName:"id"`
}

type DeleteStartScriptInput struct {
	ID string `location:"uri" locationName:"id"`
}

// ListStartScripts lists all startup scripts
func (c *StartScripts) ListStartScripts() ([]*StartScriptResponse, error) {
	op := &request.Operation{
		Name:       "ListStartScripts",
		HTTPMethod: "GET",
		HTTPPath:   "/scripts",
	}

	var scripts []*StartScriptResponse
	req := c.newRequest(op, nil, &scripts)

	return scripts, req.Send()
}

// GetStartScript gets a single startup script by ID
func (c *StartScripts) GetStartScript(id string) ([]*StartScriptResponse, error) {
	op := &request.Operation{
		Name:       "GetStartScript",
		HTTPMethod: "GET",
		HTTPPath:   "/scripts/{id}",
	}

	input := &GetStartScriptInput{
		ID: id,
	}

	// API returns array, so unmarshal as array and take first element
	var scripts []*StartScriptResponse
	req := c.newRequest(op, input, &scripts)

	err := req.Send()
	if err != nil {
		return nil, err
	}

	return scripts, nil
}

// CreateStartScript creates a new startup script
func (c *StartScripts) CreateStartScript(input *CreateStartScriptInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateStartScript",
		HTTPMethod: "POST",
		HTTPPath:   "/scripts",
	}

	var scriptID string
	req := c.newRequest(op, input, &scriptID)

	// This API returns a plain string, not JSON, so use string unmarshaler
	req.Handlers.Unmarshal.RemoveByName("datacrunchsdk.restjson.Unmarshal")
	req.Handlers.Unmarshal.PushBackNamed(restjson.StringUnmarshalHandler)

	return scriptID, req.Send()
}

// DeleteStartScripts deletes multiple startup scripts
func (c *StartScripts) DeleteStartScripts(input *DeleteStartScriptsInput) error {
	op := &request.Operation{
		Name:       "DeleteStartScripts",
		HTTPMethod: "DELETE",
		HTTPPath:   "/scripts",
	}

	req := c.newRequest(op, input, nil)

	return req.Send()
}

// DeleteStartScript deletes a single startup script by ID
func (c *StartScripts) DeleteStartScript(id string) error {
	op := &request.Operation{
		Name:       "DeleteStartScript",
		HTTPMethod: "DELETE",
		HTTPPath:   "/scripts/{id}",
	}

	input := &DeleteStartScriptInput{
		ID: id,
	}

	req := c.newRequest(op, input, nil)

	return req.Send()
}
