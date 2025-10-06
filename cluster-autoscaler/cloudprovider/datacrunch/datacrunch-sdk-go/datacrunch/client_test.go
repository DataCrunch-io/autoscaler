package datacrunch

import (
	"net/http"
	"testing"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/testutil"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		opts      []ClientOption
		wantError bool
	}{
		{
			name: "valid config",
			opts: []ClientOption{
				WithClientID("test_id"),
				WithClientSecret("test_secret"),
			},
			wantError: false,
		},
		{
			name: "missing client ID",
			opts: []ClientOption{
				WithClientSecret("test_secret"),
			},
			wantError: true,
		},
		{
			name: "missing client secret",
			opts: []ClientOption{
				WithClientID("test_id"),
			},
			wantError: true,
		},
		{
			name: "custom base URL",
			opts: []ClientOption{
				WithClientID("test_id"),
				WithClientSecret("test_secret"),
				WithBaseURL("https://custom.example.com/v1"),
			},
			wantError: false,
		},
		{
			name: "custom HTTP client",
			opts: []ClientOption{
				WithClientID("test_id"),
				WithClientSecret("test_secret"),
				WithHTTPClient(&http.Client{}),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.opts...)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected client, got nil")
				return
			}

			// Check default base URL
			expectedBaseURL := client.BaseURL
			if expectedBaseURL == "" {
				expectedBaseURL = DefaultBaseURL
			}
			if client.BaseURL != expectedBaseURL {
				t.Errorf("expected BaseURL %s, got %s", expectedBaseURL, client.BaseURL)
			}

			// Check credentials are set (validation happens in NewClient)
			if client.ClientID == "" {
				t.Error("ClientID should be set")
			}
			if client.ClientSecret == "" {
				t.Error("ClientSecret should be set")
			}

			// Check services are initialized
			if client.Auth == nil {
				t.Error("Auth service not initialized")
			}
			if client.Instances == nil {
				t.Error("Instances service not initialized")
			}
			if client.Balance == nil {
				t.Error("Balance service not initialized")
			}
			if client.SSHKeys == nil {
				t.Error("SSHKeys service not initialized")
			}
			if client.Volumes == nil {
				t.Error("Volumes service not initialized")
			}
			if client.StartupScripts == nil {
				t.Error("StartupScripts service not initialized")
			}
			if client.Locations == nil {
				t.Error("Locations service not initialized")
			}
			if client.Containers == nil {
				t.Error("Containers service not initialized")
			}
		})
	}
}

func TestClientMakeRequest(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Test successful request
	t.Run("successful request", func(t *testing.T) {
		resp, err := client.makeRequest("GET", "/balance", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close() //nolint:errcheck // Test code
		} else {
			t.Error("expected response, got nil")
		}
	})

	// Test request with body
	t.Run("request with body", func(t *testing.T) {
		requestBody := map[string]string{"test": "data"}
		resp, err := client.makeRequest("POST", "/instances", requestBody)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close() //nolint:errcheck // Test code
		} else {
			t.Error("expected response, got nil")
		}
	})
}

func TestClientHandleResponse(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("successful response parsing", func(t *testing.T) {
		resp, err := client.makeRequest("GET", "/balance", nil)
		if err != nil {
			t.Fatalf("unexpected error making request: %v", err)
		}

		var balance Balance
		err = client.handleResponse(resp, &balance)
		if err != nil {
			t.Errorf("unexpected error handling response: %v", err)
		}

		if balance.Amount != 100.50 {
			t.Errorf("expected amount 100.50, got %f", balance.Amount)
		}
		if balance.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", balance.Currency)
		}
	})

	t.Run("error response", func(t *testing.T) {
		// Set up a custom handler that returns an error
		mockServer.SetHandler("GET", "/error-test", func(w http.ResponseWriter, r *http.Request) {
			testutil.ErrorResponse(w, http.StatusBadRequest, "Test error message")
		})

		resp, err := client.makeRequest("GET", "/error-test", nil)
		if err != nil {
			t.Fatalf("unexpected error making request: %v", err)
		}

		err = client.handleResponse(resp, nil)
		if err == nil {
			t.Error("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Errorf("expected APIError, got %T", err)
		} else {
			if apiErr.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
			}
			if apiErr.Message != "Test error message" {
				t.Errorf("expected message 'Test error message', got '%s'", apiErr.Message)
			}
		}
	})
}
