package startscripts

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/restjson"
)

const (
	EndpointsID = "startscripts"
	APIVersion  = "v1"
)

// StartScripts provides the API operation methods for making requests to
type StartScripts struct {
	*client.Client
}

// Client is an alias for StartScripts to match the expected interface
type Client = *StartScripts

// Used for custom client initialization logic
var initClient func(*client.Client)

// Used for custom request initialization logic
var initRequest func(*request.Request)

// New creates a new instance of the StartScripts client with a config provider.
// If additional configuration is needed for the client instance use the optional
// client.Config parameter to add your extra config.
//
// Example:
//
//	mySession := session.Must(session.New())
//
//	// Create a StartScripts client from just a session.
//	svc := startscripts.New(mySession)
//
//	// Create a StartScripts client with additional configuration
//	svc := startscripts.New(mySession, &client.Config{Timeout: 60 * time.Second})
func New(p client.ConfigProvider, cfgs ...*datacrunch.Config) *StartScripts {
	c := p.ClientConfig(EndpointsID, cfgs...)
	return newClient(c.Config, c.Handlers)
}

// newClient creates, initializes and returns a new service client instance.
func newClient(cfg datacrunch.Config, handlers request.Handlers) *StartScripts {

	svc := &StartScripts{
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

func (c *StartScripts) newRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.NewRequest(op, params, data)

	// Run custom request initialization if present
	if initRequest != nil {
		initRequest(req)
	}

	return req
}
