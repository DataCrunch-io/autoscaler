package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
)

// StartScriptsAPI provides the interface for the startup scripts service
type StartScriptsAPI interface {
	// ListStartScripts lists all startup scripts
	ListStartScripts() ([]*startscripts.StartScriptResponse, error)
	// GetStartScript gets a single startup script by ID
	GetStartScript(id string) (*startscripts.StartScriptResponse, error)
	// CreateStartScript creates a new startup script
	CreateStartScript(input *startscripts.CreateStartScriptInput) (string, error)
	// DeleteStartScripts deletes multiple startup scripts
	DeleteStartScripts(input *startscripts.DeleteStartScriptsInput) error
	// DeleteStartScript deletes a single startup script by ID
	DeleteStartScript(id string) error
}

var _ StartScriptsAPI = (*startscripts.StartScripts)(nil)
