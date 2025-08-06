package datacrunch

import (
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/session"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instanceavailability"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/locations"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/sshkeys"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/volumes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/volumetypes"
)

// ClientConfig holds optional configuration for the DataCrunch SDK
type ClientConfig struct {
	// Optional timeout override
	Timeout *time.Duration

	// Optional base URL override
	BaseURL *string

	// Optional credentials (for static credential use cases)
	Credentials *credentials.Credentials

	// Optional retry configuration
	MaxRetries *int
	Retryer    interface{}
}

// Client represents a convenience wrapper that bundles all DataCrunch services
type Client struct {
	// Session used by all services
	Session *session.Session

	// Service clients - all use the same session with credential chain
	Instance             *instance.Instance
	InstanceAvailability *instanceavailability.InstanceAvailability
	InstanceTypes        *instancetypes.InstanceTypes
	Locations            *locations.Locations
	SSHKeys              *sshkeys.SSHKey
	StartScripts         *startscripts.StartScripts
	Volumes              *volumes.Volumes
	VolumeTypes          *volumetypes.VolumeTypes
}

// Session represents a shared configuration and state for service clients
type Session = *session.Session

// Option is a functional option for configuring the DataCrunch client
type Option func(*ClientConfig)

// WithBaseURL sets the base URL for the API
func WithBaseURL(baseURL string) Option {
	return func(c *ClientConfig) {
		c.BaseURL = &baseURL
	}
}

// WithCredentials sets static OAuth2 client credentials
func WithCredentials(clientID, clientSecret string) Option {
	return func(c *ClientConfig) {
		c.Credentials = credentials.NewStaticCredentials(clientID, clientSecret, "https://api.datacrunch.io")
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *ClientConfig) {
		c.Timeout = &timeout
	}
}

// WithCredentialsProvider sets custom credentials provider
func WithCredentialsProvider(creds *credentials.Credentials) Option {
	return func(c *ClientConfig) {
		c.Credentials = creds
	}
}

// New creates a new DataCrunch SDK client with optional configuration
//
// This is a convenience method that creates a session and all services.
// For more control, create a session directly and individual services as needed.
//
// Example:
//
//	client := datacrunch.New() // Uses credential chain automatically
//	client := datacrunch.New(datacrunch.WithCredentials("id", "secret"))
func New(options ...Option) *Client {
	config := &ClientConfig{}

	// Apply all options
	for _, option := range options {
		option(config)
	}

	// Build session options
	var sessionOpts []func(*session.Options)

	if config.Timeout != nil {
		sessionOpts = append(sessionOpts, session.WithTimeout(*config.Timeout))
	}

	if config.BaseURL != nil {
		sessionOpts = append(sessionOpts, session.WithBaseURL(*config.BaseURL))
	}

	if config.Credentials != nil {
		sessionOpts = append(sessionOpts, session.WithCredentialsProvider(config.Credentials))
	}

	if config.MaxRetries != nil {
		sessionOpts = append(sessionOpts, session.WithMaxRetries(*config.MaxRetries))
	}

	if config.Retryer != nil {
		sessionOpts = append(sessionOpts, session.WithRetryer(config.Retryer))
	}

	// Create session (uses credential chain by default)
	sess := session.New(sessionOpts...)

	return &Client{
		Session:              sess,
		Instance:             instance.New(sess),
		InstanceAvailability: instanceavailability.New(sess),
		InstanceTypes:        instancetypes.New(sess),
		Locations:            locations.New(sess),
		SSHKeys:              sshkeys.New(sess),
		StartScripts:         startscripts.New(sess),
		Volumes:              volumes.New(sess),
		VolumeTypes:          volumetypes.New(sess),
	}
}

// NewFromEnv creates a new DataCrunch SDK client using environment variables
//
// This is a convenience method equivalent to New() since the default credential
// chain automatically tries environment variables first.
//
// Supported environment variables:
// - DATACRUNCH_CLIENT_ID (OAuth2 client ID)
// - DATACRUNCH_CLIENT_SECRET (OAuth2 client secret)
// - DATACRUNCH_BASE_URL (API base URL, optional)
// - DATACRUNCH_TIMEOUT (request timeout, optional)
func NewFromEnv(options ...Option) *Client {
	config := &ClientConfig{}

	// Apply additional options (these can override defaults)
	for _, option := range options {
		option(config)
	}

	// Build session options, start with env credentials
	sessionOpts := []func(*session.Options){
		session.WithCredentialsProvider(credentials.NewEnvCredentials()),
	}

	if config.Timeout != nil {
		sessionOpts = append(sessionOpts, session.WithTimeout(*config.Timeout))
	}

	if config.BaseURL != nil {
		sessionOpts = append(sessionOpts, session.WithBaseURL(*config.BaseURL))
	}

	// Create session with environment credentials
	sess := session.New(sessionOpts...)

	return &Client{
		Session:              sess,
		Instance:             instance.New(sess),
		InstanceAvailability: instanceavailability.New(sess),
		InstanceTypes:        instancetypes.New(sess),
		Locations:            locations.New(sess),
		SSHKeys:              sshkeys.New(sess),
		StartScripts:         startscripts.New(sess),
		Volumes:              volumes.New(sess),
		VolumeTypes:          volumetypes.New(sess),
	}
}

// NewWithCredentials creates a new DataCrunch SDK client with static credentials
//
// This is a convenience method for testing and development.
// For production, prefer using New() with credential chain.
func NewWithCredentials(clientID, clientSecret string, baseURL ...string) *Client {
	var url string
	if len(baseURL) > 0 {
		url = baseURL[0]
	} else {
		url = "https://api.datacrunch.io"
	}

	creds := credentials.NewStaticCredentials(clientID, clientSecret, url)
	return New(WithCredentialsProvider(creds))
}

// NewSession creates a new session with functional options
func NewSession(options ...func(*session.Options)) Session {
	return session.New(options...)
}

// NewSessionFromEnv creates a new session using environment variables
func NewSessionFromEnv(options ...func(*session.Options)) Session {
	return session.NewFromEnv(options...)
}

// NewWithSession creates a new DataCrunch SDK client with an existing session
//
// This is the recommended way when you need to share a session across
// multiple clients or when you need fine control over session configuration.
//
// Example:
//
//	sess := session.New() // Uses credential chain
//	client := datacrunch.NewWithSession(sess)
func NewWithSession(sess Session) *Client {
	return &Client{
		Session:              sess,
		Instance:             instance.New(sess),
		InstanceAvailability: instanceavailability.New(sess),
		InstanceTypes:        instancetypes.New(sess),
		Locations:            locations.New(sess),
		SSHKeys:              sshkeys.New(sess),
		StartScripts:         startscripts.New(sess),
		Volumes:              volumes.New(sess),
		VolumeTypes:          volumetypes.New(sess),
	}
}

// Legacy support - these methods maintain backward compatibility

// WithRetryConfig configures retry behavior (supported again)
func WithRetryConfig(maxRetries int, retryDelay, maxRetryDelay time.Duration) Option {
	return func(c *ClientConfig) {
		c.MaxRetries = &maxRetries
		// Custom retry delays require custom retryer - use WithRetryer for that
	}
}

// WithRetryer sets a custom retryer implementation (supported again)
func WithRetryer(retryer interface{}) Option {
	return func(c *ClientConfig) {
		c.Retryer = retryer
	}
}

// WithNoRetries disables retry functionality entirely
func WithNoRetries() Option {
	return WithRetryConfig(0, 0, 0)
}
