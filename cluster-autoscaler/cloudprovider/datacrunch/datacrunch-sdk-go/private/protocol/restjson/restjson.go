package restjson

import (
	"encoding/json"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/private/protocol/rest"
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
		
		// Build JSON body if we have parameters that aren't already handled by REST
		if r.ParamsFilled() && r.HTTPRequest.Body == nil {
			if data, err := json.Marshal(r.Params); err == nil {
				r.SetBufferBody(data)
			} else {
				r.Error = err
			}
		}
	}
}

// Unmarshal unmarshals a response body for the REST JSON protocol.
func Unmarshal(r *request.Request) {
	if t := rest.PayloadType(r.Data); t == "structure" || t == "" {
		// Handle JSON response
		if r.HTTPResponse != nil && r.HTTPResponse.Body != nil && r.DataFilled() {
			defer r.HTTPResponse.Body.Close()
			if err := json.NewDecoder(r.HTTPResponse.Body).Decode(r.Data); err != nil {
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
