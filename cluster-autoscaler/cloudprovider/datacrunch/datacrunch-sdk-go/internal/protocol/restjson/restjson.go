package restjson

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/logger"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/json/jsonutil"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/rest"
)

// BuildHandler is a named request handler for building restjson protocol
// requests
var BuildHandler = request.NamedHandler{
	Name: "datacrunchsdk.restjson.Build",
	Fn:   Build,
}

// UnmarshalHandler is a named request handler for unmarshaling restjson
// protocol requests
var UnmarshalHandler = request.NamedHandler{
	Name: "datacrunchsdk.restjson.Unmarshal",
	Fn:   Unmarshal,
}

// UnmarshalMetaHandler is a named request handler for unmarshaling restjson
// protocol request metadata
var UnmarshalMetaHandler = request.NamedHandler{
	Name: "datacrunchsdk.restjson.UnmarshalMeta",
	Fn:   UnmarshalMeta,
}

// StringUnmarshalHandler is a named request handler for plain string response unmarshaling
// Used for APIs that return plain text strings instead of JSON
var StringUnmarshalHandler = request.NamedHandler{
	Name: "datacrunchsdk.restjson.StringUnmarshal",
	Fn:   StringUnmarshal,
}

// Build builds a request for the REST JSON protocol.
func Build(r *request.Request) {
	logger.Debug("restjson.Build: called for request %v", r)
	rest.Build(r)

	if t := rest.PayloadType(r.Params); t == "structure" || t == "" {
		logger.Debug("restjson.Build: PayloadType is structure or empty, setting Content-Type and building JSON body")
		if v := r.HTTPRequest.Header.Get("Content-Type"); len(v) == 0 {
			logger.Debug("restjson.Build: Content-Type header not set, setting to application/json")
			r.HTTPRequest.Header.Set("Content-Type", "application/json")
		}
		// Build JSON body using protocol-specific JSON utilities
		if r.ParamsFilled() {
			logger.Debug("restjson.Build: ParamsFilled, building JSON body")
			if body, err := jsonutil.BuildJSON(r.Params); err != nil {
				logger.Debug("restjson.Build: error building JSON body: %v", err)
				r.Error = err
			} else {
				logger.Debug("restjson.Build: JSON body built successfully, setting buffer body")
				r.SetBufferBody(body)
			}
		}
	}
}

// Unmarshal unmarshals a response body for the REST JSON protocol.
func Unmarshal(r *request.Request) {
	logger.Debug("restjson.Unmarshal: called for request %v", r)

	// Error handling is now done by DefaultErrorHandler in core defaults
	// This function only handles successful responses

	if t := rest.PayloadType(r.Data); t == "structure" || t == "" {
		logger.Debug("restjson.Unmarshal: PayloadType is structure or empty, will unmarshal JSON")
		// Unmarshal JSON using protocol-specific JSON utilities
		if r.DataFilled() && r.HTTPResponse.Body != nil {
			logger.Debug("restjson.Unmarshal: DataFilled and HTTPResponse.Body is not nil, unmarshaling JSON")
			defer func() {
				if err := r.HTTPResponse.Body.Close(); err != nil {
					logger.Debug("restjson.Unmarshal: error closing HTTPResponse.Body: %v", err)
					_ = err // Suppress unused variable warning
				}
			}()
			// Read the body first to log it
			body, readErr := io.ReadAll(r.HTTPResponse.Body)
			if readErr != nil {
				logger.Debug("restjson.Unmarshal: error reading response body: %v", readErr)
				r.Error = readErr
				return
			}

			// Log raw JSON for instance-availability endpoint
			if strings.Contains(r.HTTPRequest.URL.Path, "instance-availability") {
				logger.Debug("restjson.Unmarshal: Raw JSON response: %s", string(body))
			}

			// Create new reader from the body bytes
			bodyReader := bytes.NewReader(body)

			if err := jsonutil.UnmarshalJSON(r.Data, bodyReader); err != nil {
				logger.Debug("restjson.Unmarshal: error unmarshaling JSON: %v", err)
				r.Error = err
			} else {
				logger.Debug("restjson.Unmarshal: JSON unmarshaled successfully")
			}
		} else {
			logger.Debug("restjson.Unmarshal: Data not filled or HTTPResponse.Body is nil, skipping JSON unmarshal")
		}
	} else {
		logger.Debug("restjson.Unmarshal: PayloadType is not structure, delegating to rest.Unmarshal")
		rest.Unmarshal(r)
	}
}

// UnmarshalMeta unmarshals response headers for the REST JSON protocol.
func UnmarshalMeta(r *request.Request) {
	logger.Debug("restjson.UnmarshalMeta: called for request %v", r)
	rest.UnmarshalMeta(r)
}

// StringUnmarshal unmarshals a plain string response body.
// Used for APIs that return plain text strings instead of JSON objects.
func StringUnmarshal(r *request.Request) {
	logger.Debug("restjson.StringUnmarshal: called for request %v", r)
	if r.DataFilled() && r.HTTPResponse.Body != nil {
		logger.Debug("restjson.StringUnmarshal: DataFilled and HTTPResponse.Body is not nil, reading body as string")
		defer func() {
			if err := r.HTTPResponse.Body.Close(); err != nil {
				logger.Debug("restjson.StringUnmarshal: error closing HTTPResponse.Body: %v", err)
				_ = err // Suppress unused variable warning
			}
		}()

		// Read the response body as plain text
		body, err := io.ReadAll(r.HTTPResponse.Body)
		if err != nil {
			logger.Debug("restjson.StringUnmarshal: failed to read string response: %v", err)
			r.Error = dcerr.New(request.ErrCodeSerialization, "failed to read string response", err)
			return
		}

		// Set the string value directly
		if stringPtr, ok := r.Data.(*string); ok {
			logger.Debug("restjson.StringUnmarshal: setting string value on *string")
			*stringPtr = string(body)
		} else {
			logger.Debug("restjson.StringUnmarshal: expected *string data type, got %T", r.Data)
			r.Error = dcerr.New(request.ErrCodeSerialization,
				fmt.Sprintf("expected *string data type, got %T", r.Data), nil)
		}
	} else {
		logger.Debug("restjson.StringUnmarshal: Data not filled or HTTPResponse.Body is nil, skipping string unmarshal")
	}
}
