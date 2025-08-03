package datacrunch

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the cloud provider name for DataCrunch
	ProviderName = "datacrunch"
)

// DatacrunchCloudProvider implements CloudProvider interface for DataCrunch
type DatacrunchCloudProvider struct {
	manager    *DatacrunchManager
	wrapper    *DatacrunchWrapper
	nodeGroups []cloudprovider.NodeGroup
}

// BuildDatacrunchCloudProvider creates new DatacrunchCloudProvider
func BuildDatacrunchCloudProvider(manager *DatacrunchManager, discoveryOpts cloudprovider.NodeGroupDiscoveryOptions) (*DatacrunchCloudProvider, error) {
	wrapper := &DatacrunchWrapper{
		manager: manager,
	}

	provider := &DatacrunchCloudProvider{
		manager:    manager,
		wrapper:    wrapper,
		nodeGroups: []cloudprovider.NodeGroup{},
	}

	return provider, nil
}

// Name returns name of the cloud provider
func (d *DatacrunchCloudProvider) Name() string {
	return ProviderName
}

// NodeGroups returns all node groups configured for this cloud provider
func (d *DatacrunchCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	return d.nodeGroups
}

// NodeGroupForNode returns the node group for the given node
func (d *DatacrunchCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// HasInstance returns whether the given node has a corresponding instance in this cloud provider
func (d *DatacrunchCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	return false, cloudprovider.ErrNotImplemented  
}

// Pricing returns pricing model for this cloud provider or error if not available
func (d *DatacrunchCloudProvider) Pricing() (cloudprovider.PricingModel, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider
func (d *DatacrunchCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return []string{}, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided
func (d *DatacrunchCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.)
func (d *DatacrunchCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return d.manager.GetResourceLimiter(), nil
}

// GPULabel returns the label added to nodes with GPU resource
func (d *DatacrunchCloudProvider) GPULabel() string {
	return ""
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports
func (d *DatacrunchCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	return nil
}

// Cleanup cleans up resources open by this provider
func (d *DatacrunchCloudProvider) Cleanup() error {
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state
func (d *DatacrunchCloudProvider) Refresh() error {
	klog.V(4).Info("DataCrunch cloud provider refresh called")
	return nil
}