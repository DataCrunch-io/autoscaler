/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datacrunch

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/session"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instanceavailability"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
	klog "k8s.io/klog/v2"
)

// safeDeref safely dereferences an int64 pointer, returning the value or "nil" if pointer is nil
func safeDeref(p *int64) interface{} {
	if p == nil {
		return "nil"
	}
	return *p
}

type instanceI interface {
	ListInstances(input *instance.ListInstancesInput) ([]*instance.ListInstancesResponse, error)
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
	GetInstanceAvailabilityLocation(instanceType string, locations []string) (string, error)
	GetInstanceTypeDetails(instanceType string) (*InstanceResource, error)
}

type customInstanceAvailability struct {
	instanceAvailabilityI
	instanceTypesI
}

type customInstance struct {
	instanceI
}

type customInstanceI interface {
	GetInstanceByHostname(hostname string) (instance.ListInstancesResponse, error)
	GetAllInstancesByDescription(description string) ([]instance.ListInstancesResponse, error)
}

// DatacrunchWrapper provides high-level operations for DataCrunch instances with environment variable injection
type datacrunchWrapper struct {
	instanceI
	instanceTypesI
	startScriptsI
	customInstanceAvailabilityI
	customInstanceI
}

func newCustomInstanceAvailability(session *session.Session) *customInstanceAvailability {
	return &customInstanceAvailability{
		instanceAvailabilityI: instanceavailability.New(session),
		instanceTypesI:        instancetypes.New(session),
	}
}

func (ia *customInstanceAvailability) GetInstanceAvailabilityLocation(instanceType string, locations []string) (string, error) {
	if len(locations) == 0 {
		return "", fmt.Errorf("locations is empty")
	}

	instanceAvailabilityResponses, err := ia.ListInstanceAvailability()
	if err != nil {
		klog.Errorf("[DEBUG] Error fetching availability data: %v\n", err)
		return "", err
	}

	// convert all locations to upper case
	newLocations := make([]string, len(locations))
	for i, location := range locations {
		newLocations[i] = strings.ToUpper(location)
	}

	for _, availabilityData := range instanceAvailabilityResponses {
		if slices.Contains(newLocations, availabilityData.LocationCode) {
			for _, availability := range availabilityData.Availabilities {
				if availability == instanceType {
					klog.Infof("[DEBUG] Instance type %s is AVAILABLE in location %s\n", instanceType, availabilityData.LocationCode)
					return availabilityData.LocationCode, nil
				}
			}
			continue
		}
	}

	klog.Warningf("[DEBUG] Location %s not found in availability data\n", newLocations)
	return "", nil
}

func (ia *customInstanceAvailability) GetInstanceTypeDetails(instanceType string) (*InstanceResource, error) {
	if instanceType == "" {
		return nil, fmt.Errorf("instance type is empty")
	}

	instanceTypeDetails, err := ia.ListInstanceTypes()
	if err != nil {
		klog.Errorf("[DEBUG] Error fetching instance types: %v\n", err)
		return nil, err
	}

	// Add detailed debugging for each instance type
	for _, it := range instanceTypeDetails {

		// Look for your specific instance type
		if it.InstanceType == instanceType {

			klog.Infof("[DEBUG] MATCH FOUND for %s: CPU=%v, Memory=%vGB, GPU=%v\n",
				instanceType, safeDeref(it.CPU.NumberOfCores), safeDeref(it.Memory.SizeInGigabytes), safeDeref(it.GPU.NumberOfGPUs))
			return &InstanceResource{
				InstanceType: it.InstanceType,
				Arch:         "amd64",
				CPU:          *it.CPU.NumberOfCores,
				Memory:       *it.Memory.SizeInGigabytes * 1024 * 1024 * 1024,
				GPU:          *it.GPU.NumberOfGPUs,
			}, nil
		}
	}
	klog.Errorf("[DEBUG] Instance type %s not found in API response\n", instanceType)

	return nil, fmt.Errorf("instance type %s not found", instanceType)
}

func newCustomInstance(session *session.Session) *customInstance {
	return &customInstance{
		instanceI: instance.New(session),
	}
}

func (ia *customInstance) GetInstanceByHostname(hostname string) (instance.ListInstancesResponse, error) {
	instances, err := ia.ListInstances(&instance.ListInstancesInput{Status: string(instance.InstanceStatusRunning)})
	if err != nil {
		return instance.ListInstancesResponse{}, err
	}

	for _, instance := range instances {
		if instance.Hostname == hostname {
			return *instance, nil
		}
	}
	return instance.ListInstancesResponse{}, fmt.Errorf("instance with hostname %s not found", hostname)
}

func (ia *customInstance) GetAllInstancesByDescription(description string) ([]instance.ListInstancesResponse, error) {
	instances, err := ia.ListInstances(&instance.ListInstancesInput{
		Status: string(instance.InstanceStatusRunning),
	})
	if err != nil {
		klog.Errorf("[DEBUG] ListInstances API call failed: %v", err)
		return []instance.ListInstancesResponse{}, err
	}

	filteredInstances := make([]instance.ListInstancesResponse, 0, len(instances))
	for _, instance := range instances {
		// will match with asg name with description which uses for asg name
		if instance.Description == description {
			filteredInstances = append(filteredInstances, *instance)
		}
	}

	klog.Infof("[DEBUG] GetAllInstancesByDescription returning %d filtered instances for description '%s'",
		len(filteredInstances), description)
	return filteredInstances, nil
}
