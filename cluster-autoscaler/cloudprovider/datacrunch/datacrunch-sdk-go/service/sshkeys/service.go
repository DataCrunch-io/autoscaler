package sshkeys

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const (
	// EndpointsID is the service identifier
	EndpointsID = "sshkey"
)

// SSHKey provides the API operation methods for making requests to
// the SSH key service.
type SSHKey struct {
	*client.Client
}

// Client is an alias for SSHKey to match the expected interface
type Client = *SSHKey

// Used for custom client initialization logic
var initClient func(*client.Client)

// Used for custom request initialization logic
var initRequest func(*request.Request)

// New creates a new instance of the SSH key client with a session.
func New(cfg *client.Config) *SSHKey {
	svc := &SSHKey{
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

// NewRequest creates a new request for the SSH key service.
func (c *SSHKey) NewRequest(op *request.Operation, params, data interface{}) *request.Request {
	req := c.Client.NewRequest(op, params, data)
	req.HTTPRequest.Header.Set("Accept", "application/json")
	return req
}

// NewClient creates a new SSH keys client with the provided HTTP client wrapper
func NewClient(httpClient interface{}) Client {
	// For now, return a simple SSHKey client - this would be enhanced with proper client wrapping
	return &SSHKey{}
}
