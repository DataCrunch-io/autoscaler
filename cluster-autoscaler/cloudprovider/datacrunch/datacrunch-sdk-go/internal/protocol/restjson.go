package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RESTJSONProtocol handles REST JSON protocol for DataCrunch API
type RESTJSONProtocol struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// NewRESTJSONProtocol creates a new REST JSON protocol handler
func NewRESTJSONProtocol(baseURL string, client *http.Client) *RESTJSONProtocol {
	return &RESTJSONProtocol{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		HTTPClient: client,
		UserAgent:  "datacrunch-sdk-go/1.0.0",
	}
}

// Request represents an API request
type Request struct {
	Method      string
	Path        string
	Query       url.Values
	Headers     map[string]string
	Body        interface{}
	ContentType string
}

// Response represents an API response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Execute executes an API request
func (p *RESTJSONProtocol) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Build URL
	requestURL := p.BaseURL + req.Path
	if len(req.Query) > 0 {
		requestURL += "?" + req.Query.Encode()
	}

	// Prepare request body
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("User-Agent", p.UserAgent)

	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	} else if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	httpReq.Header.Set("Accept", "application/json")

	// Add custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	httpResp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer func() {
		if err := httpResp.Body.Close(); err != nil {
			// Log the error but don't fail the function
			_ = err // Suppress unused variable warning
		}
	}()

	// Read response body
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Header,
		Body:       bodyBytes,
	}, nil
}

// UnmarshalResponse unmarshals a JSON response into a target struct
func (p *RESTJSONProtocol) UnmarshalResponse(resp *Response, target interface{}) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error response
		var apiError APIError
		if err := json.Unmarshal(resp.Body, &apiError); err == nil {
			return &apiError
		}
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(resp.Body))
	}

	if target == nil {
		return nil
	}

	if len(resp.Body) == 0 {
		return nil
	}

	if err := json.Unmarshal(resp.Body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return nil
}

// APIError represents an API error response
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %d: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

// BuildRequest is a helper to build common REST requests
func BuildRequest(method, path string) *Request {
	return &Request{
		Method:  method,
		Path:    path,
		Query:   make(url.Values),
		Headers: make(map[string]string),
	}
}

// WithQuery adds query parameters to a request
func (r *Request) WithQuery(key, value string) *Request {
	r.Query.Set(key, value)
	return r
}

// WithHeader adds a header to a request
func (r *Request) WithHeader(key, value string) *Request {
	r.Headers[key] = value
	return r
}

// WithBody sets the request body
func (r *Request) WithBody(body interface{}) *Request {
	r.Body = body
	return r
}
