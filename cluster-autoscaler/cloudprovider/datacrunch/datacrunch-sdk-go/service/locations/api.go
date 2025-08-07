package locations

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// LocationResponse represents a location
type LocationResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
}

// ListLocations lists all available locations
func (c *Locations) ListLocations() ([]*LocationResponse, error) {
	op := &request.Operation{
		Name:       "ListLocations",
		HTTPMethod: "GET",
		HTTPPath:   "/locations",
	}

	var locations []*LocationResponse
	req := c.newRequest(op, nil, &locations)

	return locations, req.Send()
}
