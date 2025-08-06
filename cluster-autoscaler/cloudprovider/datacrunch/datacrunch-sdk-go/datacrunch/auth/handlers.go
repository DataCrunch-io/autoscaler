package auth

import (
	"log"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// OAuth2AuthHandler adds OAuth2 authentication to requests using credential chain
func OAuth2AuthHandler(r *request.Request) {
	log.Printf("Starting OAuth2 authentication with credential chain")

	// Get credentials from the request's session
	var creds *credentials.Credentials
	var err error

	// Try to extract credentials from different sources
	if sessionCreds := getCredentialsFromRequest(r); sessionCreds != nil {
		creds = sessionCreds
		log.Printf("Using session credentials")
	} else {
		// Fallback to creating OAuth2 credentials from config
		log.Printf("Falling back to config-based credentials")
		if err = r.Error; err != nil {
			return
		}

		// Try to extract from config
		oauth2Creds := getOAuth2CredentialsFromConfig(r)
		if oauth2Creds == nil {
			r.Error = dcerr.New("InvalidCredentialType", "no valid credentials found in session or config", nil)
			return
		}

		// Get token from OAuth2 wrapper
		token, err := oauth2Creds.GetToken(r.Context())
		if err != nil {
			log.Printf("Failed to get token from OAuth2 credentials: %v", err)
			r.Error = err
			return
		}

		// Add the Authorization header
		r.HTTPRequest.Header.Set("Authorization", "Bearer "+token)
		log.Printf("Added Authorization header from OAuth2 credentials")
		return
	}

	// Get credential values from chain
	credValue, err := creds.GetWithContext(r.Context())
	if err != nil {
		log.Printf("Failed to get credentials: %v", err)
		r.Error = err
		return
	}

	log.Printf("Got credentials from provider: %s", credValue.ProviderName)

	// Create OAuth2Credentials wrapper for token management
	oauth2Creds := credentials.NewOAuth2CredentialsFromProvider(creds)

	// Get a valid access token
	token, err := oauth2Creds.GetToken(r.Context())
	if err != nil {
		log.Printf("Failed to get OAuth2 token: %v", err)
		r.Error = err
		return
	}

	log.Printf("Got valid token, length: %d", len(token))

	// Add the Authorization header
	r.HTTPRequest.Header.Set("Authorization", "Bearer "+token)
	log.Printf("Added Authorization header")
}

// SessionWithCredentials defines an interface for session-like objects with credentials
type SessionWithCredentials interface {
	GetCredentials() *credentials.Credentials
}

// getCredentialsFromRequest extracts credentials from request config
func getCredentialsFromRequest(r *request.Request) *credentials.Credentials {
	// Try to extract credentials from different config types
	switch cfg := r.Config.(type) {
	case SessionWithCredentials:
		return cfg.GetCredentials()
	default:
		return nil
	}
}

// getOAuth2CredentialsFromConfig creates OAuth2Credentials from config for backwards compatibility
func getOAuth2CredentialsFromConfig(r *request.Request) *credentials.OAuth2Credentials {
	switch cfg := r.Config.(type) {
	case *credentials.OAuth2Credentials:
		return cfg
	case map[string]interface{}:
		// Handle interface{} config - extract credentials info
		if clientID, ok := cfg["ClientID"].(string); ok {
			if clientSecret, ok := cfg["ClientSecret"].(string); ok {
				if baseURL, ok := cfg["BaseURL"].(string); ok {
					return credentials.NewOAuth2Credentials(clientID, clientSecret, baseURL)
				}
			}
		}
	}
	return nil
}

// DebugHandler logs request and response details
func DebugHandler(r *request.Request) {
	if r.HTTPResponse != nil {
		log.Printf("Response status: %d", r.HTTPResponse.StatusCode)
		log.Printf("Response headers: %v", r.HTTPResponse.Header)
	}
	if r.Error != nil {
		log.Printf("Request error: %v", r.Error)
	}
}
