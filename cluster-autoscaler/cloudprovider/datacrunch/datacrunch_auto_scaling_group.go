package datacrunch

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	schedulerframework "k8s.io/autoscaler/cluster-autoscaler/simulator/framework"
	klog "k8s.io/klog/v2"
)

type Asg struct {
	manager      *DatacrunchManager
	minSize      int
	maxSize      int
	locationCode string
	id           string
	instanceType string
}

// MaxSize returns maximum size of the node group.
func (asg *Asg) MaxSize() int {
	return asg.maxSize
}

// MinSize returns minimum size of the node group.
func (asg *Asg) MinSize() int {
	return asg.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kubernetes.
func (asg *Asg) TargetSize() (int, error) {
	size, err := asg.manager.GetAsgSize(asg)
	return int(size), err
}

// IncreaseSize increases Asg size
func (asg *Asg) IncreaseSize(delta int) error {
	klog.Infof("increase ASG:%s with %d nodes", asg.id, delta)
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := asg.manager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size)+delta > asg.MaxSize() {
		return fmt.Errorf("size increase is too large - desired:%d max:%d", int(size)+delta, asg.MaxSize())
	}
	return asg.manager.SetAsgSize(asg, size+int64(delta))
}

// AtomicIncreaseSize is not implemented.
func (asg *Asg) AtomicIncreaseSize(delta int) error { return cloudprovider.ErrNotImplemented }

// DecreaseTargetSize decreases the target size of the node group. Delta should be negative.
func (asg *Asg) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease size must be negative")
	}
	size, err := asg.manager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	nodes, err := asg.manager.GetAsgNodes(asg)
	if err != nil {
		return err
	}
	if int(size)+delta < len(nodes) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%d delta:%d existingNodes: %d", size, delta, len(nodes))
	}
	return asg.manager.SetAsgSize(asg, size+int64(delta))
}

// Belongs returns true if the given node belongs to the ASG.
func (asg *Asg) Belongs(node *apiv1.Node) (bool, error) {
	instanceID, _, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetAsg, err := asg.manager.GetAsgForInstance(instanceID)
	if err != nil {
		return false, err
	}
	if targetAsg == nil {
		return false, fmt.Errorf("%s doesn't belong to a known Asg", node.Name)
	}
	return targetAsg.Id() == asg.Id(), nil
}

// DeleteNodes deletes the nodes from the group.
func (asg *Asg) DeleteNodes(nodes []*apiv1.Node) error {
	size, err := asg.manager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size) <= asg.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	nodeIds := make([]string, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := asg.Belongs(node)
		if err != nil {
			return err
		}
		if !belongs {
			return fmt.Errorf("%s belongs to a different asg than %s", node.Name, asg.Id())
		}
		instanceID, _, err := toInstanceIDAndHostname(node.Spec.ProviderID)
		if err != nil {
			return err
		}
		nodeIds = append(nodeIds, instanceID)
	}
	return asg.manager.DeleteInstances(nodeIds)
}

// ForceDeleteNodes deletes nodes from the group regardless of constraints.
func (asg *Asg) ForceDeleteNodes(nodes []*apiv1.Node) error {
	return asg.DeleteNodes(nodes)
}

// Id returns asg id.
func (asg *Asg) Id() string { return asg.id }

// Debug returns a debug string for the Asg.
func (asg *Asg) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", asg.Id(), asg.MinSize(), asg.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
func (asg *Asg) Nodes() ([]cloudprovider.Instance, error) {
	instanceNames, err := asg.manager.GetAsgNodes(asg)
	if err != nil {
		return nil, err
	}
	instances := make([]cloudprovider.Instance, 0, len(instanceNames))
	for _, instanceName := range instanceNames {
		instances = append(instances, cloudprovider.Instance{Id: instanceName})
	}
	return instances, nil
}

// TemplateNodeInfo returns a node template for this node group.
func (asg *Asg) TemplateNodeInfo() (*schedulerframework.NodeInfo, error) {
	template, err := asg.manager.getAsgTemplate(asg.id)
	if err != nil {
		return nil, err
	}

	node, err := asg.manager.buildNodeFromTemplate(asg, template)
	if err != nil {
		klog.Errorf("failed to build node from template for ASG %s: %v", asg.Id(), err)
		return nil, err
	}

	nodeInfo := schedulerframework.NewNodeInfo(node, nil)
	return nodeInfo, nil
}

// Exist checks if the node group really exists on the cloud provider side.
func (asg *Asg) Exist() bool { return true }

// Create creates the node group on the cloud provider side.
func (asg *Asg) Create() (cloudprovider.NodeGroup, error) {
	klog.V(2).Infof("Creating ASG: %s", asg.id)
	// In DataCrunch, ASGs are logical - no explicit creation needed
	return asg, nil
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (asg *Asg) Autoprovisioned() bool { return false }

// Delete deletes the node group on the cloud provider side.
func (asg *Asg) Delete() error {
	klog.V(2).Infof("Deleting ASG: %s", asg.id)
	
	// Get all nodes and delete them
	nodes, err := asg.Nodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes for deletion: %v", err)
	}
	
	// Convert cloudprovider.Instance to instance IDs for deletion
	var instanceIds []string
	for _, instance := range nodes {
		instanceIds = append(instanceIds, instance.Id)
	}
	
	return asg.manager.DeleteInstances(instanceIds)
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular ASG
func (asg *Asg) GetOptions(defaults config.NodeGroupAutoscalingOptions) (*config.NodeGroupAutoscalingOptions, error) {
	return &defaults, nil
}
