package instanceiface

import (
	"context"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
)

// InstanceAPI provides the interface for the instance service
type InstanceAPI interface {
	// ListInstances lists all instances
	ListInstances(ctx context.Context) ([]*instance.ListInstancesResponse, error)
	// CreateInstance creates a new instance
	CreateInstance(ctx context.Context, input *instance.CreateInstanceInput) (string, error)
	// PerformInstanceAction performs an action on an instance
	PerformInstanceAction(ctx context.Context, input *instance.InstanceActionInput) error
}

var _ InstanceAPI = (*instance.Instance)(nil)
