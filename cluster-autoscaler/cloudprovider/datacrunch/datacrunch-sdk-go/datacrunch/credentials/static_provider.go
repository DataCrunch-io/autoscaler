package credentials

import (
	"context"
)

// StaticProvider provides static credentials
type StaticProvider struct {
	Value Value
}

// NewStaticCredentials returns a new StaticProvider with static credentials
func NewStaticCredentials(clientID, clientSecret, baseURL string) *Credentials {
	return NewCredentials(&StaticProvider{
		Value: Value{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			BaseURL:      baseURL,
			ProviderName: StaticProviderName,
		},
	})
}

// NewStaticCredentialsFromValue returns a new StaticProvider with the provided Value
func NewStaticCredentialsFromValue(creds Value) *Credentials {
	return NewCredentials(&StaticProvider{
		Value: creds,
	})
}

// Retrieve returns the static credentials
func (s *StaticProvider) Retrieve() (Value, error) {
	if !s.Value.HasKeys() {
		return Value{ProviderName: StaticProviderName}, ErrStaticCredentialsEmpty
	}

	return s.Value, nil
}

// RetrieveWithContext returns the static credentials with context support
func (s *StaticProvider) RetrieveWithContext(ctx context.Context) (Value, error) {
	return s.Retrieve()
}

// IsExpired returns false for static credentials (they don't expire unless explicitly set)
func (s *StaticProvider) IsExpired() bool {
	return s.Value.IsExpired()
}
