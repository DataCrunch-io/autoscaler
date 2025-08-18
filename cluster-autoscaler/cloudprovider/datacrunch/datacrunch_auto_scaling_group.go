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
	"errors"
	"fmt"
	"sync"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	schedulerframework "k8s.io/autoscaler/cluster-autoscaler/simulator/framework"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"
)

type Asg struct {
	manager      *DatacrunchManager
	kubeClient   kubernetes.Interface
	minSize      int
	maxSize      int
	id           string
	instanceType string

	AvailabilityLocations []string

	asgMutex sync.Mutex
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
	klog.Infof("[DEBUG] TargetSize for ASG %s: %d (err: %v)", asg.id, size, err)
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

	asg.asgMutex.Lock()
	defer asg.asgMutex.Unlock()

	// instead of delegate to manager we need to do all in ASG scope

	// check if required resources are available
	location, err := asg.manager.instanceTypeAvailableLocation(asg.instanceType, asg.AvailabilityLocations)
	if err != nil {
		return fmt.Errorf("failed to check if instance type %s is available: %v", asg.instanceType, err)
	}
	if location == "" {
		return fmt.Errorf("instance type %s is not available in any of the locations", asg.instanceType)
	}
	klog.Infof("instance type %s is available in location %s", asg.instanceType, location)

	// set post update action like update cache or update target size
	defer func() {
		// post update action update cache
		klog.Infof("updating cache for ASG %s", asg.id)
	}()

	// prepare node config for each server
	nodeConfig, err := asg.manager.getNodeConfigForAsg(asg)
	if err != nil {
		return fmt.Errorf("failed to get node config for ASG %s: %v", asg.id, err)
	}

	// datcrunch doesnt support group server creation, we need to create each server manually
	// we need concurrent like create server and if error then need reduce actual delta
	waitGroup := sync.WaitGroup{}
	errsCh := make(chan error, delta)
	for i := 0; i < delta; i++ {
		waitGroup.Add(1)
		go func(index int, location string) {
			defer waitGroup.Done()
			klog.Infof("[DEBUG] Creating instance %d/%d for ASG %s", index+1, delta, asg.id)
			instanceID, hostname, err := asg.manager.createInstanceForAsg(asg, nodeConfig, location)
			if err != nil {
				klog.Errorf("[DEBUG] Failed to create instance %d for ASG %s: %v", index+1, asg.id, err)
				errsCh <- err
			} else {
				// if exists do nothing
				node, err := getNodeByName(asg.kubeClient, hostname)
				if err != nil {
					klog.Infof("node %s not found in k8s", hostname)
				}
				if node != nil {
					// if not exists then need to need update over k8s api
					providerID := fmt.Sprintf("%s%s/%s", datacrunchProviderIDPrefix, location, hostname)
					setNodeProviderID(asg.kubeClient, hostname, providerID)
				}
				klog.Infof("[DEBUG] Successfully created instance %s (hostname: %s) for ASG %s [%d/%d]", instanceID, hostname, asg.id, index+1, delta)
			}
		}(i, location)
	}
	waitGroup.Wait()
	close(errsCh)

	errs := make([]error, 0, delta)
	for err := range errsCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to create all servers: %w", errors.Join(errs...))
	}

	return nil
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

	if int(size)-delta < asg.minSize {
		return fmt.Errorf("attempt to scale down ASG %s by %d would go below min size %d (current: %d)",
			asg.id, delta, asg.minSize, int(size))
	}

	asg.asgMutex.Lock()
	defer asg.asgMutex.Unlock()

	// instead of delegate to manager we need to do all in ASG scope

	defer func() {
		// post update action update cache after delete
		klog.Infof("updating cache for ASG %s", asg.id)
	}()

	// get all instances by ASG
	instances, err := asg.manager.dcService.GetAllInstancesByDescription(asg.id)
	if err != nil {
		return fmt.Errorf("failed to get instances for ASG %s: %v", asg.id, err)
	}

	// delete instances
	deleteCount := 0
	waitGroup := sync.WaitGroup{}
	errsCh := make(chan error, len(instances))
	for i := 0; i < delta; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			instanceID := instances[i].ID
			err := asg.manager.DeleteInstances([]string{instanceID})
			if err != nil {
				errsCh <- err
			}
			deleteCount++
		}()
	}
	waitGroup.Wait()
	close(errsCh)

	errs := make([]error, 0, len(instances))
	for err := range errsCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete all instances: %w", errors.Join(errs...))
	}

	return nil
}

// Belongs returns true if the given node belongs to the ASG.
func (asg *Asg) Belongs(node *apiv1.Node) (bool, error) {
	klog.Infof("[DEBUG] Belongs() called for node %s against ASG %s", node.Name, asg.id)
	_, hostname, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		klog.Infof("[DEBUG] Failed to parse providerID for node %s: %v", node.Name, err)
		return false, err
	}
	klog.Infof("[DEBUG] Checking if hostname %s belongs to ASG %s", hostname, asg.id)
	targetAsg, err := asg.manager.GetAsgForInstanceByHostname(hostname)
	if err != nil {
		klog.Infof("[DEBUG] Error finding ASG for hostname %s: %v", hostname, err)
		return false, err
	}
	if targetAsg == nil {
		klog.Infof("[DEBUG] No ASG found for hostname %s", hostname)
		return false, fmt.Errorf("%s doesn't belong to a known Asg", node.Name)
	}
	belongs := targetAsg.Id() == asg.Id()
	klog.Infof("[DEBUG] Node %s belongs to ASG %s: %t (target ASG: %s)", node.Name, asg.id, belongs, targetAsg.Id())
	return belongs, nil
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
	klog.Infof("[DEBUG] Nodes() called for ASG %s", asg.id)
	instanceNames, err := asg.manager.GetAsgNodes(asg)
	if err != nil {
		klog.Infof("[DEBUG] Error getting nodes for ASG %s: %v", asg.id, err)
		return nil, err
	}
	klog.Infof("[DEBUG] ASG %s has %d nodes", asg.id, len(instanceNames))
	instances := make([]cloudprovider.Instance, 0, len(instanceNames))
	for i, instanceName := range instanceNames {
		klog.Infof("[DEBUG] ASG %s node %d: %s", asg.id, i+1, instanceName)
		instances = append(instances, cloudprovider.Instance{Id: instanceName})
	}
	return instances, nil
}

// TemplateNodeInfo returns a node template for this node group.
func (asg *Asg) TemplateNodeInfo() (*schedulerframework.NodeInfo, error) {
	klog.Infof("[DEBUG] TemplateNodeInfo requested for ASG %s (type: %s)",
		asg.id, asg.instanceType)
	template, err := asg.manager.getAsgTemplate(asg.id)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to get template for ASG %s: %v", asg.id, err)
		return nil, err
	}

	node, err := asg.manager.buildNodeFromTemplate(asg, template)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to build node from template for ASG %s: %v", asg.Id(), err)
		return nil, err
	}

	nodeInfo := schedulerframework.NewNodeInfo(node, nil)
	klog.Infof("[DEBUG] Created template node for ASG %s: CPU=%v, Memory=%v, GPU=%v",
		asg.id, node.Status.Capacity["cpu"], node.Status.Capacity["memory"], node.Status.Capacity["nvidia.com/gpu"])
	return nodeInfo, nil
}

// Exist checks if the node group really exists on the cloud provider side.
func (asg *Asg) Exist() bool { return true }

// Create creates the node group on the cloud provider side.
func (asg *Asg) Create() (cloudprovider.NodeGroup, error) {
	klog.Infof("Creating ASG: %s", asg.id)
	// In DataCrunch, ASGs are logical - no explicit creation needed
	return asg, nil
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (asg *Asg) Autoprovisioned() bool { return false }

// Delete deletes the node group on the cloud provider side.
func (asg *Asg) Delete() error {
	klog.Infof("Deleting ASG: %s", asg.id)

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
