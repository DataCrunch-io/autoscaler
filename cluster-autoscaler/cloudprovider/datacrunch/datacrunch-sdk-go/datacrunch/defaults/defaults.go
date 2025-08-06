package defaults

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/auth"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/client"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
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
		Fn:   auth.OAuth2AuthHandler,
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

// CredProvidersWithEndpoints returns credential providers including endpoint-based providers
// This is useful when you want to include endpoint credentials in your chain
func CredProvidersWithEndpoints(endpointConfigs []EndpointCredentialConfig) []credentials.Provider {
	providers := []credentials.Provider{
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{Filename: "", Profile: ""},
	}

	// Add endpoint providers as configured
	for _, config := range endpointConfigs {
		if config.Endpoint != "" {
			providers = append(providers, credentials.NewEndpointProvider(
				config.ClientConfig,
				config.Endpoint,
				config.Options...,
			))
		}
	}

	return providers
}

// EndpointCredentialConfig holds configuration for endpoint-based credential providers
type EndpointCredentialConfig struct {
	ClientConfig client.Config
	Endpoint     string
	Options      []credentials.EndpointProviderOptions
}

// NewChainWithEndpoints creates a credential chain that includes endpoint providers
func NewChainWithEndpoints(endpointConfigs []EndpointCredentialConfig, verbose bool) *credentials.Credentials {
	providers := CredProvidersWithEndpoints(endpointConfigs)
	if verbose {
		return credentials.NewChainCredentialsVerbose(providers, true)
	}
	return credentials.NewChainCredentials(providers)
}

// ValidateCredentialsHandler validates that credentials are available
func ValidateCredentialsHandler(r *request.Request) {
	if r.Config == nil {
		r.Error = credentials.ErrNoValidProvidersFoundInChain
	}
}
