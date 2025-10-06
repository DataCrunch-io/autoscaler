package datacrunch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Response represents an HTTP response with additional metadata
type Response struct {
	*http.Response
	// Add any additional fields we might need later
}

// Standalone generic request functions

// getRequest performs a GET request and returns the response body, HTTP response, and error
// T represents the expected response body type that will be unmarshaled from JSON
func getRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error) {
	var respBody T

	req, err := client.NewRequest(ctx, "GET", url, nil)
	if err != nil {
		return respBody, nil, err
	}

	resp, err := client.Do(req, &respBody)
	if err != nil {
		return respBody, resp, err
	}

	return respBody, resp, nil
}

// postRequest performs a POST request and returns the response body, HTTP response, and error
// T represents the expected response body type that will be unmarshaled from JSON
func postRequest[T any](ctx context.Context, client *Client, url string, reqBody any) (T, *Response, error) {
	var respBody T

	var reqBodyReader io.Reader
	if reqBody != nil {
		reqBodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return respBody, nil, err
		}

		reqBodyReader = bytes.NewReader(reqBodyBytes)
	}

	req, err := client.NewRequest(ctx, "POST", url, reqBodyReader)
	if err != nil {
		return respBody, nil, err
	}

	resp, err := client.Do(req, &respBody)
	if err != nil {
		return respBody, resp, err
	}

	return respBody, resp, nil
}

// putRequest performs a PUT request and returns the response body, HTTP response, and error
// T represents the expected response body type that will be unmarshaled from JSON
// Note: Currently unused by DataCrunch API services, but provided for completeness and future API endpoints
//
//nolint:unused // Provided for complete HTTP method coverage
func putRequest[T any](ctx context.Context, client *Client, url string, reqBody any) (T, *Response, error) {
	var respBody T

	var reqBodyReader io.Reader
	if reqBody != nil {
		reqBodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return respBody, nil, err
		}

		reqBodyReader = bytes.NewReader(reqBodyBytes)
	}

	req, err := client.NewRequest(ctx, "PUT", url, reqBodyReader)
	if err != nil {
		return respBody, nil, err
	}

	resp, err := client.Do(req, &respBody)
	if err != nil {
		return respBody, resp, err
	}

	return respBody, resp, nil
}

// deleteRequest performs a DELETE request and returns the response body, HTTP response, and error
// T represents the expected response body type that will be unmarshaled from JSON
// Note: Currently unused by DataCrunch API services, but provided for completeness and future API endpoints
//
//nolint:unused // Provided for complete HTTP method coverage
func deleteRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error) {
	var respBody T

	req, err := client.NewRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return respBody, nil, err
	}

	resp, err := client.Do(req, &respBody)
	if err != nil {
		return respBody, resp, err
	}

	return respBody, resp, nil
}

// deleteRequestNoResult performs a DELETE request without expecting a response body
func deleteRequestNoResult(ctx context.Context, client *Client, url string) (*Response, error) {
	req, err := client.NewRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	return client.Do(req, nil)
}
