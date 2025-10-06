package datacrunch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DefaultBaseURL = "https://api.datacrunch.io/v1"
)

type Client struct {
	BaseURL         string
	ClientID        string
	ClientSecret    string
	AuthBearerToken string

	HTTPClient *http.Client
	Logger     Logger

	// Middleware management for all requests
	Middleware *Middleware

	// Services
	Auth           *AuthService
	Balance        *BalanceService
	Instances      *InstanceService
	Volumes        *VolumeService
	SSHKeys        *SSHKeyService
	StartupScripts *StartupScriptService
	Locations      *LocationService
	Containers     *ContainerService
}

type ClientOption func(*Client)

func NewClient(options ...ClientOption) (*Client, error) {

	client := &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{},
		Logger:     &NoOpLogger{}, // Default: no logging
	}

	for _, option := range options {
		option(client)
	}

	// Initialize middleware with the configured logger
	client.Middleware = NewDefaultMiddleware(client.Logger)

	// Validate required fields
	if client.ClientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}
	if client.ClientSecret == "" {
		return nil, fmt.Errorf("client secret is required")
	}

	client.Auth = &AuthService{client: client}
	client.Balance = &BalanceService{client: client}
	client.Instances = &InstanceService{client: client}
	client.Volumes = &VolumeService{client: client}
	client.SSHKeys = &SSHKeyService{client: client}
	client.StartupScripts = &StartupScriptService{client: client}
	client.Locations = &LocationService{client: client}
	client.Containers = &ContainerService{client: client}

	return client, nil
}

// AddRequestMiddleware adds a request middleware to the client
func (c *Client) AddRequestMiddleware(middleware RequestMiddleware) {
	c.Middleware.AddRequestMiddleware(middleware)
}

// AddResponseMiddleware adds a response middleware to the client
func (c *Client) AddResponseMiddleware(middleware ResponseMiddleware) {
	c.Middleware.AddResponseMiddleware(middleware)
}

// SetRequestMiddleware replaces all request middleware
func (c *Client) SetRequestMiddleware(middleware []RequestMiddleware) {
	c.Middleware.SetRequestMiddleware(middleware)
}

// SetResponseMiddleware replaces all response middleware
func (c *Client) SetResponseMiddleware(middleware []ResponseMiddleware) {
	c.Middleware.SetResponseMiddleware(middleware)
}

// ClearRequestMiddleware removes all request middleware
func (c *Client) ClearRequestMiddleware() {
	c.Middleware.ClearRequestMiddleware()
}

// ClearResponseMiddleware removes all response middleware
func (c *Client) ClearResponseMiddleware() {
	c.Middleware.ClearResponseMiddleware()
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

func WithClientID(clientID string) ClientOption {
	return func(c *Client) {
		c.ClientID = clientID
	}
}

func WithClientSecret(clientSecret string) ClientOption {
	return func(c *Client) {
		c.ClientSecret = clientSecret

	}
}

func WithAuthBearerToken(token string) ClientOption {
	return func(c *Client) {
		c.AuthBearerToken = token
	}
}

// WithLogger sets a custom logger for the client
func WithLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.Logger = logger
	}
}

// WithDebugLogging enables debug logging using the standard logger
func WithDebugLogging(enabled bool) ClientOption {
	return func(c *Client) {
		c.Logger = NewStdLogger(enabled)
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

func (c *Client) WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req = req.WithContext(ctx)
	return req, nil
}

// Do executes an HTTP request with middleware support and handles the response
func (c *Client) Do(req *http.Request, result any) (*Response, error) {
	// Get thread-safe copies of client's default middleware
	requestMiddleware, responseMiddleware := c.Middleware.Snapshot()

	// Create request context from the HTTP request
	reqCtx := &RequestContext{
		Method:  req.Method,
		Path:    req.URL.Path,
		Body:    nil, // Body is already in the request
		Headers: req.Header.Clone(),
		Query:   req.URL.Query(),
		Client:  c,
		Request: req,
	}

	// Build and execute request middleware chain
	requestHandler := c.buildRequestChain(requestMiddleware)
	if err := requestHandler(reqCtx); err != nil {
		return nil, fmt.Errorf("request middleware failed: %w", err)
	}

	// Apply any headers modified by middleware
	for name, values := range reqCtx.Headers {
		req.Header.Del(name) // Clear existing
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Execute the HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Wrap the response
	wrappedResp := &Response{Response: resp}

	// Handle response parsing if result is provided
	if result != nil {
		if err := c.handleResponse(resp, result); err != nil {
			// Create response context for middleware with error
			respCtx := &ResponseContext{
				Request:    reqCtx,
				Response:   resp,
				Body:       nil, // Body was consumed by handleResponse
				StatusCode: resp.StatusCode,
				Error:      err,
			}

			// Build and execute response middleware chain
			responseHandler := c.buildResponseChain(responseMiddleware)
			if middlewareErr := responseHandler(respCtx); middlewareErr != nil {
				return wrappedResp, fmt.Errorf("response middleware failed: %w", middlewareErr)
			}

			// Return the original error or modified error from middleware
			if respCtx.Error != nil {
				return wrappedResp, respCtx.Error
			}
			return wrappedResp, err
		}
	}

	// Create response context for successful response
	respCtx := &ResponseContext{
		Request:    reqCtx,
		Response:   resp,
		Body:       nil, // Body was consumed by handleResponse
		StatusCode: resp.StatusCode,
		Error:      nil,
	}

	// Build and execute response middleware chain
	responseHandler := c.buildResponseChain(responseMiddleware)
	if err := responseHandler(respCtx); err != nil {
		return wrappedResp, fmt.Errorf("response middleware failed: %w", err)
	}

	// Check if middleware set an error
	if respCtx.Error != nil {
		return wrappedResp, respCtx.Error
	}

	return wrappedResp, nil
}

// buildRequestChain builds the request middleware chain
func (c *Client) buildRequestChain(requestMiddleware []RequestMiddleware) RequestHandler {
	// Default handler that does nothing
	handler := func(ctx *RequestContext) error {
		return nil
	}

	// Apply middleware in reverse order (last middleware wraps first)
	for i := len(requestMiddleware) - 1; i >= 0; i-- {
		handler = requestMiddleware[i](handler)
	}

	return handler
}

// buildResponseChain builds the response middleware chain
func (c *Client) buildResponseChain(responseMiddleware []ResponseMiddleware) ResponseHandler {
	// Default handler that does nothing
	handler := func(ctx *ResponseContext) error {
		return nil
	}

	// Apply middleware in reverse order (last middleware wraps first)
	for i := len(responseMiddleware) - 1; i >= 0; i-- {
		handler = responseMiddleware[i](handler)
	}

	return handler
}

// generic request alternative request
func (c *Client) makeRequest(method, path string, body any) (*http.Response, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	bearerToken, err := c.Auth.GetBearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %w", err)
	}
	req.Header.Set("Authorization", bearerToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handleResponse reads the response body and unmarshals it into the result if provided
func (c *Client) handleResponse(resp *http.Response, result any) error {
	defer func() {
		_ = resp.Body.Close() // Ignore close error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		apiError.StatusCode = resp.StatusCode
		return &apiError
	}

	// If there's no response body, nothing to unmarshal
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}

	if result != nil {
		if err := json.Unmarshal(trimmed, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
