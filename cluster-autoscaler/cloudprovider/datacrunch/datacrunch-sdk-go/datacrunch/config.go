package datacrunch

import (
	"log/slog"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
)

// Config holds configuration for the DataCrunch SDK
type Config struct {
	// API configuration
	BaseURL *string
	Timeout *time.Duration

	// Credential configuration
	Credentials *credentials.Credentials

	// Retry configuration
	MaxRetries *int
	Retryer    interface{}

	// Logging configuration default to false
	Debug  bool
	Logger *slog.Logger
}

// Option is a functional option for configuring the DataCrunch client
type Option func(*Config)

// Copy creates a copy of the Config
func (c *Config) Copy(cfgs ...*Config) *Config {
	newConfig := &Config{
		BaseURL:     c.BaseURL,
		Timeout:     c.Timeout,
		Credentials: c.Credentials,
		MaxRetries:  c.MaxRetries,
		Retryer:     c.Retryer,
		Debug:       c.Debug,
		Logger:      c.Logger,
	}

	for _, cfg := range cfgs {
		if cfg.BaseURL != nil {
			newConfig.BaseURL = cfg.BaseURL
		}
		if cfg.Timeout != nil {
			newConfig.Timeout = cfg.Timeout
		}
		if cfg.Credentials != nil {
			newConfig.Credentials = cfg.Credentials
		}
		if cfg.MaxRetries != nil {
			newConfig.MaxRetries = cfg.MaxRetries
		}
		if cfg.Retryer != nil {
			newConfig.Retryer = cfg.Retryer
		}
		if cfg.Logger != nil {
			newConfig.Logger = cfg.Logger
		}
	}

	return newConfig
}

// New creates a new Config
func NewConfig(options ...Option) *Config {
	cfg := &Config{}
	for _, option := range options {
		option(cfg)
	}
	return cfg
}

// WithBaseURL sets the base URL for the API
func WithBaseURL(baseURL string) Option {
	return func(c *Config) {
		c.BaseURL = &baseURL
	}
}

// WithCredentials sets static OAuth2 client credentials
func WithCredentials(clientID, clientSecret string) Option {
	return func(c *Config) {
		// Note: We need to handle baseURL properly here
		baseURL := "https://api.datacrunch.io/v1"
		if c.BaseURL != nil {
			baseURL = *c.BaseURL
		}
		c.Credentials = credentials.NewStaticCredentials(clientID, clientSecret, baseURL)
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = &timeout
	}
}

// WithCredentialsProvider sets custom credentials provider
func WithCredentialsProvider(creds *credentials.Credentials) Option {
	return func(c *Config) {
		c.Credentials = creds
	}
}

// WithRetryConfig configures retry behavior
func WithRetryConfig(maxRetries int, retryDelay, maxRetryDelay time.Duration) Option {
	return func(c *Config) {
		c.MaxRetries = &maxRetries
		// Custom retry delays require custom retryer - use WithRetryer for that
	}
}

// WithRetryer sets a custom retryer implementation
func WithRetryer(retryer interface{}) Option {
	return func(c *Config) {
		c.Retryer = retryer
	}
}

// WithNoRetries disables retry functionality entirely
func WithNoRetries() Option {
	return WithRetryConfig(0, 0, 0)
}

// WithDebug enables or disables debug logging
func WithDebug(debug bool) Option {
	return func(c *Config) {
		c.Debug = debug
	}
}

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}
