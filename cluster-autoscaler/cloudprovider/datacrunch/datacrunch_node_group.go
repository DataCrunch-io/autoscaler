package datacrunch

import (
	"fmt"
	"strings"
	"sync"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	schedulerframework "k8s.io/autoscaler/cluster-autoscaler/simulator/framework"
	"k8s.io/klog/v2"
)

// DatacrunchNodeGroup implements cloudprovider.NodeGroup interface for DataCrunch
type DatacrunchNodeGroup struct {
	name         string
	nodeConfig   *NodeConfig
	instanceType string
	location     string
	minSize      int
	maxSize      int
	targetSize   int
	mutex        sync.Mutex
}

type DatacrunchNodeGroupSpec struct {
	minSize      int
	maxSize      int
	instanceType string
	location     string
	name         string
}

// DatacrunchInstance represents a DataCrunch instance for the cluster autoscaler
type DatacrunchInstance struct {
	ID           string
	ProjectID    string
	Hostname     string
	Status       string
	InstanceType string
	LocationCode Location
	Description  string // store the node group name
	CreatedAt    time.Time
}

// MaxSize returns maximum size of the node group
func (ng *DatacrunchNodeGroup) MaxSize() int {
	return ng.maxSize
}

// MinSize returns minimum size of the node group
func (ng *DatacrunchNodeGroup) MinSize() int {
	return ng.minSize
}

// TargetSize returns the current target size of the node group
func (ng *DatacrunchNodeGroup) TargetSize() (int, error) {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()
	return ng.targetSize, nil
}

// AtomicIncreaseSize increases the size of the node group atomically
func (ng *DatacrunchNodeGroup) AtomicIncreaseSize(delta int) error {
	return ng.IncreaseSize(delta)
}

// IncreaseSize increases the size of the node group
func (ng *DatacrunchNodeGroup) IncreaseSize(delta int) error {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()

	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}

	newTargetSize := ng.targetSize + delta
	if newTargetSize > ng.maxSize {
		return fmt.Errorf("size increase too large, would exceed maximum size %d", ng.maxSize)
	}

	klog.V(2).Infof("Increasing node group %s size from %d to %d", ng.name, ng.targetSize, newTargetSize)

	// Create new instances
	for i := 0; i < delta; i++ {
		_, err := ng.manager.dcService.CreateInstance(ng.nodeConfig, ng.name, ng.instanceType, ng.location)
		if err != nil {
			klog.Errorf("Failed to create instance for node group %s: %v", ng.name, err)
			return fmt.Errorf("failed to create instance: %v", err)
		}
	}

	ng.targetSize = newTargetSize
	klog.V(2).Infof("Successfully increased node group %s to %d instances", ng.name, ng.targetSize)
	return nil
}

// DecreaseTargetSize decreases the target size of the node group
func (ng *DatacrunchNodeGroup) DecreaseTargetSize(delta int) error {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()

	if delta < 0 {
		return fmt.Errorf("size decrease must be positive")
	}

	newTargetSize := ng.targetSize - delta
	if newTargetSize < ng.minSize {
		return fmt.Errorf("size decrease too large, would go below minimum size %d", ng.minSize)
	}

	klog.V(2).Infof("Decreasing node group %s target size from %d to %d", ng.name, ng.targetSize, newTargetSize)
	ng.targetSize = newTargetSize
	return nil
}

// ForceDeleteNodes forcefully deletes nodes from the group
func (ng *DatacrunchNodeGroup) ForceDeleteNodes(nodes []*apiv1.Node) error {
	return ng.DeleteNodes(nodes)
}

// DeleteNodes deletes nodes from the group
func (ng *DatacrunchNodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()

	if len(nodes) == 0 {
		return nil
	}

	klog.V(2).Infof("Deleting %d nodes from node group %s", len(nodes), ng.name)

	for _, node := range nodes {
		// Extract instance ID from node
		instanceID := ng.getInstanceIDFromNode(node)
		if instanceID == "" {
			klog.Warningf("Could not extract instance ID from node %s", node.Name)
			continue
		}

		err := ng.manager.dcService.DeleteNode(ng.name, instanceID)
		if err != nil {
			klog.Errorf("Failed to delete node %s: %v", node.Name, err)
			return fmt.Errorf("failed to delete node %s: %v", node.Name, err)
		}

		ng.targetSize--
	}

	klog.V(2).Infof("Successfully deleted %d nodes from node group %s", len(nodes), ng.name)
	return nil
}

// Id returns an unique identifier of the node group
func (ng *DatacrunchNodeGroup) Id() string {
	return ng.name
}

// Debug returns a string containing all information regarding this node group
func (ng *DatacrunchNodeGroup) Debug() string {
	return fmt.Sprintf("NodeGroup{name: %s, instanceType: %s, location: %s, minSize: %d, maxSize: %d, targetSize: %d}",
		ng.name, ng.instanceType, ng.location, ng.minSize, ng.maxSize, ng.targetSize)
}

// Nodes returns a list of all nodes that belong to this node group
func (ng *DatacrunchNodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	klog.V(4).Infof("Getting nodes for node group: %s", ng.name)

	nodeIDs, err := ng.manager.dcService.GetNodeGroupNodes(ng.name)
	if err != nil {
		return nil, fmt.Errorf("failed to get node group nodes: %v", err)
	}

	instances := make([]cloudprovider.Instance, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		instances[i] = cloudprovider.Instance{
			Id: nodeID,
			Status: &cloudprovider.InstanceStatus{
				State: cloudprovider.InstanceRunning,
			},
		}
	}

	return instances, nil
}

// TemplateNodeInfo returns a scheduler framework NodeInfo structure of an empty
// (as if just started) node, with all of the labels, capacity and allocatable
// information as well as all pods that should be running on the node.
func (ng *DatacrunchNodeGroup) TemplateNodeInfo() (*schedulerframework.NodeInfo, error) {
	klog.V(4).Infof("Creating template node info for node group: %s", ng.name)

	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("template-node-%s", ng.name),
			Labels: map[string]string{
				"kubernetes.io/arch":               "amd64",
				"kubernetes.io/os":                 "linux",
				"node.kubernetes.io/instance-type": ng.instanceType,
				"topology.kubernetes.io/zone":      ng.location,
				"datacrunch.io/node-group":         ng.name,
			},
		},
		Status: apiv1.NodeStatus{
			Capacity:    ng.getNodeCapacity(),
			Allocatable: ng.getNodeAllocatable(),
			Conditions: []apiv1.NodeCondition{
				{
					Type:   apiv1.NodeReady,
					Status: apiv1.ConditionTrue,
				},
			},
		},
	}

	// Add node group specific labels
	if ng.nodeConfig.Labels != nil {
		for key, value := range ng.nodeConfig.Labels {
			node.Labels[key] = value
		}
	}

	// Add taints
	if ng.nodeConfig.Taints != nil {
		for _, taint := range ng.nodeConfig.Taints {
			node.Spec.Taints = append(node.Spec.Taints, apiv1.Taint{
				Key:    taint.Key,
				Value:  taint.Value,
				Effect: apiv1.TaintEffect(taint.Effect),
			})
		}
	}

	nodeInfo := schedulerframework.NewNodeInfo(node, nil)

	return nodeInfo, nil
}

// Exist checks if the node group really exists on the cloud provider side
func (ng *DatacrunchNodeGroup) Exist() bool {
	// Node group exists if it's been created and has valid configuration
	return ng.nodeConfig != nil
}

// Create creates the node group on the cloud provider side
func (ng *DatacrunchNodeGroup) Create() (cloudprovider.NodeGroup, error) {
	klog.V(2).Infof("Creating node group: %s", ng.name)
	// In DataCrunch, node groups are logical - no explicit creation needed
	return ng, nil
}

// Delete deletes the node group on the cloud provider side
func (ng *DatacrunchNodeGroup) Delete() error {
	klog.V(2).Infof("Deleting node group: %s", ng.name)

	// Get all nodes and delete them
	nodes, err := ng.Nodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes for deletion: %v", err)
	}

	// Convert cloudprovider.Instance to apiv1.Node for deletion
	var nodeList []*apiv1.Node
	for _, instance := range nodes {
		node := &apiv1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: instance.Id,
			},
		}
		nodeList = append(nodeList, node)
	}

	return ng.DeleteNodes(nodeList)
}

// Autoprovisioned returns true if the node group is autoprovisioned
func (ng *DatacrunchNodeGroup) Autoprovisioned() bool {
	return false
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular NodeGroup
func (ng *DatacrunchNodeGroup) GetOptions(defaults config.NodeGroupAutoscalingOptions) (*config.NodeGroupAutoscalingOptions, error) {
	return &defaults, nil
}

// Helper methods

// getNodeCapacity returns the capacity of nodes in this node group
func (ng *DatacrunchNodeGroup) getNodeCapacity() apiv1.ResourceList {
	capacity := apiv1.ResourceList{
		apiv1.ResourcePods:   resource.MustParse("110"),
		apiv1.ResourceCPU:    resource.MustParse("4"),
		apiv1.ResourceMemory: resource.MustParse("16Gi"),
	}

	// Add GPU resources if this is a GPU node group
	if strings.Contains(ng.instanceType, "L40S") || strings.Contains(ng.instanceType, "A40") || strings.Contains(ng.instanceType, "A6000") {
		capacity["nvidia.com/gpu"] = resource.MustParse("1")
	}

	return capacity
}

// getNodeAllocatable returns the allocatable resources of nodes in this node group
func (ng *DatacrunchNodeGroup) getNodeAllocatable() apiv1.ResourceList {
	allocatable := apiv1.ResourceList{
		apiv1.ResourcePods:   resource.MustParse("110"),
		apiv1.ResourceCPU:    resource.MustParse("3800m"), // Reserve some CPU for system
		apiv1.ResourceMemory: resource.MustParse("15Gi"),  // Reserve some memory for system
	}

	// Add GPU resources if this is a GPU node group
	if strings.Contains(ng.instanceType, "L40S") || strings.Contains(ng.instanceType, "A40") || strings.Contains(ng.instanceType, "A6000") {
		allocatable["nvidia.com/gpu"] = resource.MustParse("1")
	}

	return allocatable
}

// getInstanceIDFromNode extracts the DataCrunch instance ID from a Kubernetes node
func (ng *DatacrunchNodeGroup) getInstanceIDFromNode(node *apiv1.Node) string {
	// Try to get from provider ID first
	if node.Spec.ProviderID != "" {
		if strings.HasPrefix(node.Spec.ProviderID, "datacrunch://") {
			return strings.TrimPrefix(node.Spec.ProviderID, "datacrunch://")
		}
	}

	// Fallback to node name if provider ID is not available
	return node.Name
}

// NewDatacrunchNodeGroup creates a new DataCrunch node group
func NewDatacrunchNodeGroup(
	manager *DatacrunchManager,
	wrapper *datacrunchWrapper,
	name string,
	nodeConfig *NodeConfig,
	instanceType string,
	location string,
	minSize int,
	maxSize int,
) *DatacrunchNodeGroup {
	return &DatacrunchNodeGroup{
		manager:      manager,
		name:         name,
		nodeConfig:   nodeConfig,
		instanceType: instanceType,
		location:     location,
		minSize:      minSize,
		maxSize:      maxSize,
		targetSize:   0, // Initialize to 0
	}
}
