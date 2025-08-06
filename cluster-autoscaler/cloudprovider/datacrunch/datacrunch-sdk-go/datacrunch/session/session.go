package session

import (
	"os"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/defaults"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// Session provides a shared configuration and state for service clients
type Session struct {
	Config      *client.Config
	Handlers    request.Handlers
	Credentials *credentials.Credentials
}

// Options for configuring a session
type Options struct {
	// API configuration
	BaseURL      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration

	// Credential configuration
	Credentials                   *credentials.Credentials
	CredentialsChainVerboseErrors *bool

	// Retry configuration
	MaxRetries *int
	Retryer    interface{}
}

// DefaultOptions returns default session options with sensible retry defaults
func DefaultOptions() *Options {
	defaultMaxRetries := 3 // Provide good defaults for all users
	return &Options{
		BaseURL:    "https://api.datacrunch.io/v1",
		Timeout:    30 * time.Second,
		MaxRetries: &defaultMaxRetries, // Default to 3 retries for resilience
	}
}

// New creates a new session with the provided options
func New(options ...func(*Options)) *Session {
	opts := DefaultOptions()

	// Apply all options
	for _, option := range options {
		option(opts)
	}

	config := &client.Config{
		BaseURL:      opts.BaseURL,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Timeout:      opts.Timeout,
		MaxRetries:   opts.MaxRetries,
		Retryer:      opts.Retryer,
	}

	// Setup credentials
	var creds *credentials.Credentials
	if opts.Credentials != nil {
		creds = opts.Credentials
	} else if opts.ClientID != "" && opts.ClientSecret != "" {
		// Use static credentials if provided directly
		creds = credentials.NewStaticCredentials(opts.ClientID, opts.ClientSecret, opts.BaseURL)
	} else {
		// Use default credential chain
		creds = defaults.CredChain()
	}

	// If no explicit base URL was provided, try to get it from credentials
	if opts.BaseURL == "https://api.datacrunch.io/v1" { // Only if using default
		if credValue, err := creds.Get(); err == nil && credValue.BaseURL != "" {
			config.BaseURL = credValue.BaseURL
		}
	}

	return &Session{
		Config:      config,
		Handlers:    defaults.Handlers(),
		Credentials: creds,
	}
}

// NewFromEnv creates a new session using environment variables
// Supported environment variables:
// - DATACRUNCH_BASE_URL (default: https://api.datacrunch.io/v1)
// - DATACRUNCH_CLIENT_ID (required)
// - DATACRUNCH_CLIENT_SECRET (required)
// - DATACRUNCH_TIMEOUT (default: 30s, format: "30s", "1m", etc.)
func NewFromEnv(options ...func(*Options)) *Session {
	opts := DefaultOptions()

	// Load from environment variables
	if baseURL := os.Getenv("DATACRUNCH_BASE_URL"); baseURL != "" {
		opts.BaseURL = baseURL
	}

	if clientID := os.Getenv("DATACRUNCH_CLIENT_ID"); clientID != "" {
		opts.ClientID = clientID
	}

	if clientSecret := os.Getenv("DATACRUNCH_CLIENT_SECRET"); clientSecret != "" {
		opts.ClientSecret = clientSecret
	}

	if timeoutStr := os.Getenv("DATACRUNCH_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			opts.Timeout = timeout
		}
	}

	// Apply additional options (these can override env vars)
	for _, option := range options {
		option(opts)
	}

	config := &client.Config{
		BaseURL:      opts.BaseURL,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Timeout:      opts.Timeout,
		MaxRetries:   opts.MaxRetries,
		Retryer:      opts.Retryer,
	}

	// Setup credentials
	var creds *credentials.Credentials
	if opts.Credentials != nil {
		creds = opts.Credentials
	} else if opts.ClientID != "" && opts.ClientSecret != "" {
		// Use static credentials if provided directly
		creds = credentials.NewStaticCredentials(opts.ClientID, opts.ClientSecret, opts.BaseURL)
	} else {
		// Use default credential chain
		creds = defaults.CredChain()
	}

	// If no explicit base URL was provided, try to get it from credentials
	if opts.BaseURL == "https://api.datacrunch.io/v1" { // Only if using default
		if credValue, err := creds.Get(); err == nil && credValue.BaseURL != "" {
			config.BaseURL = credValue.BaseURL
		}
	}

	return &Session{
		Config:      config,
		Handlers:    defaults.Handlers(),
		Credentials: creds,
	}
}

// WithBaseURL sets the base URL for the API
func WithBaseURL(baseURL string) func(*Options) {
	return func(o *Options) {
		o.BaseURL = baseURL
	}
}

// WithCredentials sets the OAuth2 client credentials
func WithCredentials(clientID, clientSecret string) func(*Options) {
	return func(o *Options) {
		o.ClientID = clientID
		o.ClientSecret = clientSecret
	}
}

// WithMaxRetries sets the maximum number of retries for requests
func WithMaxRetries(maxRetries int) func(*Options) {
	return func(o *Options) {
		o.MaxRetries = &maxRetries
	}
}

// WithRetryer sets a custom retryer implementation
func WithRetryer(retryer interface{}) func(*Options) {
	return func(o *Options) {
		o.Retryer = retryer
	}
}

// WithNoRetries disables retry functionality entirely
func WithNoRetries() func(*Options) {
	return WithMaxRetries(0)
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) func(*Options) {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithCredentials sets custom credentials
func WithCredentialsProvider(creds *credentials.Credentials) func(*Options) {
	return func(o *Options) {
		o.Credentials = creds
	}
}

// WithCredentialChainVerboseErrors sets whether to use verbose errors in credential chain
func WithCredentialChainVerboseErrors(verbose bool) func(*Options) {
	return func(o *Options) {
		o.CredentialsChainVerboseErrors = &verbose
	}
}

// ClientConfig implements the client.ConfigProvider interface
func (s *Session) ClientConfig(serviceName string, cfgs ...*interface{}) client.Config {
	return *s.Config
}

// ClientConfigNoResolveEndpoint implements the client.ConfigNoResolveEndpointProvider interface
func (s *Session) ClientConfigNoResolveEndpoint(cfgs ...*interface{}) client.Config {
	return *s.Config
}

// GetCredentials returns the session's credentials (implements SessionWithCredentials interface)
func (s *Session) GetCredentials() *credentials.Credentials {
	return s.Credentials
}
