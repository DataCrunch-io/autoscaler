package credentials

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
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

	if clientID == "" {
		return Value{ProviderName: EnvProviderName}, ErrAccessKeyIDNotFound
	}

	if clientSecret == "" {
		return Value{ProviderName: EnvProviderName}, ErrSecretAccessKeyNotFound
	}

	clientID = strings.TrimRight(e.decodeIfBase64(clientID), "\n")
	clientSecret = strings.TrimRight(e.decodeIfBase64(clientSecret), "\n")

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

// decodeIfBase64 detects and decodes base64 encoded values
// This handles Kubernetes secrets that are automatically base64 encoded
func (e *EnvProvider) decodeIfBase64(value string) string {
	if value == "" {
		return value
	}

	// Simple base64 detection: check if it looks like base64 and is longer than original after decoding would be
	if strings.Contains(value, "=") || (len(value)%4 == 0 && len(value) > 20) {
		if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
			decodedStr := strings.TrimSpace(string(decoded))
			// Only use decoded value if it's printable and shorter (indicating it was actually encoded)
			if len(decodedStr) > 0 && len(decodedStr) < len(value) && isPrintable(decodedStr) {
				return decodedStr
			}
		}
	}

	return value
}

// isPrintable checks if string contains only printable characters
func isPrintable(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
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
