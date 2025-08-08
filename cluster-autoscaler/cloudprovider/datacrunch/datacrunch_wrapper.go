package datacrunch

import (
	"fmt"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/session"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instanceavailability"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
)

type instanceI interface {
	ListInstances() ([]*instance.ListInstancesResponse, error)
	CreateInstance(input *instance.CreateInstanceInput) (string, error)
	PerformInstanceAction(input *instance.InstanceActionInput) error
}

type instanceTypesI interface {
	ListInstanceTypes() ([]*instancetypes.InstanceTypeResponse, error)
}

type startScriptsI interface {
	ListStartScripts() ([]*startscripts.StartScriptResponse, error)
	CreateStartScript(input *startscripts.CreateStartScriptInput) (string, error)
	DeleteStartScript(id string) error
}

type instanceAvailabilityI interface {
	ListInstanceAvailability() ([]*instanceavailability.InstanceAvailabilityResponse, error)
}

type customInstanceAvailabilityI interface {
	instanceAvailabilityI
	CheckInstanceAvailability(instanceType string, locationCode string) (bool, error)
}

type customInstanceAvailability struct {
	instanceAvailabilityI
}

// DatacrunchWrapper provides high-level operations for DataCrunch instances with environment variable injection
type datacrunchWrapper struct {
	instanceI
	instanceTypesI
	startScriptsI
	customInstanceAvailabilityI
}

func newCustomInstanceAvailability(session *session.Session) *customInstanceAvailability {
	return &customInstanceAvailability{
		instanceAvailabilityI: instanceavailability.New(session),
	}
}

// GetInstanceType retrieves instance type information
func (d *datacrunchWrapper) GetInstanceType(instanceTypeName string) (*InstanceType, error) {
	instanceTypes, err := d.ListInstanceTypes()
	if err != nil {
		return nil, err
	}

	for _, it := range instanceTypes {
		if it.InstanceType == instanceTypeName {
			return &InstanceType{
				CPU:    int64(it.CPU.NumberOfCores),
				Memory: int64(it.Memory.SizeInGigabytes) * 1024 * 1024 * 1024, // Convert GB to bytes
				GPU:    int64(it.GPU.NumberOfGPUs),
			}, nil
		}
	}

	// Return default values if not found
	return nil, fmt.Errorf("instance type %s not found", instanceTypeName)
}

func (ia *customInstanceAvailability) CheckInstanceAvailability(instanceType string, locationCode string) (bool, error) {
	instanceAvailabilityResponses, err := ia.ListInstanceAvailability()
	if err != nil {
		return false, err
	}
	// uppercase instanceType and locationCode
	_locationCode := strings.ToUpper(locationCode)
	_instanceType := strings.ToUpper(instanceType)

	for _, ia := range instanceAvailabilityResponses {
		if ia.LocationCode == _locationCode {
			// for each instanceType in ia.InstanceTypes
			for _, availability := range ia.Availabilities {
				if availability == _instanceType {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
