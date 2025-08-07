package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
)

// InstanceAPI provides the interface for the instance service
type InstanceAPI interface {
	// ListInstances lists all instances
	ListInstances() ([]*instance.ListInstancesResponse, error)
	// CreateInstance creates a new instance
	CreateInstance(input *instance.CreateInstanceInput) (string, error)
	// PerformInstanceAction performs an action on an instance
	PerformInstanceAction(input *instance.InstanceActionInput) error
}

var _ InstanceAPI = (*instance.Instance)(nil)
