package datacrunch

import (
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
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
		c.Credentials = credentials.NewStaticCredentials(clientID, clientSecret, *c.BaseURL)
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
