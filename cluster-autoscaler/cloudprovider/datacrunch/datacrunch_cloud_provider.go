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
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	// GPULabel is the label added to nodes with GPU resource.
	GPULabel       = "datacrunch.io/gpu-node"
	nodeGroupLabel = "datacrunch.io/node-group"
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
	d.manager.asgs.cacheMutex.Lock()
	defer d.manager.asgs.cacheMutex.Unlock()

	groups := make([]cloudprovider.NodeGroup, 0, len(d.manager.asgs.registeredAsgs))
	for _, asg := range d.manager.asgs.registeredAsgs {
		groups = append(groups, asg.config)
	}
	klog.Infof("[DEBUG] NodeGroups() returning %d node groups", len(groups))
	for i, group := range groups {
		klog.Infof("[DEBUG] NodeGroup %d: %s (min:%d, max:%d)", i+1, group.Id(), group.MinSize(), group.MaxSize())
	}
	return groups
}

// NodeGroupForNode returns the ASG for the given node
func (d *DatacrunchCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	location, hostname, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		klog.Errorf("[DEBUG] Node %s does not belong to DataCrunch (invalid providerID format): %v", node.Name, err)
		// Return nil, nil (not an error) when node doesn't belong to this cloud provider
		return nil, nil
	}

	klog.Infof("[DEBUG] Found location: %s, hostname: %s for node %s", location, hostname, node.Name)

	// Use the registry to find the ASG for this instance
	asg, err := d.manager.GetAsgForInstanceByHostname(hostname)
	if err != nil {
		klog.Warningf("[DEBUG] Error finding ASG for hostname %s: %v", hostname, err)
		return nil, err
	}

	if asg != nil {
		klog.Infof("[DEBUG] Found ASG %s for node %s (hostname: %s)", asg.Id(), node.Name, hostname)
		return asg, nil
	}

	klog.Warningf("[DEBUG] No ASG found for node %s (hostname: %s, location: %s)", node.Name, hostname, location)
	return nil, nil
}

// HasInstance returns whether the given node has a corresponding instance in this cloud provider
func (d *DatacrunchCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	_, hostname, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		klog.Errorf("[DEBUG] Node %s does not belong to DataCrunch (invalid providerID format): %v", node.Name, err)
		return false, nil
	}

	asg, err := d.manager.GetAsgForInstanceByHostname(hostname)
	if err != nil {
		klog.Warningf("[DEBUG] Error finding ASG for hostname %s: %v", hostname, err)
		return false, err
	}

	if asg != nil {
		return true, nil
	}

	return false, nil
}

// Pricing returns pricing model for this cloud provider or error if not available
func (d *DatacrunchCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider
func (d *DatacrunchCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return d.manager.getAvailableMachineTypes()
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
	if gpuQuantity, exists := node.Status.Capacity["nvidia.com/gpu"]; exists && !gpuQuantity.IsZero() {
		// DataCrunch uses nvidia.com/gpu as the standard GPU resource name
		return &cloudprovider.GpuConfig{
			Label:        "accelerator",
			Type:         "nvidia-gpu",
			ResourceName: "nvidia.com/gpu",
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
	return nil

	// return d.manager.Refresh()
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

	manager, err := createDatacrunchManager(configFile, createKubeClient(opts))
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch manager: %v", err)
	}

	provider, err := newDatacrunchCloudProvider(manager, rl)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch cloud provider: %v", err)
	}

	// add static ASGs
	if do.StaticDiscoverySpecified() {
		err := provider.addStaticASGs(do.NodeGroupSpecs, manager.cfg, createKubeClient(opts))
		if err != nil {
			klog.Fatalf("Failed to add static ASGs: %v", err)
		}
	}

	return provider
}

func (d *DatacrunchCloudProvider) addStaticASGs(asgSpecs []string, cfg *cloudConfig, kubeClient kubernetes.Interface) error {
	klog.Infof("[DEBUG] Adding %d static ASG specifications", len(asgSpecs))
	for i, spec := range asgSpecs {
		klog.Infof("[DEBUG] Processing ASG spec %d: %s", i+1, spec)
		asgSpec, err := d.parseAsgSpec(spec)
		if err != nil {
			klog.Errorf("Failed to parse ASG spec: %v", err)
			return err
		}

		// Initialize ASG wrapper for this static group
		klog.Infof("[DEBUG] Creating ASG: name=%s, min=%d, max=%d, type=%s",
			asgSpec.name, asgSpec.minSize, asgSpec.maxSize, asgSpec.instanceType)
		asg := &Asg{
			manager:               d.manager,
			kubeClient:            kubeClient,
			id:                    asgSpec.name,
			minSize:               asgSpec.minSize,
			maxSize:               asgSpec.maxSize,
			instanceType:          asgSpec.instanceType,
			AvailabilityLocations: cfg.AvailableLocations,
		}
		d.manager.RegisterAsg(asg)
		klog.Infof("[DEBUG] Successfully registered ASG: %s", asgSpec.name)

		// Check and enforce minimum size immediately after registration
		currentSize, err := d.manager.GetAsgSize(asg)
		if err != nil {
			klog.Errorf("[DEBUG] Error getting size for newly registered ASG %s: %v", asg.id, err)
		} else {
			klog.Infof("[DEBUG] Newly registered ASG %s current size: %d, minimum size: %d", asg.id, currentSize, asg.minSize)
			if currentSize < int64(asg.minSize) {
				needed := int64(asg.minSize) - currentSize
				klog.Infof("[DEBUG] ASG %s is below minimum size at startup! Current: %d, Min: %d, scaling up by %d instances",
					asg.id, currentSize, asg.minSize, needed)
				err = d.manager.SetAsgSize(asg, int64(asg.minSize))
				if err != nil {
					klog.Errorf("[DEBUG] Failed to scale ASG %s to minimum size at startup: %v", asg.id, err)
				} else {
					klog.Infof("[DEBUG] Successfully initiated startup scale-up for ASG %s to minimum size %d", asg.id, asg.minSize)
				}
			} else {
				klog.Infof("[DEBUG] ASG %s is already at or above minimum size (%d >= %d)", asg.id, currentSize, asg.minSize)
			}
		}
	}
	return nil
}

// parse format: min:max:instance-type:asg-name
func (d *DatacrunchCloudProvider) parseAsgSpec(spec string) (*DatacrunchAsgSpec, error) {
	klog.Infof("[DEBUG] Parsing ASG spec: %s", spec)
	parts := strings.Split(spec, ":")
	if len(parts) != 4 {
		klog.Errorf("[DEBUG] Invalid ASG spec format: expected 4 parts, got %d - %v", len(parts), parts)
		return nil, fmt.Errorf("invalid ASG spec: %s", spec)
	}

	instanceType := parts[2]
	asgName := parts[3]

	minSize, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid min size: %s", parts[0])
	}

	maxSize, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid max size: %s", parts[1])
	}

	validAsgName := regexp.MustCompile(`^[a-z0-9A-Z]+[a-z0-9A-Z\-\.\_]*[a-z0-9A-Z]+$|^[a-z0-9A-Z]{1}$`)
	if !validAsgName.MatchString(asgName) {
		return nil, fmt.Errorf("invalid ASG name: %s", asgName)
	}

	klog.Infof("[DEBUG] Parsed ASG spec successfully: min=%d, max=%d, instanceType=%s, name=%s",
		minSize, maxSize, instanceType, asgName)
	return &DatacrunchAsgSpec{
		minSize:      minSize,
		maxSize:      maxSize,
		instanceType: instanceType,
		name:         asgName,
	}, nil
}

// toInstanceID parses the providerID and returns the instanceID
func toInstanceIDAndHostname(providerID string) (string, string, error) {
	// try to parse the providerID as datacrunch://location/hostname
	if !strings.HasPrefix(providerID, datacrunchProviderIDPrefix) {
		return "", "", fmt.Errorf("invalid providerID format: %s", providerID)
	}
	//
	_providerID := strings.TrimPrefix(providerID, datacrunchProviderIDPrefix)
	parts := strings.Split(_providerID, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid providerID format: %s", providerID)
	}
	return parts[0], parts[1], nil
}

func getKubeConfig(opts config.AutoscalingOptions) *rest.Config {
	klog.Infof("Using kubeconfig file: %s", opts.KubeClientOpts.KubeConfigPath)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeClientOpts.KubeConfigPath)
	if err != nil {
		klog.Fatalf("Failed to build kubeConfig: %v", err)
	}

	return kubeConfig
}

func createKubeClient(opts config.AutoscalingOptions) kubernetes.Interface {
	return kubernetes.NewForConfigOrDie(getKubeConfig(opts))
}
