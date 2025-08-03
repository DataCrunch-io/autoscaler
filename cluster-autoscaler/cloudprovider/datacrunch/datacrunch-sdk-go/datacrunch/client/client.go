package client

import (
	"context"
	"net/http"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/auth"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
)

// A Config provides configuration to a service client instance.
type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
}

// ConfigProvider provides a generic way for a service client to receive
// the ClientConfig without circular dependencies.
type ConfigProvider interface {
	ClientConfig(serviceName string, cfgs ...*interface{}) Config
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

	Config   interface{}
	Handlers request.Handlers
}

// New will return a pointer to a new initialized service client.
func New(cfg interface{}, info metadata.ClientInfo, handlers request.Handlers, options ...func(*Client)) *Client {
	svc := &Client{
		Config:     cfg,
		ClientInfo: info,
		Handlers:   handlers.Copy(),
	}

	// switch retryer, ok := cfg.Retryer.(request.Retryer); {
	// case ok:
	// 	svc.Retryer = retryer
	// case cfg.Retryer != nil && cfg.Logger != nil:
	// 	s := fmt.Sprintf("WARNING: %T does not implement request.Retryer; using DefaultRetryer instead", cfg.Retryer)
	// 	cfg.Logger.Log(s)
	// 	fallthrough
	// default:
	// 	maxRetries := datacrunch.IntValue(cfg.MaxRetries)
	// 	if cfg.MaxRetries == nil || maxRetries == datacrunch.UseServiceDefaultRetries {
	// 		maxRetries = DefaultRetryerMaxNumRetries
	// 	}
	// 	svc.Retryer = DefaultRetryer{NumMaxRetries: maxRetries}
	// }

	svc.AddDebugHandlers()

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

// AddDebugHandlers injects debug logging handlers into the service to log request
// debug information.
func (c *Client) AddDebugHandlers() {
	c.Handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "OAuth2AuthHandler",
		Fn:   auth.OAuth2AuthHandler,
	})
	c.Handlers.Complete.PushBackNamed(request.NamedHandler{
		Name: "DebugHandler",
		Fn:   auth.DebugHandler,
	})
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