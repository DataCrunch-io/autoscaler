package instanceavailability

import (
	"context"
	"fmt"
	"log"

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

	var availabilities []*InstanceAvailabilityResponse
	req := c.NewRequest(op, nil, &availabilities)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
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

	var available bool
	req := c.NewRequest(op, nil, &available)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return false, err
	}

	log.Printf("Instance type %s availability: %v", instanceType, available)
	return available, nil
}
