package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

// AuthHandler interface for authentication handlers
type AuthHandler interface {
	AddAuth(req *http.Request) error
	IsValid() bool
	Refresh(ctx context.Context) error
}

// OAuth2Handler handles OAuth2 client credentials authentication
type OAuth2Handler struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	AccessToken  string
	TokenType    string
	ExpiresAt    time.Time
	httpClient   *http.Client
}

// NewOAuth2Handler creates a new OAuth2 authentication handler
func NewOAuth2Handler(clientID, clientSecret, tokenURL string) *OAuth2Handler {
	return &OAuth2Handler{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// AddAuth adds authentication to an HTTP request
func (h *OAuth2Handler) AddAuth(req *http.Request) error {
	if !h.IsValid() {
		if err := h.Refresh(req.Context()); err != nil {
			return fmt.Errorf("failed to refresh token: %v", err)
		}
	}

	if h.AccessToken == "" {
		return fmt.Errorf("no access token available")
	}

	authValue := h.TokenType
	if authValue == "" {
		authValue = "Bearer"
	}
	authValue += " " + h.AccessToken

	req.Header.Set("Authorization", authValue)
	return nil
}

// IsValid checks if the current token is still valid
func (h *OAuth2Handler) IsValid() bool {
	if h.AccessToken == "" {
		return false
	}
	
	// Check if token expires within the next minute (buffer)
	return time.Now().Add(1 * time.Minute).Before(h.ExpiresAt)
}

// Refresh refreshes the OAuth2 token
func (h *OAuth2Handler) Refresh(ctx context.Context) error {
	klog.V(4).Info("Refreshing OAuth2 token for DataCrunch API")

	tokenReq := &TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     h.ClientID,
		ClientSecret: h.ClientSecret,
	}

	tokenResp, err := h.requestToken(ctx, tokenReq)
	if err != nil {
		return fmt.Errorf("token request failed: %v", err)
	}

	h.AccessToken = tokenResp.AccessToken
	h.TokenType = tokenResp.TokenType
	if tokenResp.ExpiresIn > 0 {
		h.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second) // 60s buffer
	} else {
		h.ExpiresAt = time.Now().Add(1 * time.Hour) // Default 1 hour
	}

	klog.V(4).Infof("OAuth2 token refreshed, expires at: %v", h.ExpiresAt)
	return nil
}

// TokenRequest represents an OAuth2 token request
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// TokenResponse represents an OAuth2 token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

// requestToken makes the actual token request
func (h *OAuth2Handler) requestToken(ctx context.Context, tokenReq *TokenRequest) (*TokenResponse, error) {
	// This would implement the actual HTTP request to get the token
	// For now, returning a mock response since we're focusing on structure
	return &TokenResponse{
		AccessToken: "mock_access_token_" + fmt.Sprintf("%d", time.Now().Unix()),
		TokenType:   "Bearer",
		ExpiresIn:   3600, // 1 hour
	}, nil
}