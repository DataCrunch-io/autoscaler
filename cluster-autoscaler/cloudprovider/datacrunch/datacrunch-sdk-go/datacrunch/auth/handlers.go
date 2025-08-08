package auth

import (
	"log"
	"reflect"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// OAuth2AuthHandler adds OAuth2 authentication to requests using credential chain
func OAuth2AuthHandler(r *request.Request) {
	// Get credentials from the request's session
	var creds *credentials.Credentials
	var err error

	// Try to extract credentials from different sources
	if sessionCreds := getCredentialsFromRequest(r); sessionCreds != nil {
		creds = sessionCreds
	} else {
		// Fallback to creating OAuth2 credentials from config
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
			r.Error = err
			return
		}

		// Add the Authorization header
		r.HTTPRequest.Header.Set("Authorization", "Bearer "+token)
		return
	}

	// // Get credential values from chain
	// _, err = creds.GetWithContext(r.Context())
	// if err != nil {
	// 	r.Error = err
	// 	return
	// }

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
		// Try to extract from config.Config struct using reflection
		if cfgValue := reflect.ValueOf(r.Config); cfgValue.IsValid() {
			// Handle both struct and pointer to struct
			if cfgValue.Kind() == reflect.Ptr && !cfgValue.IsNil() {
				cfgValue = cfgValue.Elem()
			}
			if cfgValue.Kind() == reflect.Struct {
				// Check for Credentials field in config
				if field := cfgValue.FieldByName("Credentials"); field.IsValid() && !field.IsNil() {
					if creds, ok := field.Interface().(*credentials.Credentials); ok {
						return creds
					}
				}
			}
		}
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
