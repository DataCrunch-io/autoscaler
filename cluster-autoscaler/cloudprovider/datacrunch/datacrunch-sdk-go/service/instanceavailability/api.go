package instanceavailability

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

const (
	// ServiceName is the name of the service
	ServiceName = "instance-availability"
)

// InstanceAvailabilityResponse represents the availability of instance types in a location
type InstanceAvailabilityResponse struct {
	LocationCode   string   `json:"location_code"`
	Availabilities []string `json:"availabilities"`
}

// ListInstanceAvailability lists all available instance types by location
func (c *InstanceAvailability) ListInstanceAvailability(ctx context.Context) ([]*InstanceAvailabilityResponse, error) {
	op := &request.Operation{
		Name:       "ListInstanceAvailability",
		HTTPMethod: "GET",
		HTTPPath:   "/instance-availability",
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
	var availabilities []*InstanceAvailabilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&availabilities); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	// Log the response
	log.Printf("Successfully retrieved availability for %d locations", len(availabilities))
	return availabilities, nil
}

// CheckInstanceAvailability checks if a specific instance type is available
func (c *InstanceAvailability) CheckInstanceAvailability(ctx context.Context, instanceType string) (bool, error) {
	op := &request.Operation{
		Name:       "CheckInstanceAvailability",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/instance-availability/%s", instanceType),
	}

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return false, req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return false, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return false, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return false, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Read the response body which should be a boolean
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, dcerr.New("SerializationError", "failed to read response body", err)
	}

	// Parse the boolean response
	var available bool
	if err := json.Unmarshal(body, &available); err != nil {
		return false, dcerr.New("SerializationError", "failed to decode response", err)
	}

	log.Printf("Instance type %s availability: %v", instanceType, available)
	return available, nil
}
