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
	"io"
	"os"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/klog/v2"
)

const (
	// GPULabel is the label added to nodes with GPU resource.
	GPULabel          = "datacrunch.io/gpu-node"
	nodeGroupLabel    = "datacrunch.io/node-group"
	ResourceNvidiaGPU = "nvidia.com/gpu"
)

// DatacrunchCloudProvider implements CloudProvider interface for DataCrunch
type DatacrunchCloudProvider struct {
	manager         *DatacrunchManager
	resourceLimiter *cloudprovider.ResourceLimiter
}

// DatacrunchAsgSpec holds ASG specification
type DatacrunchAsgSpec struct {
	minSize      int
	maxSize      int
	instanceType string
	name         string
}

// newDatacrunchCloudProvider implement CloudProvider interface
func newDatacrunchCloudProvider(manager *DatacrunchManager, rl *cloudprovider.ResourceLimiter) (*DatacrunchCloudProvider, error) {
	return &DatacrunchCloudProvider{
		manager:         manager,
		resourceLimiter: rl,
	}, nil
}

// Name returns name of the cloud provider
func (d *DatacrunchCloudProvider) Name() string {
	return cloudprovider.DatacrunchProviderName
}

// NodeGroups returns all ASGs configured for this cloud provider
func (d *DatacrunchCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	asgs := d.manager.getAsgs()
	groups := make([]cloudprovider.NodeGroup, 0, len(asgs))
	for _, asg := range asgs {
		groups = append(groups, &DatacrunchNodeGroup{asg: asg, manager: d.manager})
	}

	klog.Infof("[DEBUG] NodeGroups() returning %d node groups", len(groups))
	return groups
}

// NodeGroupForNode returns the ASG for the given node
func (d *DatacrunchCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	instanceRef, err := instanceRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return nil, nil
	}

	asg, err := d.manager.GetAsgForInstance(instanceRef)
	if err != nil {
		return nil, err
	}

	return &DatacrunchNodeGroup{asg: asg, manager: d.manager}, nil
}

// HasInstance returns whether the given node has a corresponding instance in this cloud provider
func (d *DatacrunchCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	instanceRef, err := instanceRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return false, nil
	}

	asg, err := d.manager.GetAsgForInstance(instanceRef)
	if err != nil {
		return false, err
	}

	return asg != nil, nil
}

// Pricing returns pricing model for this cloud provider or error if not available
func (d *DatacrunchCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider
func (d *DatacrunchCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return d.manager.GetAvailableMachineTypes()
}

// NewNodeGroup builds a theoretical ASG based on the node definition provided
func (d *DatacrunchCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.)
func (d *DatacrunchCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return d.resourceLimiter, nil
}

// GPULabel returns the label added to nodes with GPU resource
func (d *DatacrunchCloudProvider) GPULabel() string {
	return GPULabel
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports
func (d *DatacrunchCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	return d.manager.GetAvailableGPUTypes()
}

// GetNodeGpuConfig returns the label, type and resource name for the GPU added to node
func (d *DatacrunchCloudProvider) GetNodeGpuConfig(node *apiv1.Node) *cloudprovider.GpuConfig {
	// Check if node has GPU resources
	gpuLabel := d.GPULabel()
	_, hasGpuLabel := node.Labels[gpuLabel]
	gpuAllocatable, hasGpuAllocatable := node.Status.Allocatable[ResourceNvidiaGPU]
	if hasGpuLabel || (hasGpuAllocatable && !gpuAllocatable.IsZero()) {
		return &cloudprovider.GpuConfig{
			Label:        gpuLabel,
			Type:         node.Labels[gpuLabel],
			ResourceName: ResourceNvidiaGPU,
		}
	}
	return nil
}

// Cleanup cleans up resources open by this provider
func (d *DatacrunchCloudProvider) Cleanup() error {
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state
func (d *DatacrunchCloudProvider) Refresh() error {
	klog.Info("[DEBUG] DataCrunch cloud provider refresh called - checking ASG states")

	return d.manager.Refresh()
}

// BuildDatacrunchCloudProvider builds the DataCrunch cloud provider.
func BuildDatacrunch(
	opts config.AutoscalingOptions,
	do cloudprovider.NodeGroupDiscoveryOptions,
	rl *cloudprovider.ResourceLimiter,
) cloudprovider.CloudProvider {

	var configFile io.ReadCloser
	if opts.CloudConfig != "" {
		var err error
		configFile, err = os.Open(opts.CloudConfig)
		if err != nil {
			klog.Fatalf("Couldn't open cloud provider configuration %s: %#v", opts.CloudConfig, err)
		}
		defer configFile.Close()
	}

	manager, err := createDatacrunchManager(configFile, do)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch manager: %v", err)
	}

	provider, err := newDatacrunchCloudProvider(manager, rl)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch cloud provider: %v", err)
	}

	return provider
}
