package credentials

import (
	"context"
	"os"
)

// EnvProvider retrieves credentials from environment variables
type EnvProvider struct {
	retrieved bool
}

// NewEnvCredentials returns a new Credentials with the EnvProvider
func NewEnvCredentials() *Credentials {
	return NewCredentials(&EnvProvider{})
}

// Retrieve retrieves the credentials from environment variables
func (e *EnvProvider) Retrieve() (Value, error) {
	e.retrieved = false

	// Try DataCrunch-specific environment variables first
	clientID := os.Getenv("DATACRUNCH_CLIENT_ID")
	clientSecret := os.Getenv("DATACRUNCH_CLIENT_SECRET")
	baseURL := os.Getenv("DATACRUNCH_BASE_URL")

	// Fallback to AWS-style naming for compatibility
	if clientID == "" {
		clientID = os.Getenv("DATACRUNCH_ACCESS_KEY_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("DATACRUNCH_SECRET_ACCESS_KEY")
	}

	// Set default base URL if not provided
	if baseURL == "" {
		baseURL = "https://api.datacrunch.io"
	}

	if clientID == "" {
		return Value{ProviderName: EnvProviderName}, ErrAccessKeyIDNotFound
	}

	if clientSecret == "" {
		return Value{ProviderName: EnvProviderName}, ErrSecretAccessKeyNotFound
	}

	e.retrieved = true
	return Value{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		ProviderName: EnvProviderName,
		// Note: AccessToken and RefreshToken are not typically stored in env vars
		// They will be obtained through OAuth2 flow
	}, nil
}

// RetrieveWithContext retrieves credentials with context support
func (e *EnvProvider) RetrieveWithContext(ctx context.Context) (Value, error) {
	return e.Retrieve()
}

// IsExpired returns false since environment credentials don't expire
// (though the OAuth2 tokens they generate might)
func (e *EnvProvider) IsExpired() bool {
	return !e.retrieved
}
