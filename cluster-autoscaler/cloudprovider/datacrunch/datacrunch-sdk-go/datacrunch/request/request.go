package request

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
)

const (
	// ErrCodeSerialization is the serialization error code that is received
	// during protocol unmarshaling.
	ErrCodeSerialization = "SerializationError"

	// ErrCodeRead is an error that is returned during HTTP reads.
	ErrCodeRead = "ReadError"

	// ErrCodeResponseTimeout is the connection timeout error that is received
	// during body reads.
	ErrCodeResponseTimeout = "ResponseTimeout"

	// CanceledErrorCode is the error code that will be returned by an
	// API request that was canceled. Requests given a context.Context may
	// return this error when canceled.
	CanceledErrorCode = "RequestCanceled"

	// ErrCodeRequestError is an error preventing the SDK from continuing to
	// process the request.
	ErrCodeRequestError = "RequestError"
)

// Request is a simplified version focusing only on the essential functionality
// for making HTTP requests with retry support.
type Request struct {
	Config   interface{} // Using interface{} to avoid import cycle
	Handlers Handlers

	Retryer
	Operation    *Operation
	HTTPRequest  *http.Request
	HTTPResponse *http.Response
	Body         io.ReadSeeker
	Params       interface{}
	Error        error
	Data         interface{}
	RetryCount   int
	Retryable    *bool
	RetryDelay   time.Duration
	Time         time.Time

	context context.Context
	built   bool

	// Additional API error codes that should be retried.
	RetryErrorCodes []string

	// Additional API error codes that should be retried with throttle backoff delay.
	ThrottleErrorCodes []string
}

// Operation defines the API operation to be made.
type Operation struct {
	Name       string
	HTTPMethod string
	HTTPPath   string
}

// New creates a new Request pointer.
func New(cfg interface{}, handlers Handlers, retryer Retryer, operation *Operation, params interface{}, data interface{}) *Request {
	if retryer == nil {
		retryer = noOpRetryer{}
	}

	method := operation.HTTPMethod
	if method == "" {
		method = http.MethodPost
	}

	httpReq, _ := http.NewRequest(method, "", nil)

	// Extract BaseURL from config
	baseURL := "https://api.datacrunch.io/v1" // default fallback value

	// Try different config types
	switch config := cfg.(type) {
	case map[string]interface{}:
		if url, exists := config["BaseURL"]; exists {
			if str, ok := url.(string); ok {
				baseURL = str
			}
		}
	default:
		// Use reflection to check for BaseURL field in any config struct
		if cfgValue := reflect.ValueOf(cfg); cfgValue.IsValid() {
			// Handle both struct and pointer to struct
			if cfgValue.Kind() == reflect.Ptr && !cfgValue.IsNil() {
				cfgValue = cfgValue.Elem()
			}
			if cfgValue.Kind() == reflect.Struct {
				if field := cfgValue.FieldByName("BaseURL"); field.IsValid() && field.Kind() == reflect.String {
					if urlStr := field.String(); urlStr != "" {
						baseURL = urlStr
					}
				}
			}
		}
	}

	var err error
	httpReq.URL, err = url.Parse(baseURL)
	if err != nil {
		httpReq.URL = &url.URL{}
		err = dcerr.New("InvalidEndpointURL", "invalid endpoint uri", err)
	}

	if len(operation.HTTPPath) != 0 {
		opHTTPPath := operation.HTTPPath
		var opQueryString string
		if idx := strings.Index(opHTTPPath, "?"); idx >= 0 {
			opQueryString = opHTTPPath[idx+1:]
			opHTTPPath = opHTTPPath[:idx]
		}

		if strings.HasSuffix(httpReq.URL.Path, "/") && strings.HasPrefix(opHTTPPath, "/") {
			opHTTPPath = opHTTPPath[1:]
		}
		httpReq.URL.Path += opHTTPPath
		httpReq.URL.RawQuery = opQueryString
	}

	return &Request{
		Config:       cfg,
		Operation:    operation,
		Retryer:      retryer,
		Handlers:     handlers.Copy(),
		HTTPRequest:  httpReq,
		HTTPResponse: nil,
		Body:         nil,
		Params:       params,
		Error:        err,
		RetryCount:   0,
		Retryable:    nil,
	}
}

// Option is a functional option that can augment or modify a request.
type Option func(*Request)

// WithGetResponseHeader builds a request Option which will retrieve a single
// header value from the HTTP Response.
func WithGetResponseHeader(key string, val *string) Option {
	return func(r *Request) {
		r.Handlers.Complete.PushFunc(func(req *Request) {
			*val = req.HTTPResponse.Header.Get(key)
		})
	}
}

// WithGetResponseHeaders builds a request Option which will retrieve the
// headers from the HTTP response and assign them to the passed in headers
// variable.
func WithGetResponseHeaders(headers *http.Header) Option {
	return func(r *Request) {
		r.Handlers.Complete.PushFunc(func(req *Request) {
			*headers = req.HTTPResponse.Header
		})
	}
}

// ApplyOptions will apply each option to the request calling them in the order
// the were provided.
func (r *Request) ApplyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(r)
	}
}

// Context returns the request's context.
func (r *Request) Context() context.Context {
	if r.context == nil {
		return context.Background()
	}
	return r.context
}

// SetContext adds a Context to the current request.
func (r *Request) SetContext(ctx context.Context) {
	if ctx == nil {
		panic("nil context")
	}
	r.context = ctx
}

// ParamsFilled returns if the request's parameters have been populated
// and the parameters are valid.
func (r *Request) ParamsFilled() bool {
	return r.Params != nil && reflect.ValueOf(r.Params).Elem().IsValid()
}

// SetReaderBody sets the request's body reader.
func (r *Request) SetReaderBody(reader io.ReadSeeker) {
	r.Body = reader
	r.ResetBody()
}

// getNextRequestBody returns a new request body for the HTTP request.
// For simplified implementation, we just reset the body position.
func (r *Request) getNextRequestBody() (body io.ReadCloser, err error) {
	if r.Body != nil {
		// Reset the body to the beginning for retry
		_, err = r.Body.Seek(0, io.SeekStart)
		if err != nil {
			return nil, dcerr.New(ErrCodeSerialization,
				"failed to reset request body", err)
		}
		return io.NopCloser(r.Body), nil
	}

	return http.NoBody, nil
}

// DataFilled returns true if the request's data for response deserialization
// target has been set and is a valid.
func (r *Request) DataFilled() bool {
	return r.Data != nil && reflect.ValueOf(r.Data).Elem().IsValid()
}

// SetBufferBody sets the request's body bytes.
func (r *Request) SetBufferBody(buf []byte) {
	r.SetReaderBody(bytes.NewReader(buf))
}

// SetStringBody sets the body of the request to be backed by a string.
func (r *Request) SetStringBody(s string) {
	r.SetReaderBody(strings.NewReader(s))
}

// ResetBody rewinds the request body back to its starting position, and
// sets the HTTP Request body reference.
func (r *Request) ResetBody() {
	body, err := r.getNextRequestBody()
	if err != nil {
		r.Error = dcerr.New(ErrCodeSerialization,
			"failed to reset request body", err)
		return
	}

	r.HTTPRequest.Body = body
	r.HTTPRequest.GetBody = r.getNextRequestBody
}

// Send sends the request, returning error if errors are encountered.
func (r *Request) Send() error {
	defer func() {
		// Ensure a non-nil HTTPResponse parameter is set to ensure handlers
		// checking for HTTPResponse values, don't fail.
		if r.HTTPResponse == nil {
			r.HTTPResponse = &http.Response{
				Header: http.Header{},
				Body:   io.NopCloser(&bytes.Buffer{}),
			}
		}
		// Regardless of success or failure of the request trigger the Complete
		// request handlers.
		r.Handlers.Complete.Run(r)
	}()

	if err := r.Error; err != nil {
		return err
	}

	// Build the request once
	if !r.built {
		r.Handlers.Validate.Run(r)
		if r.Error != nil {
			return r.Error
		}
		r.Handlers.Build.Run(r)
		if r.Error != nil {
			return r.Error
		}
		r.built = true
	}

	for {
		r.Error = nil
		r.Time = time.Now()

		if err := r.sendRequest(); err == nil {
			return nil
		}

		// Check if we should retry
		if !r.ShouldRetry(r) || r.RetryCount >= r.MaxRetries() {
			return r.Error
		}

		// Calculate retry delay
		r.RetryDelay = r.RetryRules(r)
		if r.RetryDelay > 0 {
			time.Sleep(r.RetryDelay)
		}

		r.RetryCount++

		// Prepare for retry
		if err := r.prepareRetry(); err != nil {
			r.Error = err
			return err
		}
	}
}

// sendRequest performs the actual HTTP request
func (r *Request) sendRequest() error {
	// Set the request body
	body, err := r.getNextRequestBody()
	if err != nil {
		r.Error = err
		return err
	}
	r.HTTPRequest.Body = body

	// Create HTTP client with context support
	client := &http.Client{}
	if r.context != nil {
		r.HTTPRequest = r.HTTPRequest.WithContext(r.context)
	}

	// Perform the HTTP request
	resp, err := client.Do(r.HTTPRequest)
	if err != nil {
		r.Error = dcerr.New(ErrCodeRequestError, "failed to send request", err)
		return r.Error
	}

	r.HTTPResponse = resp

	// Run unmarshal handlers to process the response
	r.Handlers.Unmarshal.Run(r)
	return r.Error
}

// prepareRetry prepares the request for retry by resetting the body.
func (r *Request) prepareRetry() error {
	// Reset the body to the beginning for retry if body exists
	if r.Body != nil {
		_, err := r.Body.Seek(0, io.SeekStart)
		if err != nil {
			return dcerr.New(ErrCodeSerialization,
				"failed to prepare body for retry", err)
		}
	}

	// Closing response body to ensure that no response body is leaked
	// between retry attempts.
	if r.HTTPResponse != nil && r.HTTPResponse.Body != nil {
		if err := r.HTTPResponse.Body.Close(); err != nil {
			// Log the error but don't fail the retry preparation
			// as this is just cleanup
			_ = err // Suppress unused variable warning
		}
	}

	return nil
}
