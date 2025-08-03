package auth

import (
	"log"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// OAuth2AuthHandler adds OAuth2 authentication to requests
func OAuth2AuthHandler(r *request.Request) {
	log.Printf("Starting OAuth2 authentication")

	// Try to get credentials from various config types
	var creds *credentials.OAuth2Credentials

	switch cfg := r.Config.(type) {
	case *credentials.OAuth2Credentials:
		creds = cfg
	case map[string]interface{}:
		// Handle interface{} config - extract credentials info
		if clientID, ok := cfg["ClientID"].(string); ok {
			if clientSecret, ok := cfg["ClientSecret"].(string); ok {
				if baseURL, ok := cfg["BaseURL"].(string); ok {
					creds = credentials.NewOAuth2Credentials(clientID, clientSecret, baseURL)
				}
			}
		}
	default:
		log.Printf("Invalid credential type: %T", r.Config)
		r.Error = dcerr.New("InvalidCredentialType", "expected OAuth2Credentials or config map", nil)
		return
	}

	if creds == nil {
		r.Error = dcerr.New("InvalidCredentialType", "could not extract OAuth2 credentials", nil)
		return
	}

	log.Printf("Got OAuth2 credentials, client ID: %s", creds.ClientID)

	// Get a valid token
	token, err := creds.GetToken(r.Context())
	if err != nil {
		log.Printf("Failed to get token: %v", err)
		r.Error = err
		return
	}
	log.Printf("Got valid token, length: %d", len(token))

	// Add the Authorization header
	r.HTTPRequest.Header.Set("Authorization", "Bearer "+token)
	log.Printf("Added Authorization header")
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
