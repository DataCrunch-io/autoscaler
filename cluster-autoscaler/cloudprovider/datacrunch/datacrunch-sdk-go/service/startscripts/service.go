package startscripts

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const (
	// EndpointsID is the service identifier
	EndpointsID = "startscripts"
)

// StartScripts provides the API operation methods for making requests to
// the startup scripts service.
type StartScripts struct {
	*client.Client
}

// Client is an alias for StartScripts to match the expected interface
type Client = *StartScripts

// Used for custom client initialization logic
var initClient func(*client.Client)

// New creates a new instance of the startup scripts client with a session.
func New(cfg *client.Config) *StartScripts {
	svc := &StartScripts{
		Client: client.New(cfg, metadata.ClientInfo{
			ServiceName: EndpointsID,
			APIVersion:  "v1",
			Endpoint:    "https://api.datacrunch.io/v1",
		}, request.Handlers{}),
	}

	// Run custom client initialization if present
	if initClient != nil {
		initClient(svc.Client)
	}

	return svc
}

// NewRequest creates a new request for the startup scripts service.
func (c *StartScripts) NewRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.Client.NewRequest(op, params, data)
	req.HTTPRequest.Header.Set("Accept", "application/json")
	return req
}

// NewClient creates a new start scripts client with the provided HTTP client wrapper
func NewClient(httpClient interface{}) Client {
	// For now, return a simple StartScripts client - this would be enhanced with proper client wrapping
	return &StartScripts{}
}
