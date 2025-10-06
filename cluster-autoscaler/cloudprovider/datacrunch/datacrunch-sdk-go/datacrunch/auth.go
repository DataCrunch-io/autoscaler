package datacrunch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AuthService struct {
	client *Client
	mu     sync.RWMutex
	token  *TokenResponse
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	ExpiresAt    time.Time
}

func (s *AuthService) Authenticate() (*TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.doTokenRequest(TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     s.client.ClientID,
		ClientSecret: s.client.ClientSecret,
	})
}

func (s *AuthService) RefreshToken() (*TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token == nil || s.token.RefreshToken == "" {
		return s.authenticateWithoutLock()
	}

	return s.doTokenRequest(TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: s.token.RefreshToken,
		ClientID:     s.client.ClientID,
		ClientSecret: s.client.ClientSecret,
	})
}

func (s *AuthService) authenticateWithoutLock() (*TokenResponse, error) {
	return s.doTokenRequest(TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     s.client.ClientID,
		ClientSecret: s.client.ClientSecret,
	})
}

// doTokenRequest posts to /oauth2/token attempting JSON first and falling back to form-encoded
func (s *AuthService) doTokenRequest(body TokenRequest) (*TokenResponse, error) {
	// First try JSON body (per production API docs)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", s.client.BaseURL+"/oauth2/token", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.client.AuthBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.client.AuthBearerToken)
	}

	resp, err := s.client.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authentication request failed: %w", err)
	}

	var tokenResp TokenResponse
	if err := s.client.handleResponse(resp, &tokenResp); err == nil {
		tokenResp.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		s.token = &tokenResp
		return &tokenResp, nil
	} else {
		// Potentially staging requires form-encoded; inspect error and retry
		if apiErr, ok := err.(*APIError); ok {
			msg := strings.ToLower(apiErr.Message)
			if apiErr.StatusCode == 400 && (strings.Contains(msg, "grant_type") && strings.Contains(msg, "not specified") || strings.Contains(msg, "unsupported grant type") || strings.Contains(msg, "not valid json")) {
				form := url.Values{}
				form.Set("grant_type", body.GrantType)
				if body.ClientID != "" {
					form.Set("client_id", body.ClientID)
				}
				if body.ClientSecret != "" {
					form.Set("client_secret", body.ClientSecret)
				}
				if body.RefreshToken != "" {
					form.Set("refresh_token", body.RefreshToken)
				}

				req2, err2 := http.NewRequest("POST", s.client.BaseURL+"/oauth2/token", strings.NewReader(form.Encode()))
				if err2 != nil {
					return nil, fmt.Errorf("failed to create token request (form): %w", err2)
				}
				req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req2.Header.Set("Accept", "application/json")
				if s.client.AuthBearerToken != "" {
					req2.Header.Set("Authorization", "Bearer "+s.client.AuthBearerToken)
				}
				resp2, err2 := s.client.HTTPClient.Do(req2)
				if err2 != nil {
					return nil, fmt.Errorf("authentication request failed (form): %w", err2)
				}
				var tokenResp2 TokenResponse
				if err3 := s.client.handleResponse(resp2, &tokenResp2); err3 == nil {
					tokenResp2.ExpiresAt = time.Now().Add(time.Duration(tokenResp2.ExpiresIn) * time.Second)
					s.token = &tokenResp2
					return &tokenResp2, nil
				}
			}
		}
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
}

func (s *AuthService) GetValidToken() (*TokenResponse, error) {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == nil {
		return s.Authenticate()
	}

	if time.Now().Add(30 * time.Second).After(token.ExpiresAt) {
		return s.RefreshToken()
	}

	return token, nil
}

func (s *AuthService) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.token == nil {
		return true
	}

	return time.Now().Add(30 * time.Second).After(s.token.ExpiresAt)
}

func (s *AuthService) GetBearerToken() (string, error) {
	token, err := s.GetValidToken()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Bearer %s", token.AccessToken), nil
}
