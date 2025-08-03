package locations

import (
	"context"
	"log"

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

	var locations []*LocationResponse
	req := c.NewRequest(op, nil, &locations)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return locations, nil
}
