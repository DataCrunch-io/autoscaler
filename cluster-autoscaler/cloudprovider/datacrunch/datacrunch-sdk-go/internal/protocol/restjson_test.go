package protocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/testutil"
)

func TestRESTJSONProtocol_Execute(t *testing.T) {
	tests := []struct {
		name           string
		request        *Request
		serverResponse func(*testutil.TestServer)
		expectedResult *Response
		expectedError  string
	}{
		{
			name: "successful GET request",
			request: &Request{
				Method: "GET",
				Path:   "/api/v1/instances",
				Query:  url.Values{"limit": []string{"10"}},
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetJSONResponse(200, testutil.MockAPIResponses.InstanceList)
			},
			expectedResult: &Response{
				StatusCode: 200,
			},
		},
		{
			name: "successful POST request with body",
			request: &Request{
				Method: "POST",
				Path:   "/api/v1/instances",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
				Body: testutil.MockRequestBodies.CreateInstance,
			},
			serverResponse: func(ts *testutil.TestServer) {
				response := map[string]interface{}{
					"id":     "inst-new123",
					"status": "creating",
				}
				ts.SetJSONResponse(201, response)
			},
			expectedResult: &Response{
				StatusCode: 201,
			},
		},
		{
			name: "handle 4xx error response",
			request: &Request{
				Method: "POST",
				Path:   "/api/v1/instances",
				Body:   map[string]interface{}{"invalid": "data"},
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetErrorResponse(400, "Invalid request data")
			},
			expectedResult: &Response{
				StatusCode: 400,
			},
		},
		{
			name: "handle 5xx error response",
			request: &Request{
				Method: "GET",
				Path:   "/api/v1/instances",
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetErrorResponse(500, "Internal server error")
			},
			expectedResult: &Response{
				StatusCode: 500,
			},
		},
		{
			name: "handle empty response body",
			request: &Request{
				Method: "DELETE",
				Path:   "/api/v1/instances/inst-123",
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetResponse(204, "", nil)
			},
			expectedResult: &Response{
				StatusCode: 204,
			},
		},
		{
			name: "handle custom headers",
			request: &Request{
				Method: "GET",
				Path:   "/api/v1/instances",
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"User-Agent":      "custom-agent/1.0",
				},
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetJSONResponse(200, testutil.MockAPIResponses.SuccessResponse)
			},
			expectedResult: &Response{
				StatusCode: 200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			ts := testutil.NewTestServer(t)
			defer ts.Close()

			// Configure server response
			tt.serverResponse(ts)

			// Create protocol with test server URL
			protocol := NewRESTJSONProtocol(ts.URL, &http.Client{
				Timeout: 5 * time.Second,
			})

			// Execute request
			ctx := context.Background()
			result, err := protocol.Execute(ctx, tt.request)

			// Check error expectation
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			// Check for unexpected errors
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check response
			if result.StatusCode != tt.expectedResult.StatusCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedResult.StatusCode, result.StatusCode)
			}

			// Verify request was made correctly
			lastReq := ts.GetLastRequest()
			if lastReq == nil {
				t.Fatal("No request was made to the server")
			}

			// Check method
			if lastReq.Method != tt.request.Method {
				t.Errorf("Expected method %s, got %s", tt.request.Method, lastReq.Method)
			}

			// Check path
			if lastReq.URL.Path != tt.request.Path {
				t.Errorf("Expected path %s, got %s", tt.request.Path, lastReq.URL.Path)
			}

			// Check query parameters
			for key, expectedValues := range tt.request.Query {
				actualValues := lastReq.URL.Query()[key]
				if len(actualValues) != len(expectedValues) {
					t.Errorf("Expected query param %s to have %d values, got %d", key, len(expectedValues), len(actualValues))
					continue
				}
				for i, expectedValue := range expectedValues {
					if actualValues[i] != expectedValue {
						t.Errorf("Expected query param %s[%d] to be %s, got %s", key, i, expectedValue, actualValues[i])
					}
				}
			}

			// Check headers
			for key, expectedValue := range tt.request.Headers {
				actualValue := lastReq.Header.Get(key)
				if actualValue != expectedValue {
					t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Check default headers
			if lastReq.Header.Get("Accept") != "application/json" {
				t.Errorf("Expected Accept header to be application/json, got %s", lastReq.Header.Get("Accept"))
			}

			// Check User-Agent (should be set if not provided)
			if tt.request.Headers["User-Agent"] == "" && lastReq.Header.Get("User-Agent") == "" {
				t.Error("Expected User-Agent header to be set")
			}

			// Check Content-Type for requests with body
			if tt.request.Body != nil {
				expectedContentType := tt.request.ContentType
				if expectedContentType == "" {
					expectedContentType = "application/json"
				}
				actualContentType := lastReq.Header.Get("Content-Type")
				if actualContentType != expectedContentType {
					t.Errorf("Expected Content-Type to be %s, got %s", expectedContentType, actualContentType)
				}
			}
		})
	}
}

func TestRESTJSONProtocol_Execute_NetworkErrors(t *testing.T) {
	tests := []struct {
		name          string
		serverSetup   func(*testutil.TestServer)
		expectedError string
	}{
		{
			name: "connection timeout",
			serverSetup: func(ts *testutil.TestServer) {
				ts.SimulateTimeout(10 * time.Second)
			},
			expectedError: "deadline exceeded",
		},
		{
			name: "connection refused",
			serverSetup: func(ts *testutil.TestServer) {
				ts.SimulateNetworkError()
			},
			expectedError: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := testutil.NewTestServer(t)
			defer ts.Close()

			tt.serverSetup(ts)

			protocol := NewRESTJSONProtocol(ts.URL, &http.Client{
				Timeout: 1 * time.Second, // Short timeout for testing
			})

			ctx := context.Background()
			request := &Request{
				Method: "GET",
				Path:   "/api/v1/test",
			}

			_, err := protocol.Execute(ctx, request)
			if err == nil {
				t.Error("Expected network error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
			}
		})
	}
}

func TestRESTJSONProtocol_UnmarshalResponse(t *testing.T) {
	protocol := NewRESTJSONProtocol("http://example.com", &http.Client{})

	tests := []struct {
		name           string
		response       *Response
		target         interface{}
		expectedResult interface{}
		expectedError  string
	}{
		{
			name: "successful JSON unmarshaling",
			response: &Response{
				StatusCode: 200,
				Body:       []byte(`{"id": "inst-123", "name": "test-instance"}`),
			},
			target:         &map[string]interface{}{},
			expectedResult: &map[string]interface{}{"id": "inst-123", "name": "test-instance"},
		},
		{
			name: "empty response body",
			response: &Response{
				StatusCode: 204,
				Body:       []byte{},
			},
			target:         &map[string]interface{}{},
			expectedResult: &map[string]interface{}{},
		},
		{
			name: "nil target",
			response: &Response{
				StatusCode: 200,
				Body:       []byte(`{"id": "inst-123"}`),
			},
			target:         nil,
			expectedResult: nil,
		},
		{
			name: "invalid JSON",
			response: &Response{
				StatusCode: 200,
				Body:       []byte(`{"invalid": json}`),
			},
			target:        &map[string]interface{}{},
			expectedError: "failed to unmarshal response",
		},
		{
			name: "API error response",
			response: &Response{
				StatusCode: 400,
				Body:       []byte(`{"code": 400, "message": "Bad request"}`),
			},
			target:        &map[string]interface{}{},
			expectedError: "API error 400",
		},
		{
			name: "HTTP error without JSON body",
			response: &Response{
				StatusCode: 500,
				Body:       []byte(`Internal Server Error`),
			},
			target:        &map[string]interface{}{},
			expectedError: "API request failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protocol.UnmarshalResponse(tt.response, tt.target)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.target != nil && tt.expectedResult != nil {
				expectedJSON, _ := json.Marshal(tt.expectedResult)
				actualJSON, _ := json.Marshal(tt.target)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("Expected result %s, got %s", string(expectedJSON), string(actualJSON))
				}
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		path            string
		modifications   func(*Request) *Request
		expectedMethod  string
		expectedPath    string
		expectedQuery   url.Values
		expectedHeaders map[string]string
		expectedBody    interface{}
	}{
		{
			name:            "basic request",
			method:          "GET",
			path:            "/api/v1/instances",
			modifications:   nil,
			expectedMethod:  "GET",
			expectedPath:    "/api/v1/instances",
			expectedQuery:   url.Values{},
			expectedHeaders: map[string]string{},
		},
		{
			name:   "request with query and headers",
			method: "GET",
			path:   "/api/v1/instances",
			modifications: func(r *Request) *Request {
				return r.WithQuery("limit", "10").
					WithQuery("page", "1").
					WithHeader("Authorization", "Bearer token123").
					WithHeader("X-Custom", "value")
			},
			expectedMethod: "GET",
			expectedPath:   "/api/v1/instances",
			expectedQuery: url.Values{
				"limit": []string{"10"},
				"page":  []string{"1"},
			},
			expectedHeaders: map[string]string{
				"Authorization": "Bearer token123",
				"X-Custom":      "value",
			},
		},
		{
			name:   "request with body",
			method: "POST",
			path:   "/api/v1/instances",
			modifications: func(r *Request) *Request {
				return r.WithBody(map[string]interface{}{
					"name": "test-instance",
					"type": "v1.small",
				})
			},
			expectedMethod:  "POST",
			expectedPath:    "/api/v1/instances",
			expectedQuery:   url.Values{},
			expectedHeaders: map[string]string{},
			expectedBody: map[string]interface{}{
				"name": "test-instance",
				"type": "v1.small",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := BuildRequest(tt.method, tt.path)

			if tt.modifications != nil {
				req = tt.modifications(req)
			}

			if req.Method != tt.expectedMethod {
				t.Errorf("Expected method %s, got %s", tt.expectedMethod, req.Method)
			}

			if req.Path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, req.Path)
			}

			// Check query parameters
			for key, expectedValues := range tt.expectedQuery {
				actualValues := req.Query[key]
				if len(actualValues) != len(expectedValues) {
					t.Errorf("Expected query param %s to have %d values, got %d", key, len(expectedValues), len(actualValues))
					continue
				}
				for i, expectedValue := range expectedValues {
					if actualValues[i] != expectedValue {
						t.Errorf("Expected query param %s[%d] to be %s, got %s", key, i, expectedValue, actualValues[i])
					}
				}
			}

			// Check headers
			for key, expectedValue := range tt.expectedHeaders {
				actualValue := req.Headers[key]
				if actualValue != expectedValue {
					t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Check body
			if tt.expectedBody != nil {
				expectedJSON, _ := json.Marshal(tt.expectedBody)
				actualJSON, _ := json.Marshal(req.Body)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("Expected body %s, got %s", string(expectedJSON), string(actualJSON))
				}
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError *APIError
		expected string
	}{
		{
			name: "error with details",
			apiError: &APIError{
				Code:    400,
				Message: "Invalid request",
				Details: "Missing required field: name",
			},
			expected: "API error 400: Invalid request (Missing required field: name)",
		},
		{
			name: "error without details",
			apiError: &APIError{
				Code:    404,
				Message: "Resource not found",
			},
			expected: "API error 404: Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.apiError.Error()
			if actual != tt.expected {
				t.Errorf("Expected error message '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}
