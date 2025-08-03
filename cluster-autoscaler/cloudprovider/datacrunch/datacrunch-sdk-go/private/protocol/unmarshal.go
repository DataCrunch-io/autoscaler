package protocol

import (
	"io"
	"io/ioutil"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// UnmarshalDiscardBodyHandler is a named request handler to empty and close a response's body
var UnmarshalDiscardBodyHandler = request.NamedHandler{Name: "datacrunchsdk.shared.UnmarshalDiscardBody", Fn: UnmarshalDiscardBody}

// UnmarshalDiscardBody is a request handler to empty a response's body and closing it.
func UnmarshalDiscardBody(r *request.Request) {
	if r.HTTPResponse == nil || r.HTTPResponse.Body == nil {
		return
	}

	_, err := io.Copy(ioutil.Discard, r.HTTPResponse.Body)
	if err != nil {
		r.Error = dcerr.New(request.ErrCodeSerialization, "failed to copy response body", err)
	}
	err = r.HTTPResponse.Body.Close()
	if err != nil {
		r.Error = dcerr.New(request.ErrCodeSerialization, "failed to close response body", err)
	}
}

// ResponseMetadata provides the SDK response metadata attributes.
type ResponseMetadata struct {
	StatusCode int
	RequestID  string
}
