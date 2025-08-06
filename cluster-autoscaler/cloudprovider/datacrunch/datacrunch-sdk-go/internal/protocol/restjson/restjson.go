package restjson

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
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

// Build builds a request for the REST JSON protocol.
func Build(r *request.Request) {
	rest.Build(r)

	if t := rest.PayloadType(r.Params); t == "structure" || t == "" {
		if v := r.HTTPRequest.Header.Get("Content-Type"); len(v) == 0 {
			r.HTTPRequest.Header.Set("Content-Type", "application/json")
		}
		// Build JSON body using protocol-specific JSON utilities
		if r.ParamsFilled() {
			if body, err := jsonutil.BuildJSON(r.Params); err != nil {
				r.Error = err
			} else {
				r.SetBufferBody(body)
			}
		}
	}
}

// Unmarshal unmarshals a response body for the REST JSON protocol.
func Unmarshal(r *request.Request) {
	if t := rest.PayloadType(r.Data); t == "structure" || t == "" {
		// Unmarshal JSON using protocol-specific JSON utilities
		if r.DataFilled() && r.HTTPResponse.Body != nil {
			defer func() {
				if err := r.HTTPResponse.Body.Close(); err != nil {
					// Log the error but don't fail the function
					_ = err // Suppress unused variable warning
				}
			}()
			if err := jsonutil.UnmarshalJSON(r.Data, r.HTTPResponse.Body); err != nil {
				r.Error = err
			}
		}
	} else {
		rest.Unmarshal(r)
	}
}

// UnmarshalMeta unmarshals response headers for the REST JSON protocol.
func UnmarshalMeta(r *request.Request) {
	rest.UnmarshalMeta(r)
}
