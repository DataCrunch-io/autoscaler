package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTestServer_BasicFunctionality(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Test basic request
	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check request was captured
	if ts.GetRequestCount() != 1 {
		t.Errorf("Expected 1 request, got %d", ts.GetRequestCount())
	}

	lastReq := ts.GetLastRequest()
	if lastReq.URL.Path != "/test" {
		t.Errorf("Expected path /test, got %s", lastReq.URL.Path)
	}
}

func TestTestServer_SetJSONResponse(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	testData := map[string]interface{}{
		"id":   "test-123",
		"name": "Test Item",
	}

	ts.SetJSONResponse(201, testData)

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var responseData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if responseData["id"] != "test-123" {
		t.Errorf("Expected id test-123, got %v", responseData["id"])
	}
}

func TestTestServer_SetErrorResponse(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SetErrorResponse(400, "Invalid input")

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	var errorData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&errorData)
	if err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	errorInfo, ok := errorData["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorInfo["message"] != "Invalid input" {
		t.Errorf("Expected error message 'Invalid input', got %v", errorInfo["message"])
	}
}

func TestTestServer_AssertionMethods(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Make a test request with specific parameters
	req, err := http.NewRequest("POST", ts.URL+"/api/v1/test?param=value", strings.NewReader(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Test assertion methods
	ts.AssertRequestMethod("POST")
	ts.AssertRequestPath("/api/v1/test")
	ts.AssertRequestHeader("Authorization", "Bearer token123")
	ts.AssertRequestQuery("param", "value")
	ts.AssertRequestBody(`{"test": "data"}`)
}

func TestTestServer_SimulateTimeout(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Simulate a 2 second timeout
	ts.SimulateTimeout(2 * time.Second)

	client := &http.Client{
		Timeout: 1 * time.Second, // Shorter than server timeout
	}

	start := time.Now()
	_, err := client.Get(ts.URL)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Should timeout around 1 second (client timeout)
	if duration < 900*time.Millisecond || duration > 1500*time.Millisecond {
		t.Errorf("Expected timeout around 1s, got %v", duration)
	}
}

func TestTestServer_ClearRequests(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Make some requests
	_, _ = http.Get(ts.URL + "/first")
	_, _ = http.Get(ts.URL + "/second")

	if ts.GetRequestCount() != 2 {
		t.Errorf("Expected 2 requests, got %d", ts.GetRequestCount())
	}

	// Clear requests
	ts.ClearRequests()

	if ts.GetRequestCount() != 0 {
		t.Errorf("Expected 0 requests after clear, got %d", ts.GetRequestCount())
	}

	if ts.GetLastRequest() != nil {
		t.Error("Expected nil last request after clear")
	}
}

func TestMockAPIResponses(t *testing.T) {
	// Test that mock responses can be marshaled
	tests := []struct {
		name     string
		response interface{}
	}{
		{"SuccessResponse", MockAPIResponses.SuccessResponse},
		{"ErrorResponse", MockAPIResponses.ErrorResponse},
		{"InstanceList", MockAPIResponses.InstanceList},
		{"SSHKeyList", MockAPIResponses.SSHKeyList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := json.Marshal(tt.response)
			if err != nil {
				t.Errorf("Failed to marshal %s: %v", tt.name, err)
			}
		})
	}
}

func TestMockRequestBodies(t *testing.T) {
	// Test that mock request bodies can be marshaled
	tests := []struct {
		name string
		body interface{}
	}{
		{"CreateInstance", MockRequestBodies.CreateInstance},
		{"CreateSSHKey", MockRequestBodies.CreateSSHKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := json.Marshal(tt.body)
			if err != nil {
				t.Errorf("Failed to marshal %s: %v", tt.name, err)
			}
		})
	}
}

func TestGetMockErrorResponse(t *testing.T) {
	errorResp := GetMockErrorResponse(404, "Resource not found")

	expectedJSON := `{"error":{"code":404,"message":"Resource not found"}}`
	actualJSON, err := json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	if string(actualJSON) != expectedJSON {
		t.Errorf("Expected JSON %s, got %s", expectedJSON, string(actualJSON))
	}
}

func TestGetMockListResponse(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": "1", "name": "Item 1"},
		map[string]interface{}{"id": "2", "name": "Item 2"},
	}

	listResp := GetMockListResponse(items, 2)

	// Check structure
	if listResp["total"] != 2 {
		t.Errorf("Expected total 2, got %v", listResp["total"])
	}

	if listResp["page"] != 1 {
		t.Errorf("Expected page 1, got %v", listResp["page"])
	}

	data, ok := listResp["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be slice")
	}

	if len(data) != 2 {
		t.Errorf("Expected 2 items in data, got %d", len(data))
	}
}

// Test error scenarios that might occur in real usage
func TestErrorScenarios(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	t.Run("malformed JSON response", func(t *testing.T) {
		ts.SetResponse(200, `{"invalid": json}`, nil)

		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Response should be returned, but JSON parsing would fail
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("empty response body", func(t *testing.T) {
		ts.SetResponse(204, "", nil)

		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != 204 {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}
	})

	t.Run("large response body", func(t *testing.T) {
		largeData := make(map[string]interface{})
		for i := 0; i < 1000; i++ {
			largeData[string(rune('A'+i%26))+string(rune('0'+i%10))] = strings.Repeat("data", 100)
		}

		ts.SetJSONResponse(200, largeData)

		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
