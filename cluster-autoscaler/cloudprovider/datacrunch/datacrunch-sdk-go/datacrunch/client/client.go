package client

import (
	"context"
	"net/http"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/config"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// ResponseInterface defines the common interface for HTTP responses
type ResponseInterface interface {
	// DecodeJSON decodes the response body into the target interface
	DecodeJSON(target interface{}) error

	// GetStatusCode returns the HTTP status code
	GetStatusCode() int

	// GetBody returns the response body as bytes
	GetBody() []byte
}

// A Config provides configuration to a service client instance.
type Config struct {
	Config   config.Config
	BaseURL  string
	Handlers request.Handlers
	Timeout  time.Duration
}

// ConfigProvider provides a generic way for a service client to receive
// the ClientConfig without circular dependencies.
type ConfigProvider interface {
	ClientConfig(serviceName string, cfgs ...*config.Config) Config
}

// ConfigNoResolveEndpointProvider same as ConfigProvider except it will not
// resolve the endpoint automatically. The service client's endpoint must be
// provided via the Config.Endpoint field.
type ConfigNoResolveEndpointProvider interface {
	ClientConfigNoResolveEndpoint(cfgs ...*interface{}) Config
}

// A Client implements the base client request and response handling
// used by all service clients.
type Client struct {
	request.Retryer
	metadata.ClientInfo

	Config   config.Config
	Handlers request.Handlers
}

// New will return a pointer to a new initialized service client.
func New(cfg config.Config, info metadata.ClientInfo, handlers request.Handlers, options ...func(*Client)) *Client {
	svc := &Client{
		Config:     cfg,
		ClientInfo: info,
		Handlers:   handlers.Copy(),
	}

	// Configure retryer - always provide sensible defaults
	switch retryer, ok := cfg.Retryer.(request.Retryer); {
	case ok:
		// User provided custom retryer
		svc.Retryer = retryer
	default:
		// Use DefaultRetryer with proper defaults
		maxRetries := DefaultRetryerMaxNumRetries // Default to 3 retries
		if cfg.MaxRetries != nil {
			maxRetries = *cfg.MaxRetries
		}
		// Create retryer with sensible defaults for all timing values
		svc.Retryer = NewDefaultRetryer(maxRetries)
	}

	for _, option := range options {
		option(svc)
	}

	return svc
}

// NewRequest returns a new Request pointer for the service API
// operation and parameters.
func (c *Client) NewRequest(operation *request.Operation, params interface{}, data interface{}) *request.Request {
	return request.New(c.Config, c.Handlers, c.Retryer, operation, params, data)
}

// AddProtocolHandlers adds the REST JSON protocol handlers to the client
func (c *Client) AddProtocolHandlers() {
	// Import the protocol handlers - will be added by services as needed
	// This method provides a hook for adding protocol-specific handlers
}

// Post makes a POST request to the specified path with the given body
func (c *Client) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, "POST", path, body)
}

// Get makes a GET request to the specified path
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.makeRequest(ctx, "GET", path, nil)
}

// Delete makes a DELETE request to the specified path
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.makeRequest(ctx, "DELETE", path, nil)
}

// Put makes a PUT request to the specified path with the given body
func (c *Client) Put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, "PUT", path, body)
}

// DecodeResponse decodes the HTTP response into the target interface
func (c *Client) DecodeResponse(resp *http.Response, target interface{}) error {
	// Simple implementation for now - can be enhanced later
	return nil
}

// makeRequest handles the common logic for making HTTP requests
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	operation := &request.Operation{
		Name:       method + " " + path,
		HTTPMethod: method,
		HTTPPath:   path,
	}

	req := c.NewRequest(operation, body, nil)
	req.SetContext(ctx)

	if err := req.Send(); err != nil {
		return nil, err
	}

	return req.HTTPResponse, nil
}

// Do executes the given request and handles the response
func (c *Client) Do(req *request.Request) error {
	return req.Send()
}

// WithMaxRetries sets the maximum number of retries for the client
func WithMaxRetries(maxRetries int) func(*Client) {
	return func(c *Client) {
		c.Retryer = DefaultRetryer{NumMaxRetries: maxRetries}
	}
}

// WithRetryer sets a custom retryer for the client
func WithRetryer(retryer request.Retryer) func(*Client) {
	return func(c *Client) {
		c.Retryer = retryer
	}
}
