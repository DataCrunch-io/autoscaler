package common

// ResponseInterface defines the common interface for HTTP responses
type ResponseInterface interface {
	// DecodeJSON decodes the response body into the target interface
	DecodeJSON(target interface{}) error
	
	// GetStatusCode returns the HTTP status code
	GetStatusCode() int
	
	// GetBody returns the response body as bytes
	GetBody() []byte
}