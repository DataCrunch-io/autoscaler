package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// OAuth2Credentials represents OAuth2 client credentials with token caching
// This is now a wrapper around the new credential system
type OAuth2Credentials struct {
	creds *Credentials

	// Cached OAuth2 state
	AccessToken  string
	RefreshToken string
	Expiry       time.Time

	mu sync.Mutex
}

// TokenResponse matches the OAuth2 token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// NewOAuth2Credentials creates a new OAuth2Credentials instance
func NewOAuth2Credentials(clientID, clientSecret, baseURL string) *OAuth2Credentials {
	creds := NewStaticCredentials(clientID, clientSecret, baseURL)
	return &OAuth2Credentials{
		creds: creds,
	}
}

// NewOAuth2CredentialsFromProvider creates OAuth2Credentials with a credential provider
func NewOAuth2CredentialsFromProvider(creds *Credentials) *OAuth2Credentials {
	return &OAuth2Credentials{
		creds: creds,
	}
}

// GetToken returns a valid access token, refreshing or fetching as needed
func (c *OAuth2Credentials) GetToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If token is still valid, return it
	if c.AccessToken != "" && time.Now().Before(c.Expiry.Add(-time.Minute)) {
		log.Printf("Using cached token, expires at: %v", c.Expiry)
		return c.AccessToken, nil
	}

	// If we have a refresh token, try to refresh
	if c.RefreshToken != "" {
		log.Printf("Attempting to refresh token")
		var err error
		if err = c.refreshWithRefreshToken(ctx); err == nil {
			return c.AccessToken, nil
		}
		log.Printf("Refresh failed, falling back to client credentials: %v", err)
		// If refresh fails, fall back to client credentials
	}

	// Otherwise, get a new token using client credentials
	log.Printf("Fetching new token using client credentials")
	if err := c.fetchWithClientCredentials(ctx); err != nil {
		return "", err
	}
	return c.AccessToken, nil
}

// GetClientCredentials returns the client credentials for basic OAuth2 flows
func (c *OAuth2Credentials) GetClientCredentials() (clientID, clientSecret string, err error) {
	return c.creds.GetClientCredentials()
}

// GetBaseURL returns the base URL from credentials
func (c *OAuth2Credentials) GetBaseURL() (string, error) {
	credValue, err := c.creds.Get()
	if err != nil {
		return "", err
	}
	return credValue.BaseURL, nil
}

// fetchWithClientCredentials gets a new token using client credentials grant
func (c *OAuth2Credentials) fetchWithClientCredentials(ctx context.Context) error {
	clientID, clientSecret, err := c.creds.GetClientCredentials()
	if err != nil {
		return err
	}

	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     clientID,
		"client_secret": clientSecret,
	}
	return c.doTokenRequest(ctx, payload)
}

// refreshWithRefreshToken gets a new token using the refresh token grant
func (c *OAuth2Credentials) refreshWithRefreshToken(ctx context.Context) error {
	if c.RefreshToken == "" {
		return errors.New("no refresh token available")
	}

	clientID, clientSecret, err := c.creds.GetClientCredentials()
	if err != nil {
		return err
	}

	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": c.RefreshToken,
		"client_id":     clientID,
		"client_secret": clientSecret,
	}
	return c.doTokenRequest(ctx, payload)
}

// doTokenRequest sends the token request and updates the credential fields
func (c *OAuth2Credentials) doTokenRequest(ctx context.Context, payload map[string]string) error {
	baseURL, err := c.GetBaseURL()
	if err != nil {
		return err
	}

	body, _ := json.Marshal(payload)
	endpoint := baseURL + "/v1/oauth2/token"
	log.Printf("Sending token request to %s", endpoint)
	log.Printf("Request payload: %s", string(body))

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// Read and log the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Printf("Token response status: %d", resp.StatusCode)
	log.Printf("Token response body: %s", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return err
	}

	c.AccessToken = tokenResp.AccessToken
	c.RefreshToken = tokenResp.RefreshToken
	c.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	log.Printf("Token obtained, expires in %d seconds", tokenResp.ExpiresIn)

	return nil
}
