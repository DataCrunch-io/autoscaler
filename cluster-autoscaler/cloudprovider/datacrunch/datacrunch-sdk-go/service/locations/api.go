package locations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// LocationResponse represents a location
type LocationResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
}

// ListLocations lists all available locations
func (c *Locations) ListLocations(ctx context.Context) ([]*LocationResponse, error) {
	op := &request.Operation{
		Name:       "ListLocations",
		HTTPMethod: "GET",
		HTTPPath:   "/locations",
	}

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return nil, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return nil, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Parse response
	var locations []*LocationResponse
	if err := json.NewDecoder(resp.Body).Decode(&locations); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return locations, nil
}
