package startscriptsiface

import (
	"context"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
)

// StartScriptsAPI provides the interface for the startup scripts service
type StartScriptsAPI interface {
	// ListStartScripts lists all startup scripts
	ListStartScripts(ctx context.Context) ([]*startscripts.StartScriptResponse, error)
	// GetStartScript gets a single startup script by ID
	GetStartScript(ctx context.Context, id string) (*startscripts.StartScriptResponse, error)
	// CreateStartScript creates a new startup script
	CreateStartScript(ctx context.Context, input *startscripts.CreateStartScriptInput) (string, error)
	// DeleteStartScripts deletes multiple startup scripts
	DeleteStartScripts(ctx context.Context, input *startscripts.DeleteStartScriptsInput) error
	// DeleteStartScript deletes a single startup script by ID
	DeleteStartScript(ctx context.Context, id string) error
}

var _ StartScriptsAPI = (*startscripts.StartScripts)(nil)
