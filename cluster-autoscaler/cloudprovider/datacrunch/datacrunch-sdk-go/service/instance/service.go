package instance

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/restjson"
)

const (
	EndpointsID = "instance"
	APIVersion  = "v1"
)

// Instance provides the API operation methods for making requests to
// DataCrunch Instance API
type Instance struct {
	*client.Client
}

// Client is an alias for Instance to match the expected interface
type Client = *Instance

// Used for custom client initialization logic
var initClient func(*client.Client)

// Used for custom request initialization logic
var initRequest func(*request.Request)

// New creates a new instance of the Instance client with a config provider.
func New(p client.ConfigProvider, cfgs ...*datacrunch.Config) *Instance {
	c := p.ClientConfig(EndpointsID, cfgs...)

	return newClient(c.Config, c.Handlers)
}

// newClient creates, initializes and returns a new service client instance.
func newClient(cfg datacrunch.Config, handlers request.Handlers) *Instance {

	svc := &Instance{
		Client: client.New(cfg, metadata.ClientInfo{
			ServiceName: EndpointsID,
			APIVersion:  APIVersion,
			Endpoint:    *cfg.BaseURL,
		}, handlers),
	}

	// Add protocol handlers for REST JSON
	svc.Handlers.Build.PushBackNamed(restjson.BuildHandler)
	svc.Handlers.Unmarshal.PushBackNamed(restjson.UnmarshalHandler)
	svc.Handlers.Complete.PushBackNamed(restjson.UnmarshalMetaHandler)

	// Run custom client initialization if present
	if initClient != nil {
		initClient(svc.Client)
	}

	return svc
}

func (c *Instance) newRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.NewRequest(op, params, data)

	// Run custom request initialization if present
	if initRequest != nil {
		initRequest(req)
	}

	return req
}
