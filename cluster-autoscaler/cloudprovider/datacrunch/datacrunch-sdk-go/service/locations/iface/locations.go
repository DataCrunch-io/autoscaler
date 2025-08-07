package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/locations"
)

// LocationsAPI provides the interface for the locations service
type LocationsAPI interface {
	// ListLocations lists all available locations
	ListLocations() ([]*locations.LocationResponse, error)
}

var _ LocationsAPI = (*locations.Locations)(nil)
