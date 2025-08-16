package defaults

import (
	"bytes"
	"fmt"
	"io"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/logger"
)

func Handlers() request.Handlers {
	var handlers request.Handlers

	// Add default handlers for authentication
	handlers.Validate.PushBackNamed(request.NamedHandler{
		Name: "core.ValidateCredentialsHandler",
		Fn:   ValidateCredentialsHandler,
	})

	handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "core.OAuth2AuthHandler",
		Fn:   OAuth2AuthHandler,
	})

	// Add default error handling for ALL protocols - runs FIRST in unmarshal chain
	handlers.Unmarshal.PushFront(request.NamedHandler{
		Name: "core.DefaultErrorHandler",
		Fn:   DefaultErrorHandler,
	})

	return handlers
}

// CredChain returns the default credential chain for DataCrunch
func CredChain() *credentials.Credentials {
	return credentials.NewChainCredentials(CredProviders())
}

// CredProviders returns the default credential providers in order of precedence
func CredProviders() []credentials.Provider {
	return []credentials.Provider{
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{Filename: "", Profile: ""},
	}
}

// ValidateCredentialsHandler validates that credentials are available
func ValidateCredentialsHandler(r *request.Request) {
	if r.Config.Credentials == nil {
		r.Error = credentials.ErrNoValidProvidersFoundInChain
	}
}

// OAuth2AuthHandler adds OAuth2 authentication to requests using credential chain
func OAuth2AuthHandler(r *request.Request) {
	// Get credentials from the request's session
	var creds *credentials.Credentials
	var err error

	// Try to extract credentials from different sources
	if sessionCreds := r.Config.Credentials; sessionCreds != nil {
		creds = sessionCreds
	} else {
		r.Error = dcerr.New("InvalidCredentialType", "no valid credentials found in session or config", nil)
		return
	}

	// Create OAuth2Credentials wrapper for token management
	oauth2Creds := credentials.NewOAuth2CredentialsFromProvider(creds)

	// Get a valid access token
	token, err := oauth2Creds.GetToken(r.Context())
	if err != nil {
		logger.Error("Failed to get OAuth2 token: %v", err)
		r.Error = err
		return
	}

	// Add the Authorization header
	r.HTTPRequest.Header.Set("Authorization", "Bearer "+token)
}

// DefaultErrorHandler handles HTTP error responses for ALL protocols
// This runs FIRST in the unmarshal chain, before protocol-specific unmarshaling
// When this handler sets r.Error, the request processing stops and doesn't continue to other unmarshal handlers
func DefaultErrorHandler(r *request.Request) {
	logger.Debug("DefaultErrorHandler: checking response status code %d", r.HTTPResponse.StatusCode)

	// Only handle non-success status codes
	if r.HTTPResponse.StatusCode >= 200 && r.HTTPResponse.StatusCode < 300 {
		logger.Debug("DefaultErrorHandler: success status code, skipping error handling")
		return // Continue to next handler (protocol-specific unmarshaling)
	}

	logger.Debug("DefaultErrorHandler: handling error response with status %d", r.HTTPResponse.StatusCode)

	// Read the error response body
	var errorBody string
	if r.HTTPResponse.Body != nil {
		body, err := io.ReadAll(r.HTTPResponse.Body)
		if err != nil {
			logger.Debug("DefaultErrorHandler: failed to read error response body: %v", err)
			r.Error = fmt.Errorf("status code: %d, failed to read error response body: %s", r.HTTPResponse.StatusCode, err)
			return // Stop processing - error is set
		}
		errorBody = string(body)
		logger.Debug("DefaultErrorHandler: error response body: %s", errorBody)

		// Close the original body
		if err := r.HTTPResponse.Body.Close(); err != nil {
			logger.Debug("DefaultErrorHandler: error closing response body: %v", err)
		}

		// Replace the closed body with a new reader containing the same data
		// This allows other handlers to still read the body if needed
		r.HTTPResponse.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Collect request info for debugging
	requestInfo := &dcerr.RequestInfo{
		RequestURL:     r.HTTPRequest.URL.String(),
		RequestHeaders: &r.HTTPRequest.Header,
		RequestBody:    nil, // Request body is usually consumed during Build phase
	}

	// Create structured HTTP error
	r.Error = dcerr.NewHTTPError(r.HTTPResponse.StatusCode, errorBody, requestInfo)
	logger.Debug("DefaultErrorHandler: created HTTPError: %v", r.Error)
	// When r.Error is set, the request processing stops and doesn't continue to other handlers
	return
}
