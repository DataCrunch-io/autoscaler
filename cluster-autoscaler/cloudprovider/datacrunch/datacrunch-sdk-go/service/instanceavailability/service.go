package instanceavailability

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const (
	// EndpointsID is the service identifier
	EndpointsID = "instance-availability"
)

// InstanceAvailability provides the API operation methods for making requests to
// the instance-availability service.
type InstanceAvailability struct {
	*client.Client
}

// Used for custom client initialization logic
var initClient func(*client.Client)

// Used for custom request initialization logic
var initRequest func(*request.Request)

// New creates a new instance of the InstanceAvailability client with a session.
func New(cfg *client.Config) *InstanceAvailability {
	svc := &InstanceAvailability{
		Client: client.New(
			cfg,
			metadata.ClientInfo{
				ServiceName: EndpointsID,
				Endpoint:    "https://api.datacrunch.io/v1",
			},
			request.Handlers{},
		),
	}

	// Run custom client initialization if present
	if initClient != nil {
		initClient(svc.Client)
	}

	return svc
}

// NewRequest creates a new request for the InstanceAvailability service.
func (c *InstanceAvailability) NewRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.Client.NewRequest(op, params, data)
	req.HTTPRequest.Header.Set("Accept", "application/json")
	return req
}
