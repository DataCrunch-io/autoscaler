package datacrunch

import (
	"context"
	"net/http"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/common"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/sshkeys"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
)

type RequestRetryer interface{}

// Config holds configuration for the DataCrunch SDK
type Config struct {
	// API configuration
	BaseURL      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration

	// HTTP client configuration
	Retryer       RequestRetryer
	MaxRetries    int
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL:       "https://api.datacrunch.io/v1",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 30 * time.Second,
	}
}

// Client represents the main DataCrunch SDK client
type Client struct {
	config *Config

	// HTTP client
	httpClient *client.Client

	// Service clients
	Instance     instance.Client
	SSHKeys      sshkeys.Client
	StartScripts startscripts.Client
}

// New creates a new DataCrunch SDK client
func New(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	// Create HTTP client
	httpClient := client.New(config, metadata.ClientInfo{
		ServiceName: "datacrunch",
		APIVersion:  "v1",
		Endpoint:    config.BaseURL,
	}, request.Handlers{})

	// Create wrapper for service clients
	wrapper := &httpClientWrapper{client: httpClient}

	return &Client{
		config:       config,
		httpClient:   httpClient,
		Instance:     instance.NewClient(wrapper),
		SSHKeys:      sshkeys.NewClient(wrapper),
		StartScripts: startscripts.NewClient(wrapper),
	}
}

// httpClientWrapper adapts the HTTP client for service clients
type httpClientWrapper struct {
	client *client.Client
}

// httpResponse adapts the HTTP response for service clients
type httpResponse struct {
	statusCode int
	body       []byte
	response   interface{}
}

// Post implements the APIClientInterface for service clients
func (w *httpClientWrapper) Post(ctx context.Context, path string, body interface{}) (common.ResponseInterface, error) {
	resp, err := w.client.Post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		response:   resp,
	}, nil
}

// Get implements the APIClientInterface for service clients
func (w *httpClientWrapper) Get(ctx context.Context, path string) (common.ResponseInterface, error) {
	resp, err := w.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		response:   resp,
	}, nil
}

// Delete implements the APIClientInterface for service clients
func (w *httpClientWrapper) Delete(ctx context.Context, path string) (common.ResponseInterface, error) {
	resp, err := w.client.Delete(ctx, path)
	if err != nil {
		return nil, err
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		response:   resp,
	}, nil
}

// Put implements the APIClientInterface for service clients
func (w *httpClientWrapper) Put(ctx context.Context, path string, body interface{}) (common.ResponseInterface, error) {
	resp, err := w.client.Put(ctx, path, body)
	if err != nil {
		return nil, err
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		response:   resp,
	}, nil
}

// DecodeJSON implements ResponseInterface
func (r *httpResponse) DecodeJSON(target interface{}) error {
	if httpResp, ok := r.response.(*http.Response); ok {
		// Use the client's DecodeResponse method
		client := &client.Client{} // This is a placeholder - in real implementation would use the actual client
		return client.DecodeResponse(httpResp, target)
	}
	return nil
}

// GetStatusCode implements ResponseInterface
func (r *httpResponse) GetStatusCode() int {
	return r.statusCode
}

// GetBody implements ResponseInterface
func (r *httpResponse) GetBody() []byte {
	return r.body
}
