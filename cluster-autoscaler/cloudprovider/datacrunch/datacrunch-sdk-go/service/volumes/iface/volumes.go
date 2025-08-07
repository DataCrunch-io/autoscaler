package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/volumes"
)

// VolumesAPI provides the interface for the volumes service
type VolumesAPI interface {
	// ListVolumes lists all volumes
	ListVolumes() ([]*volumes.VolumeResponse, error)
	// GetVolume gets a volume by ID
	GetVolume(id string) (*volumes.VolumeResponse, error)
	// CreateVolume creates a new volume
	CreateVolume(input *volumes.CreateVolumeInput) (string, error)
	// PerformVolumeAction performs an action on a volume
	PerformVolumeAction(input *volumes.VolumeActionInput) error
	// ListTrashVolumes lists all volumes in trash
	ListTrashVolumes() ([]*volumes.VolumeResponse, error)
	// DeleteVolume deletes a volume by ID
	DeleteVolume(id string, isPermanent bool) error
}

var _ VolumesAPI = (*volumes.Volumes)(nil)
