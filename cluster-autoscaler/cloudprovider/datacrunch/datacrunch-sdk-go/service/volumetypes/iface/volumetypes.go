package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/volumetypes"
)

// VolumeTypesAPI provides the interface for the volume types service
type VolumeTypesAPI interface {
	// ListVolumeTypes lists all available volume types
	ListVolumeTypes() ([]*volumetypes.VolumeTypeResponse, error)
}

var _ VolumeTypesAPI = (*volumetypes.VolumeTypes)(nil)
