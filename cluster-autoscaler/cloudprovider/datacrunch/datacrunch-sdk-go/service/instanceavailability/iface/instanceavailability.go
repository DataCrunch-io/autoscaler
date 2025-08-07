package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instanceavailability"
)

// InstanceAvailabilityAPI provides an interface to enable mocking the
// instance-availability service client's API operation.
type InstanceAvailabilityAPI interface {
	ListInstanceAvailability() ([]*instanceavailability.InstanceAvailabilityResponse, error)
	CheckInstanceAvailability(instanceType string) (bool, error)
}

var _ InstanceAvailabilityAPI = (*instanceavailability.InstanceAvailability)(nil)
