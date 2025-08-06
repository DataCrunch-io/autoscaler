package credentials

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrNoValidProvidersFoundInChain = errors.New("no valid providers found in credential chain")
	ErrStaticCredentialsEmpty       = errors.New("static credentials are empty")
	ErrAccessKeyIDNotFound          = errors.New("DATACRUNCH_CLIENT_ID or DATACRUNCH_ACCESS_KEY_ID not found in environment")
	ErrSecretAccessKeyNotFound      = errors.New("DATACRUNCH_CLIENT_SECRET or DATACRUNCH_SECRET_ACCESS_KEY not found in environment")
)

// ProviderName identifies the name of the credential provider
type ProviderName string

const (
	EnvProviderName               ProviderName = "EnvProvider"
	StaticProviderName            ProviderName = "StaticProvider"
	SharedCredentialsProviderName ProviderName = "SharedCredentialsProvider"
	ChainProviderName             ProviderName = "ChainProvider"
)

// Value contains the actual credential values
type Value struct {
	// ClientID is the OAuth2 client ID (equivalent to AWS Access Key ID)
	ClientID string

	// ClientSecret is the OAuth2 client secret (equivalent to AWS Secret Access Key)
	ClientSecret string

	// AccessToken is the current OAuth2 access token (equivalent to AWS Session Token)
	AccessToken string

	// RefreshToken is the OAuth2 refresh token (for token renewal)
	RefreshToken string

	// Expiry is when the access token expires
	Expiry time.Time

	// ProviderName identifies which provider was used to retrieve these credentials
	ProviderName ProviderName

	// BaseURL is the API base URL for token requests
	BaseURL string
}

// HasKeys returns true if the credentials have both ClientID and ClientSecret
func (v Value) HasKeys() bool {
	return v.ClientID != "" && v.ClientSecret != ""
}

// IsExpired returns true if the access token has expired
func (v Value) IsExpired() bool {
	return !v.Expiry.IsZero() && time.Now().After(v.Expiry)
}

// Provider defines the interface for credential providers
type Provider interface {
	// Retrieve returns the credentials or an error if they could not be retrieved
	Retrieve() (Value, error)

	// IsExpired returns true if the cached credentials are expired
	IsExpired() bool
}

// ProviderWithContext extends Provider to support context
type ProviderWithContext interface {
	Provider
	// RetrieveWithContext returns the credentials with context support
	RetrieveWithContext(ctx context.Context) (Value, error)
}

// Credentials manages credential providers and caches credential values
type Credentials struct {
	creds    Value
	provider Provider
	mu       sync.RWMutex
}

// NewCredentials returns a new Credentials with the given provider
func NewCredentials(provider Provider) *Credentials {
	return &Credentials{
		provider: provider,
	}
}

// Get retrieves the credentials value. If credentials have not been retrieved
// yet, or the provider's IsExpired method returns true, the credentials will
// be retrieved from the provider.
func (c *Credentials) Get() (Value, error) {
	return c.GetWithContext(context.Background())
}

// GetWithContext retrieves the credentials value with context support
func (c *Credentials) GetWithContext(ctx context.Context) (Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if current credentials are still valid
	if c.creds.HasKeys() && !c.isExpiredLocked(c.creds) {
		return c.creds, nil
	}

	var creds Value
	var err error

	// Try context-aware provider first
	if p, ok := c.provider.(ProviderWithContext); ok {
		creds, err = p.RetrieveWithContext(ctx)
	} else {
		creds, err = c.provider.Retrieve()
	}

	if err == nil {
		c.creds = creds
	}

	return creds, err
}

// IsExpired returns true if the credentials are expired
func (c *Credentials) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isExpiredLocked(c.creds)
}

// isExpiredLocked returns true if the credentials are expired (assumes lock is held)
func (c *Credentials) isExpiredLocked(creds Value) bool {
	return !creds.HasKeys() || c.provider.IsExpired() || creds.IsExpired()
}

// Expire marks the credentials as expired, forcing the next Get() call to retrieve new credentials
func (c *Credentials) Expire() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.creds = Value{}
}

// GetClientCredentials returns just the client ID and secret for basic OAuth2 flows
func (c *Credentials) GetClientCredentials() (clientID, clientSecret string, err error) {
	creds, err := c.Get()
	if err != nil {
		return "", "", err
	}

	if !creds.HasKeys() {
		return "", "", ErrStaticCredentialsEmpty
	}

	return creds.ClientID, creds.ClientSecret, nil
}

// GetAccessToken returns a valid access token, fetching/refreshing if necessary
func (c *Credentials) GetAccessToken(ctx context.Context) (string, error) {
	creds, err := c.GetWithContext(ctx)
	if err != nil {
		return "", err
	}

	// If we have an unexpired access token, return it
	if creds.AccessToken != "" && !creds.IsExpired() {
		return creds.AccessToken, nil
	}

	// If we need a new token but have client credentials, let the OAuth2 system handle it
	// This will be handled by the OAuth2Credentials wrapper
	return creds.AccessToken, nil
}
