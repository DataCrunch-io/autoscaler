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

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	schedulerframework "k8s.io/autoscaler/cluster-autoscaler/simulator/framework"
	klog "k8s.io/klog/v2"
)

type Asg struct {
	AsgRef
	minSize      int
	maxSize      int
	curSize      int
	instanceType string

	AvailabilityLocations []string
}

type AsgRef struct {
	Name string
}

type InstanceRef struct {
	ProviderID string
	Hostname   string
}

type DatacrunchNodeGroup struct {
	asg     *Asg
	manager *DatacrunchManager
}

// MaxSize returns maximum size of the node group.
func (ng *DatacrunchNodeGroup) MaxSize() int {
	return ng.asg.maxSize
}

// MinSize returns minimum size of the node group.
func (ng *DatacrunchNodeGroup) MinSize() int {
	return ng.asg.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kubernetes.
func (ng *DatacrunchNodeGroup) TargetSize() (int, error) {
	return ng.asg.curSize, nil
}

// IncreaseSize increases Asg size
func (ng *DatacrunchNodeGroup) IncreaseSize(delta int) error {
	return ng.manager.ScaleUpAsg(ng.asg, delta)
}

// AtomicIncreaseSize tries to increase the size of the node group atomically.
func (ng *DatacrunchNodeGroup) AtomicIncreaseSize(delta int) error {
	// not implemented
	return cloudprovider.ErrNotImplemented
}

// Belongs returns true if the given node belongs to the ASG.
func (ng *DatacrunchNodeGroup) Belongs(node *apiv1.Node) (bool, error) {
	ref, err := instanceRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		klog.Infof("[DEBUG] Failed to parse providerID for node %s: %v", node.Name, err)
		return false, err
	}
	targetAsg, err := ng.manager.GetAsgForInstance(ref)
	if err != nil {
		klog.Infof("[DEBUG] Error finding ASG for hostname %s: %v", ref.Hostname, err)
		return false, err
	}
	if targetAsg == nil {
		klog.Infof("[DEBUG] No ASG found for hostname %s", ref.Hostname)
		return false, fmt.Errorf("%s doesn't belong to a known asg", node.Name)
	}

	return targetAsg.Name == ng.asg.Name, nil
}

// DeleteNodes deletes the nodes from the group.
func (ng *DatacrunchNodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	if ng.asg.curSize <= ng.asg.minSize {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}

	refs := make([]InstanceRef, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := ng.Belongs(node)
		if err != nil {
			return err
		}
		if !belongs {
			return fmt.Errorf("%s belongs to a different asg than %s", node.Name, ng.asg.Name)
		}
		ref, err := instanceRefFromProviderId(node.Spec.ProviderID)
		if err != nil {
			return err
		}
		refs = append(refs, *ref)
	}

	return ng.manager.DeleteInstances(refs)
}

// ForceDeleteNodes deletes nodes from the group regardless of constraints.
func (ng *DatacrunchNodeGroup) ForceDeleteNodes(nodes []*apiv1.Node) error {
	return cloudprovider.ErrNotImplemented
}

// DecreaseTargetSize decreases the target size of the node group.
func (ng *DatacrunchNodeGroup) DecreaseTargetSize(delta int) error {
	return ng.manager.ScaleDownAsg(ng.asg, delta)
}

// Id returns asg id.
func (ng *DatacrunchNodeGroup) Id() string { return ng.asg.Name }

// Debug returns a debug string for the Asg.
func (ng *DatacrunchNodeGroup) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", ng.Id(), ng.MinSize(), ng.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
func (ng *DatacrunchNodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	return ng.manager.getInstancesForAsg(ng.asg.AsgRef)
}

// TemplateNodeInfo returns a framework.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The
// returned NodeInfo is expected to have a fully populated Node object, with
// all of the labels, capacity and allocatable information as well as all pods
// that are started on the node by default, using manifest (most likely only
// kube-proxy). Implementation optional.
func (ng *DatacrunchNodeGroup) TemplateNodeInfo() (*schedulerframework.NodeInfo, error) {
	klog.Infof("[DEBUG] TemplateNodeInfo called for ASG %s", ng.asg.Name)
	asgRef := AsgRef{Name: ng.asg.Name}
	template, err := ng.manager.getAsgTemplate(asgRef)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to get template for ASG %s: %v", ng.asg.Name, err)
		return nil, err
	}
	klog.Infof("[DEBUG] Got template for ASG %s: instanceType=%s", ng.asg.Name, ng.asg.instanceType)

	node, err := ng.manager.buildNodeFromTemplate(ng.asg, template)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to build node from template for ASG %s: %v", ng.asg.Name, err)
		return nil, err
	}

	nodeInfo := schedulerframework.NewNodeInfo(node, nil)
	klog.Infof("[DEBUG] TemplateNodeInfo created successfully for ASG %s: nodeName=%s", ng.asg.Name, node.Name)
	return nodeInfo, nil
}

// Exist checks if the node group really exists on the cloud provider side.
func (ng *DatacrunchNodeGroup) Exist() bool {
	if ng.asg == nil {
		return false
	}
	asgRef := AsgRef{Name: ng.asg.Name}
	asg, err := ng.manager.GetAsgByRef(asgRef)
	if err != nil {
		klog.Infof("[DEBUG] Error getting ASG by ref %s: %v", asgRef.Name, err)
		return false
	}
	return asg != nil
}

// Create creates the node group on the cloud provider side.
func (ng *DatacrunchNodeGroup) Create() (cloudprovider.NodeGroup, error) {
	klog.Infof("Creating ASG: %s", ng.asg.Name)
	// In DataCrunch, ASGs are logical - no explicit creation needed
	return ng, nil
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (ng *DatacrunchNodeGroup) Autoprovisioned() bool { return false }

// Delete deletes the node group on the cloud provider side.
func (ng *DatacrunchNodeGroup) Delete() error {
	return ng.manager.DeleteAsg(ng.asg)
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular ASG
func (ng *DatacrunchNodeGroup) GetOptions(defaults config.NodeGroupAutoscalingOptions) (*config.NodeGroupAutoscalingOptions, error) {
	return &defaults, nil
}
