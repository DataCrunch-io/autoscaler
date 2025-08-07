package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
)

// InstanceTypesAPI provides the interface for the instance types service
type InstanceTypesAPI interface {
	// ListInstanceTypes lists all available instance types
	ListInstanceTypes() ([]*instancetypes.InstanceTypeResponse, error)
	// GetInstanceTypePriceHistory gets the price history for instance types
	GetInstanceTypePriceHistory() (*instancetypes.PriceHistoryResponse, error)
}

var _ InstanceTypesAPI = (*instancetypes.InstanceTypes)(nil)
