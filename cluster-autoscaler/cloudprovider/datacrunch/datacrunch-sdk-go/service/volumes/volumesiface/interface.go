package volumesiface

import (
	"context"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/volumes"
)

// VolumesAPI provides the interface for the volumes service
type VolumesAPI interface {
	// ListVolumes lists all volumes
	ListVolumes(ctx context.Context) ([]*volumes.VolumeResponse, error)
	// GetVolume gets a volume by ID
	GetVolume(ctx context.Context, id string) (*volumes.VolumeResponse, error)
	// CreateVolume creates a new volume
	CreateVolume(ctx context.Context, input *volumes.CreateVolumeInput) (string, error)
	// PerformVolumeAction performs an action on a volume
	PerformVolumeAction(ctx context.Context, input *volumes.VolumeActionInput) error
	// ListTrashVolumes lists all volumes in trash
	ListTrashVolumes(ctx context.Context) ([]*volumes.VolumeResponse, error)
	// DeleteVolume deletes a volume by ID
	DeleteVolume(ctx context.Context, id string, isPermanent bool) error
}

var _ VolumesAPI = (*volumes.Volumes)(nil)
