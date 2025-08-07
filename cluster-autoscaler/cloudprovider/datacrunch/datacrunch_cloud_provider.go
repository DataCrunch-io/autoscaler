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
	"k8s.io/klog/v2"
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
	location     string
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
	return groups
}

// NodeGroupForNode returns the ASG for the given node
func (d *DatacrunchCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	instanceID, hostname, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	// find from cache will be faster need consider cache
	// the instanceID can not be used due to create the nodegroup before create the instance?
	// @todo to consider first
	klog.V(4).Infof("Found instanceID: %s, hostname: %s for node %s", instanceID, hostname, node.Name)

	// Use the registry to find the ASG for this instance
	asg, err := d.manager.GetAsgForInstance(instanceID)
	if err != nil {
		return nil, err
	}
	if asg != nil {
		return asg, nil
	}

	// do we need consider label ??? datacrunch.io/node-group

	// // Try to find node group from node labels
	// if nodeGroupName, exists := node.Labels["datacrunch.io/node-group"]; exists {
	// 	for _, ng := range d.nodeGroups {
	// 		if ng.Id() == nodeGroupName {
	// 			klog.V(4).Infof("Found node group %s for node %s", nodeGroupName, node.Name)
	// 			return ng, nil
	// 		}
	// 	}
	// }

	// // Fallback: try to match by instance type and location
	// instanceType := node.Labels["node.kubernetes.io/instance-type"]
	// zone := node.Labels["topology.kubernetes.io/zone"]

	// for _, ng := range d.nodeGroups {
	// 	if dcng, ok := ng.(*DatacrunchNodeGroup); ok {
	// 		if dcng.instanceType == instanceType && dcng.location == zone {
	// 			klog.V(4).Infof("Found node group %s for node %s by instance type matching", dcng.Id(), node.Name)
	// 			return ng, nil
	// 		}
	// 	}
	// }

	// klog.V(2).Infof("No node group found for node: %s", node.Name)
	return nil, nil
}

// HasInstance returns whether the given node has a corresponding instance in this cloud provider
func (d *DatacrunchCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	return true, cloudprovider.ErrNotImplemented
}

// Pricing returns pricing model for this cloud provider or error if not available
func (d *DatacrunchCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider
func (d *DatacrunchCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	// @todo need connect to call wrapper.GetAvailableMachineTypes()
	// and the wrapper api will create a new list based on apis and with cache
	// so the list will be expired after 10 minutes
	return []string{}, nil
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
	return ""
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports
func (d *DatacrunchCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	// @todo need connect to call wrapper.GetAvailableGPUTypes()
	// and the wrapper api will create a new list based on apis and with cache
	// so the list will be expired after 10 minutes
	return map[string]struct{}{
		"1L40S.20V": {},
		"A40.22V":   {},
		"A6000.24V": {},
	}
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
	klog.V(4).Info("DataCrunch cloud provider refresh called")

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

	manager, err := createDatacrunchManager(configFile)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch manager: %v", err)
	}

	provider, err := newDatacrunchCloudProvider(manager, rl)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch cloud provider: %v", err)
	}

	// add static ASGs
	if do.StaticDiscoverySpecified() {
		err := provider.addStaticASGs(do.NodeGroupSpecs)
		if err != nil {
			klog.Fatalf("Failed to add static ASGs: %v", err)
		}
	}

	return provider
}

func (d *DatacrunchCloudProvider) addStaticASGs(asgSpecs []string) error {
	for _, spec := range asgSpecs {
		asgSpec, err := d.parseAsgSpec(spec)
		if err != nil {
			klog.Errorf("Failed to parse ASG spec: %v", err)
			return err
		}

		// Initialize ASG wrapper for this static group
		asg := &Asg{
			manager:      d.manager,
			id:           asgSpec.name,
			minSize:      asgSpec.minSize,
			maxSize:      asgSpec.maxSize,
			locationCode: asgSpec.location,
			instanceType: asgSpec.instanceType,
		}
		d.manager.RegisterAsg(asg)
	}
	return nil
}

// parse format: min:max:instance-type:region:asg-name
func (d *DatacrunchCloudProvider) parseAsgSpec(spec string) (*DatacrunchAsgSpec, error) {
	parts := strings.Split(spec, ":")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid ASG spec: %s", spec)
	}

	instanceType := parts[2]
	region := parts[3]
	asgName := parts[4]

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

	return &DatacrunchAsgSpec{
		minSize:      minSize,
		maxSize:      maxSize,
		instanceType: instanceType,
		location:     region,
		name:         asgName,
	}, nil
}

// toInstanceID parses the providerID and returns the instanceID
func toInstanceIDAndHostname(providerID string) (string, string, error) {
	// try to parse the providerID as datacrunch://instance-id/hostname
	if !strings.HasPrefix(providerID, datacrunchProviderIDPrefix) {
		return "", "", fmt.Errorf("invalid providerID format: %s", providerID)
	}
	//
	_providerID := strings.TrimPrefix(providerID, datacrunchProviderIDPrefix)
	parts := strings.Split(_providerID, "/")
	if len(parts) < 2 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}
