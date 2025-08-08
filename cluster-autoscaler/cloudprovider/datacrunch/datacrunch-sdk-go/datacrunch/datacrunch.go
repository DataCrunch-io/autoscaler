package datacrunch

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/config"
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

// Config represents the configuration for the DataCrunch SDK
type Config = config.Config

// Option is a functional option for configuring the DataCrunch client
type Option = config.Option

// Session represents a shared configuration and state for service clients
type Session = *session.Session

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
	cfg := &Config{}

	// Apply all options
	for _, option := range options {
		option(cfg)
	}

	// Build session options
	var sessionOpts []func(*session.Options)

	if cfg.Timeout != nil {
		sessionOpts = append(sessionOpts, session.WithTimeout(*cfg.Timeout))
	}

	if cfg.BaseURL != nil {
		sessionOpts = append(sessionOpts, session.WithBaseURL(*cfg.BaseURL))
	}

	if cfg.Credentials != nil {
		sessionOpts = append(sessionOpts, session.WithCredentialsProvider(cfg.Credentials))
	}

	if cfg.MaxRetries != nil {
		sessionOpts = append(sessionOpts, session.WithMaxRetries(*cfg.MaxRetries))
	}

	if cfg.Retryer != nil {
		sessionOpts = append(sessionOpts, session.WithRetryer(cfg.Retryer))
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
	cfg := &Config{}

	// Apply additional options (these can override defaults)
	for _, option := range options {
		option(cfg)
	}

	// Build session options, start with env credentials
	sessionOpts := []func(*session.Options){
		session.WithCredentialsProvider(credentials.NewEnvCredentials()),
	}

	if cfg.Timeout != nil {
		sessionOpts = append(sessionOpts, session.WithTimeout(*cfg.Timeout))
	}

	if cfg.BaseURL != nil {
		sessionOpts = append(sessionOpts, session.WithBaseURL(*cfg.BaseURL))
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

// Re-export config options for backward compatibility

// WithBaseURL sets the base URL for the API
var WithBaseURL = config.WithBaseURL

// WithCredentials sets static OAuth2 client credentials
var WithCredentials = config.WithCredentials

// WithTimeout sets the HTTP client timeout
var WithTimeout = config.WithTimeout

// WithCredentialsProvider sets custom credentials provider
var WithCredentialsProvider = config.WithCredentialsProvider

// WithRetryConfig configures retry behavior
var WithRetryConfig = config.WithRetryConfig

// WithRetryer sets a custom retryer implementation
var WithRetryer = config.WithRetryer

// WithNoRetries disables retry functionality entirely
var WithNoRetries = config.WithNoRetries
