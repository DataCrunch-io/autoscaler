package credentials

import (
	"context"
	"fmt"
	"strings"
)

// ChainProvider provides credentials from a chain of providers
// It will try each provider in order until one succeeds
type ChainProvider struct {
	Providers     []Provider
	curr          Provider
	VerboseErrors bool
}

// NewChainCredentials returns a new Credentials with ChainProvider
func NewChainCredentials(providers []Provider) *Credentials {
	return NewCredentials(&ChainProvider{
		Providers: providers,
	})
}

// NewChainCredentialsVerbose returns a new Credentials with ChainProvider that includes verbose errors
func NewChainCredentialsVerbose(providers []Provider, verboseErrors bool) *Credentials {
	return NewCredentials(&ChainProvider{
		Providers:     providers,
		VerboseErrors: verboseErrors,
	})
}

// Retrieve retrieves credentials from the chain of providers
func (c *ChainProvider) Retrieve() (Value, error) {
	return c.RetrieveWithContext(context.Background())
}

// RetrieveWithContext retrieves credentials with context support
func (c *ChainProvider) RetrieveWithContext(ctx context.Context) (Value, error) {
	var errs []error

	for _, p := range c.Providers {
		var creds Value
		var err error

		if pc, ok := p.(ProviderWithContext); ok {
			creds, err = pc.RetrieveWithContext(ctx)
		} else {
			creds, err = p.Retrieve()
		}

		if err == nil {
			c.curr = p
			return creds, nil
		}

		errs = append(errs, err)
	}

	c.curr = nil

	var err error
	if c.VerboseErrors {
		err = fmt.Errorf("credential chain failure: %s", c.formatErrors(errs))
	} else {
		err = ErrNoValidProvidersFoundInChain
	}

	return Value{ProviderName: ChainProviderName}, err
}

// IsExpired checks if the current provider's credentials are expired
func (c *ChainProvider) IsExpired() bool {
	if c.curr == nil {
		return true
	}
	return c.curr.IsExpired()
}

// formatErrors formats a slice of errors into a readable string
func (c *ChainProvider) formatErrors(errs []error) string {
	var errStrings []string
	for i, err := range errs {
		errStrings = append(errStrings, fmt.Sprintf("provider %d: %v", i+1, err))
	}
	return strings.Join(errStrings, ", ")
}
