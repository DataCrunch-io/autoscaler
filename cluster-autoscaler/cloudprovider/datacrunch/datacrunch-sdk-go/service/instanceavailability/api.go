package instanceavailability

import (
	"fmt"

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
func (c *InstanceAvailability) ListInstanceAvailability() ([]*InstanceAvailabilityResponse, error) {
	op := &request.Operation{
		Name:       "ListInstanceAvailability",
		HTTPMethod: "GET",
		HTTPPath:   "/instance-availability",
	}

	var availabilities []*InstanceAvailabilityResponse
	req := c.newRequest(op, nil, &availabilities)

	return availabilities, req.Send()
}

// CheckInstanceAvailability checks if a specific instance type is available
func (c *InstanceAvailability) CheckInstanceAvailability(instanceType string) (bool, error) {
	op := &request.Operation{
		Name:       "CheckInstanceAvailability",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/instance-availability/%s", instanceType),
	}

	var available bool
	req := c.newRequest(op, nil, &available)

	return available, req.Send()
}
