package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestServer represents a mock HTTP server for testing
type TestServer struct {
	*httptest.Server
	requests []*http.Request
	t        *testing.T
}

// NewTestServer creates a new test server
func NewTestServer(t *testing.T) *TestServer {
	ts := &TestServer{
		t:        t,
		requests: make([]*http.Request, 0),
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

// handler is the default handler that captures requests
func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	// Capture the request for inspection
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	ts.requests = append(ts.requests, r)

	// Default response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status": "ok"}`)
}

// SetResponse sets a custom response for the next request
func (ts *TestServer) SetResponse(statusCode int, body interface{}, headers map[string]string) {
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request
		reqBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(reqBody)))
		ts.requests = append(ts.requests, r)

		// Set headers
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Set status code
		w.WriteHeader(statusCode)

		// Write body
		if body != nil {
			switch v := body.(type) {
			case string:
				_, _ = fmt.Fprint(w, v)
			case []byte:
				_, _ = w.Write(v)
			default:
				jsonBody, err := json.Marshal(v)
				if err != nil {
					ts.t.Fatalf("Failed to marshal response body: %v", err)
				}
				_, _ = w.Write(jsonBody)
			}
		}
	})
}

// SetErrorResponse sets an error response
func (ts *TestServer) SetErrorResponse(statusCode int, errorMessage string) {
	errorBody := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": errorMessage,
		},
	}
	ts.SetResponse(statusCode, errorBody, nil)
}

// SetJSONResponse sets a JSON response
func (ts *TestServer) SetJSONResponse(statusCode int, data interface{}) {
	ts.SetResponse(statusCode, data, map[string]string{"Content-Type": "application/json"})
}

// GetLastRequest returns the last captured request
func (ts *TestServer) GetLastRequest() *http.Request {
	if len(ts.requests) == 0 {
		return nil
	}
	return ts.requests[len(ts.requests)-1]
}

// GetAllRequests returns all captured requests
func (ts *TestServer) GetAllRequests() []*http.Request {
	return ts.requests
}

// GetRequestCount returns the number of requests made
func (ts *TestServer) GetRequestCount() int {
	return len(ts.requests)
}

// ClearRequests clears all captured requests
func (ts *TestServer) ClearRequests() {
	ts.requests = make([]*http.Request, 0)
}

// AssertRequestMethod asserts that the last request used the expected method
func (ts *TestServer) AssertRequestMethod(expected string) {
	req := ts.GetLastRequest()
	if req == nil {
		ts.t.Fatal("No requests captured")
	}
	if req.Method != expected {
		ts.t.Errorf("Expected method %s, got %s", expected, req.Method)
	}
}

// AssertRequestPath asserts that the last request used the expected path
func (ts *TestServer) AssertRequestPath(expected string) {
	req := ts.GetLastRequest()
	if req == nil {
		ts.t.Fatal("No requests captured")
	}
	if req.URL.Path != expected {
		ts.t.Errorf("Expected path %s, got %s", expected, req.URL.Path)
	}
}

// AssertRequestBody asserts that the last request body matches expected
func (ts *TestServer) AssertRequestBody(expected interface{}) {
	req := ts.GetLastRequest()
	if req == nil {
		ts.t.Fatal("No requests captured")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		ts.t.Fatalf("Failed to read request body: %v", err)
	}

	switch exp := expected.(type) {
	case string:
		if string(body) != exp {
			ts.t.Errorf("Expected body %s, got %s", exp, string(body))
		}
	case []byte:
		if string(body) != string(exp) {
			ts.t.Errorf("Expected body %s, got %s", string(exp), string(body))
		}
	default:
		expectedJSON, err := json.Marshal(exp)
		if err != nil {
			ts.t.Fatalf("Failed to marshal expected body: %v", err)
		}
		if string(body) != string(expectedJSON) {
			ts.t.Errorf("Expected body %s, got %s", string(expectedJSON), string(body))
		}
	}
}

// AssertRequestHeader asserts that the last request has the expected header
func (ts *TestServer) AssertRequestHeader(key, expected string) {
	req := ts.GetLastRequest()
	if req == nil {
		ts.t.Fatal("No requests captured")
	}
	if actual := req.Header.Get(key); actual != expected {
		ts.t.Errorf("Expected header %s: %s, got %s", key, expected, actual)
	}
}

// AssertRequestQuery asserts that the last request has the expected query parameter
func (ts *TestServer) AssertRequestQuery(key, expected string) {
	req := ts.GetLastRequest()
	if req == nil {
		ts.t.Fatal("No requests captured")
	}
	if actual := req.URL.Query().Get(key); actual != expected {
		ts.t.Errorf("Expected query %s: %s, got %s", key, expected, actual)
	}
}

// SimulateTimeout simulates a request timeout
func (ts *TestServer) SimulateTimeout(duration time.Duration) {
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(duration)
		ts.handler(w, r)
	})
}

// SimulateNetworkError simulates a network error by closing connection
func (ts *TestServer) SimulateNetworkError() {
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			ts.t.Fatal("Server doesn't support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			ts.t.Fatalf("Failed to hijack connection: %v", err)
		}
		_ = conn.Close()
	})
}

// Close closes the test server
func (ts *TestServer) Close() {
	ts.Server.Close()
}
