package dcerr

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPError represents an HTTP error response from the DataCrunch API
type HTTPError struct {
	// HTTP status code (e.g., 400, 401, 500)
	StatusCode int

	// Raw response body as string
	Body string

	// Parsed error response (if JSON)
	ErrorResponse *APIErrorResponse

	// Original error message
	Message string

	// Request info for debugging
	RequestInfo *RequestInfo
}

type RequestInfo struct {
	RequestURL     string
	RequestHeaders *http.Header
	RequestBody    []byte
}

// APIErrorResponse represents the standard DataCrunch API error format
type APIErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.ErrorResponse != nil {
		return fmt.Sprintf("HTTP %d: %s (%s)", e.StatusCode, e.ErrorResponse.Message, e.ErrorResponse.Code)
	}
	if e.Body != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// IsHTTPError checks if an error is an HTTPError and returns it
func IsHTTPError(err error) (*HTTPError, bool) {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr, true
	}
	return nil, false
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(statusCode int, body string, requestInfo *RequestInfo) *HTTPError {
	httpErr := &HTTPError{
		RequestInfo: requestInfo,
		StatusCode:  statusCode,
		Body:        body,
	}

	if requestInfo != nil {
		httpErr.RequestInfo = requestInfo
	}

	// Try to parse the response as JSON
	if body != "" {
		var apiErr APIErrorResponse
		if err := json.Unmarshal([]byte(body), &apiErr); err == nil {
			httpErr.ErrorResponse = &apiErr
		}
	}

	return httpErr
}

// GetStatusCode returns the HTTP status code, or 0 if not an HTTPError
func GetStatusCode(err error) int {
	if httpErr, ok := IsHTTPError(err); ok {
		return httpErr.StatusCode
	}
	return 0
}

// GetErrorBody returns the raw error response body, or empty string if not an HTTPError
func GetErrorBody(err error) string {
	if httpErr, ok := IsHTTPError(err); ok {
		return httpErr.Body
	}
	return ""
}

// GetAPIErrorCode returns the API error code, or empty string if not available
func GetAPIErrorCode(err error) string {
	if httpErr, ok := IsHTTPError(err); ok {
		if httpErr.ErrorResponse != nil {
			return httpErr.ErrorResponse.Code
		}
	}
	return ""
}

// GetAPIErrorMessage returns the API error message, or empty string if not available
func GetAPIErrorMessage(err error) string {
	if httpErr, ok := IsHTTPError(err); ok {
		if httpErr.ErrorResponse != nil {
			return httpErr.ErrorResponse.Message
		}
	}
	return ""
}
