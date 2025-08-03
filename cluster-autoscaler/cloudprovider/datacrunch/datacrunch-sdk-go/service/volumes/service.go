package volumes

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const (
	// EndpointsID is the service identifier
	EndpointsID = "volumes"
)

// Volumes provides the API operation methods for making requests to
// the volumes service.
type Volumes struct {
	*client.Client
}

// Used for custom client initialization logic
var initClient func(*client.Client)

// Used for custom request initialization logic
var initRequest func(*request.Request)

// New creates a new instance of the volumes client with a session.
func New(cfg *client.Config) *Volumes {
	svc := &Volumes{
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

// NewRequest creates a new request for the volumes service.
func (c *Volumes) NewRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.Client.NewRequest(op, params, data)
	req.HTTPRequest.Header.Set("Accept", "application/json")
	return req
}
