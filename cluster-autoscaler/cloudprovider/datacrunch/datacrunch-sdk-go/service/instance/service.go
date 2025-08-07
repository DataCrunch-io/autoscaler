package instance

import (
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

type ConfigProvider interface {
	ClientConfig(serviceName string, cfgs ...*interface{}) client.Config
}

// New creates a new instance of the Instance client with a config provider.
// If additional configuration is needed for the client instance use the optional
// client.Config parameter to add your extra config.
//
// Example:
//
//	mySession := session.Must(session.New())
//
//	// Create a Instance client from just a session.
//	svc := instance.New(mySession)
//
//	// Create a Instance client with additional configuration
//	svc := instance.New(mySession, &client.Config{Timeout: 60 * time.Second})
func New(p ConfigProvider, cfgs ...*interface{}) *Instance {
	c := p.ClientConfig(EndpointsID, cfgs...)
	return newClient(c)
}

// newClient creates, initializes and returns a new service client instance.
func newClient(cfg client.Config) *Instance {
	handlers := request.Handlers{}

	// Add protocol handlers for REST JSON
	handlers.Build.PushBackNamed(restjson.BuildHandler)
	handlers.Unmarshal.PushBackNamed(restjson.UnmarshalHandler)
	handlers.Complete.PushBackNamed(restjson.UnmarshalMetaHandler)

	svc := &Instance{
		Client: client.New(&cfg, metadata.ClientInfo{
			ServiceName: EndpointsID,
			APIVersion:  APIVersion,
			Endpoint:    cfg.BaseURL,
		}, handlers),
	}

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
