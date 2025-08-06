package credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client/metadata"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const EndpointProviderName ProviderName = "EndpointProvider"

// EndpointProvider satisfies the Provider interface, and is a client to
// retrieve credentials from an arbitrary HTTP endpoint with handler-based validation.
//
// The credentials endpoint Provider can receive both static and refreshable
// credentials that will expire. Credentials are static when an "Expiry"
// value is not provided in the endpoint's response.
//
// Static credentials response format:
//
//	{
//	    "ClientID" : "client_123...",
//	    "ClientSecret" : "secret_456...",
//	    "BaseURL": "https://api.datacrunch.io"
//	}
//
// Refreshable credentials response format:
//
//	{
//	    "ClientID" : "client_123...",
//	    "ClientSecret" : "secret_456...",
//	    "AccessToken" : "token_789...",
//	    "RefreshToken" : "refresh_abc...",
//	    "Expiry" : "2024-12-25T06:03:31Z",
//	    "BaseURL": "https://api.datacrunch.io"
//	}
//
// Error responses should be returned with 400 or 500 HTTP status codes:
//
//	{
//	    "error": "invalid_credentials",
//	    "error_description": "The provided credentials are invalid."
//	}
type EndpointProvider struct {
	staticCreds bool
	expiry      time.Time

	// Client for making HTTP requests to the credentials endpoint
	Client *client.Client

	// ExpiryWindow allows credentials to refresh before actual expiry
	// to avoid race conditions with expiring credentials
	ExpiryWindow time.Duration

	// AuthorizationToken is an optional authorization token for the endpoint request
	AuthorizationToken string

	// AuthorizationTokenProvider dynamically loads the auth token
	AuthorizationTokenProvider AuthTokenProvider

	// Endpoint is the URL to retrieve credentials from
	Endpoint string
}

// AuthTokenProvider defines an interface to dynamically load authorization tokens
type AuthTokenProvider interface {
	GetToken() (string, error)
}

// TokenProviderFunc is a func type implementing AuthTokenProvider interface
type TokenProviderFunc func() (string, error)

// GetToken retrieves auth token according to TokenProviderFunc implementation
func (p TokenProviderFunc) GetToken() (string, error) {
	return p()
}

// EndpointProviderOptions configures the EndpointProvider
type EndpointProviderOptions func(*EndpointProvider)

// WithAuthorizationToken sets a static authorization token
func WithAuthorizationToken(token string) EndpointProviderOptions {
	return func(p *EndpointProvider) {
		p.AuthorizationToken = token
	}
}

// WithAuthorizationTokenProvider sets a dynamic authorization token provider
func WithAuthorizationTokenProvider(provider AuthTokenProvider) EndpointProviderOptions {
	return func(p *EndpointProvider) {
		p.AuthorizationTokenProvider = provider
	}
}

// WithExpiryWindow sets the expiry window for credential refresh
func WithExpiryWindow(window time.Duration) EndpointProviderOptions {
	return func(p *EndpointProvider) {
		p.ExpiryWindow = window
	}
}

// NewEndpointProvider returns a new EndpointProvider for retrieving credentials
// from an HTTP endpoint with handler-based validation and error handling
func NewEndpointProvider(cfg client.Config, endpoint string, options ...EndpointProviderOptions) *EndpointProvider {
	p := &EndpointProvider{
		Endpoint:     endpoint,
		ExpiryWindow: 10 * time.Second, // Default 10 second expiry window
		Client: &client.Client{
			ClientInfo: metadata.ClientInfo{
				ServiceName: "CredentialsEndpoint",
				Endpoint:    endpoint,
			},
			Config: cfg,
		},
	}

	// Set up handlers for credential endpoint validation and processing
	p.setupHandlers()

	// Apply options
	for _, option := range options {
		option(p)
	}

	return p
}

// NewEndpointCredentials returns a new Credentials instance wrapping the EndpointProvider
func NewEndpointCredentials(cfg client.Config, endpoint string, options ...EndpointProviderOptions) *Credentials {
	return NewCredentials(NewEndpointProvider(cfg, endpoint, options...))
}

// setupHandlers configures the request handlers for credential endpoint processing
func (p *EndpointProvider) setupHandlers() {
	// Add unmarshal handler for credential responses that handles both success and error cases
	p.Client.Handlers.Unmarshal.Push(request.NamedHandler{
		Name: "UnmarshalCredentialsHandler",
		Fn:   p.unmarshalHandler,
	})
}

// credentialsResponse represents the JSON response from credentials endpoint
type credentialsResponse struct {
	ClientID     string     `json:"client_id"`
	ClientSecret string     `json:"client_secret"`
	AccessToken  string     `json:"access_token,omitempty"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	BaseURL      string     `json:"base_url,omitempty"`
	Expiry       *time.Time `json:"expiry,omitempty"`
}

// credentialsErrorResponse represents the JSON error response from credentials endpoint
type credentialsErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// unmarshalHandler processes both successful and error responses from credential endpoints
func (p *EndpointProvider) unmarshalHandler(r *request.Request) {
	defer func() {
		if r.HTTPResponse != nil && r.HTTPResponse.Body != nil {
			r.HTTPResponse.Body.Close()
		}
	}()

	// Handle error responses
	if r.HTTPResponse.StatusCode >= 400 {
		var errResp credentialsErrorResponse
		if err := json.NewDecoder(r.HTTPResponse.Body).Decode(&errResp); err != nil {
			r.Error = dcerr.New("CredentialsEndpointError",
				fmt.Sprintf("credentials endpoint returned %d with invalid error response", r.HTTPResponse.StatusCode), err)
			return
		}

		r.Error = dcerr.New("CredentialsEndpointError",
			fmt.Sprintf("credentials endpoint error: %s - %s", errResp.Error, errResp.ErrorDescription), nil)
		return
	}

	// Handle successful responses
	if r.HTTPResponse.StatusCode >= 200 && r.HTTPResponse.StatusCode < 300 {
		credResp := r.Data.(*credentialsResponse)
		if err := json.NewDecoder(r.HTTPResponse.Body).Decode(credResp); err != nil {
			r.Error = dcerr.New("DecodingError", "failed to decode credential endpoint response", err)
			return
		}

		// Validate required fields
		if credResp.ClientID == "" {
			r.Error = dcerr.New("InvalidCredentialsResponse", "client_id is required in credentials response", nil)
			return
		}
		if credResp.ClientSecret == "" {
			r.Error = dcerr.New("InvalidCredentialsResponse", "client_secret is required in credentials response", nil)
			return
		}
		return
	}

	// Handle unexpected status codes
	r.Error = dcerr.New("CredentialsEndpointError",
		fmt.Sprintf("credentials endpoint returned unexpected status code: %d", r.HTTPResponse.StatusCode), nil)
}

// Retrieve retrieves credentials from the endpoint
func (p *EndpointProvider) Retrieve() (Value, error) {
	return p.RetrieveWithContext(context.Background())
}

// RetrieveWithContext retrieves credentials from the endpoint with context support
func (p *EndpointProvider) RetrieveWithContext(ctx context.Context) (Value, error) {
	credResp, err := p.getCredentials(ctx)
	if err != nil {
		return Value{ProviderName: EndpointProviderName}, err
	}

	var expiry time.Time
	// Handle expiry
	if credResp.Expiry != nil {
		expiry = credResp.Expiry.UTC()
		p.expiry = credResp.Expiry.Add(-p.ExpiryWindow)
		p.staticCreds = false
	} else {
		p.staticCreds = true
	}

	return Value{
		ClientID:     credResp.ClientID,
		ClientSecret: credResp.ClientSecret,
		AccessToken:  credResp.AccessToken,
		RefreshToken: credResp.RefreshToken,
		Expiry:       expiry,
		BaseURL:      credResp.BaseURL,
		ProviderName: EndpointProviderName,
	}, nil
}

// getCredentials makes the HTTP request to retrieve credentials
func (p *EndpointProvider) getCredentials(ctx context.Context) (*credentialsResponse, error) {
	operation := &request.Operation{
		Name:       "GetCredentials",
		HTTPMethod: "GET",
		HTTPPath:   "",
	}

	credResp := &credentialsResponse{}
	req := p.Client.NewRequest(operation, nil, credResp)
	req.SetContext(ctx)

	// Override the URL to use our endpoint directly
	req.HTTPRequest.URL.Scheme = "https"
	if strings.HasPrefix(p.Endpoint, "http://") {
		req.HTTPRequest.URL.Scheme = "http"
		req.HTTPRequest.URL.Host = strings.TrimPrefix(p.Endpoint, "http://")
	} else if strings.HasPrefix(p.Endpoint, "https://") {
		req.HTTPRequest.URL.Host = strings.TrimPrefix(p.Endpoint, "https://")
	} else {
		req.HTTPRequest.URL.Host = p.Endpoint
	}

	// Split host and path if needed
	if idx := strings.Index(req.HTTPRequest.URL.Host, "/"); idx >= 0 {
		req.HTTPRequest.URL.Path = req.HTTPRequest.URL.Host[idx:]
		req.HTTPRequest.URL.Host = req.HTTPRequest.URL.Host[:idx]
	}

	req.HTTPRequest.Header.Set("Accept", "application/json")

	// Add authorization token if configured
	authToken := p.AuthorizationToken
	var err error
	if p.AuthorizationTokenProvider != nil {
		authToken, err = p.AuthorizationTokenProvider.GetToken()
		if err != nil {
			return nil, dcerr.New("AuthorizationTokenError", "failed to get authorization token", err)
		}
	}

	if authToken != "" {
		if strings.ContainsAny(authToken, "\r\n") {
			return nil, dcerr.New("InvalidAuthorizationToken", "authorization token contains invalid newline sequence", nil)
		}
		req.HTTPRequest.Header.Set("Authorization", authToken)
	}

	// Send the request (this will run validation, build, unmarshal, and complete handlers)
	if err := req.Send(); err != nil {
		return nil, dcerr.New("CredentialsEndpointError", "failed to retrieve credentials from endpoint", err)
	}

	return credResp, nil
}

// IsExpired returns true if the credentials are expired
func (p *EndpointProvider) IsExpired() bool {
	if p.staticCreds {
		return false
	}
	return time.Now().After(p.expiry)
}
