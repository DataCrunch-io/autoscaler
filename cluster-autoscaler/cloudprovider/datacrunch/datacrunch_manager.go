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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
	klog "k8s.io/klog/v2"
)

const (
	datacrunchProviderIDPrefix = "datacrunch://"
	defaultPodAmountsLimit     = 110
)

type DatacrunchManager struct {
	cfg         *cloudConfig
	sdkProvider *datacrunchSDKProvider
	dcService   *datacrunchWrapper
	asgs        *autoScalingGroups
}

// asgTemplate holds template information for creating nodes
type asgTemplate struct {
	InstanceType *InstanceResource
	Location     string
	Tags         map[string]string
}

// InstanceType holds instance type information
type InstanceResource struct {
	InstanceType string
	Arch         string
	CPU          int64
	Memory       int64
	GPU          int64
}

func createDatacrunchManager(cloudReader io.Reader, discoveryOpts cloudprovider.NodeGroupDiscoveryOptions) (*DatacrunchManager, error) {
	cfg := &cloudConfig{}
	if cloudReader != nil {
		decoder := json.NewDecoder(cloudReader)
		if err := decoder.Decode(cfg); err != nil {
			return nil, err
		}
	}

	if !cfg.isValid() {
		return nil, errors.New("please check whether you have provided correct AccessKeyId,AccessKeySecret,RegionId or STS Token")
	}

	cfg = verifyCloudConfigAndPatch(cfg)

	// create the sdk provider
	sdkProvider, err := createDatacrunchSDKProvider(cfg)
	if err != nil {
		return nil, err
	}

	// create the datacrunch wrapper
	dcService := &datacrunchWrapper{
		instance.New(sdkProvider.session),
		instancetypes.New(sdkProvider.session),
		startscripts.New(sdkProvider.session),
		newCustomInstanceAvailability(sdkProvider.session),
		newCustomInstance(sdkProvider.session),
	}

	manager := &DatacrunchManager{
		cfg:         cfg,
		sdkProvider: sdkProvider,
		dcService:   dcService,
		asgs:        nil, // Will be set after creation
	}

	// Initialize ASG registry
	manager.asgs, err = newAutoScalingGroups(dcService, discoveryOpts.NodeGroupSpecs, cfg)
	if err != nil {
		return nil, err
	}

	return manager, nil
}

// Refresh updates manager state before each main loop
func (m *DatacrunchManager) Refresh() error {
	if err := m.asgs.regenerate(); err != nil {
		return err
	}

	return nil
}

func (m *DatacrunchManager) updateAsgInstanceCache(asg *Asg) error {
	klog.V(4).Infof("[DEBUG] Refreshing ASG %s state from DataCrunch API", asg.Name)

	instances, err := m.allASGRunningInstances(asg.Name)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to fetch instances for ASG %s: %v", asg.Name, err)
		return err
	}
	currentCount := len(instances)

	// check if current count is different from desired count

	klog.V(4).Infof("[DEBUG] ASG %s currently has %d running instances", asg.Name, currentCount)

	return nil
}

// allInstances returns all instances that belong to a given logical ASG (matched by Description)
func (m *DatacrunchManager) allASGRunningInstances(asgName string) ([]instance.ListInstancesResponse, error) {
	if asgName == "" {
		return nil, errors.New("asgName is required")
	}
	if m.dcService == nil || m.dcService.instanceI == nil {
		return nil, nil
	}
	instances, err := m.dcService.ListInstances(&instance.ListInstancesInput{
		Status: string(instance.InstanceStatusRunning),
	})

	if err != nil {
		return nil, err
	}

	filtered := make([]instance.ListInstancesResponse, 0, len(instances))
	for _, inst := range instances {
		if inst.Description == asgName {
			filtered = append(filtered, *inst)
		}
	}
	return filtered, nil
}

// GetAsgSize returns size for a given ASG by counting instances with matching Description
func (m *DatacrunchManager) GetAsgSize(asg *Asg) (int64, error) {
	if asg == nil {
		return 0, nil
	}

	// Return curSize to be consistent with TargetSize()
	// curSize is updated both during scale operations and during regenerate() from API
	return int64(asg.curSize), nil
}

// Scaleup ASG
func (m *DatacrunchManager) ScaleUpAsg(asg *Asg, delta int) error {
	return m.asgs.scaleUpAsg(asg, delta)
}

// Scaledown ASG
func (m *DatacrunchManager) ScaleDownAsg(asg *Asg, delta int) error {
	return m.asgs.scaleDownAsg(asg, delta)
}

func (m *DatacrunchManager) getAsgs() []*Asg {
	return m.asgs.getAsgs()
}

func (m *DatacrunchManager) GetAsgByRef(ref AsgRef) (*Asg, error) {
	return m.asgs.GetAsgByRef(ref)
}

// GetAsgNodes returns node provider IDs for instances in the ASG
func (m *DatacrunchManager) GetAsgNodes(asg *Asg) ([]string, error) {
	instances, err := m.allASGRunningInstances(asg.Name)
	if err != nil {
		return nil, err
	}
	providerIDs := make([]string, 0, len(instances))
	for _, inst := range instances {
		// providerID format: datacrunch://<location>/<hostname>
		providerID := datacrunchProviderIDPrefix + inst.Location + "/" + inst.Hostname
		providerIDs = append(providerIDs, providerID)
	}
	klog.Infof("[DEBUG] GetAsgNodes for %s: returning %d provider IDs", asg.Name, len(providerIDs))
	return providerIDs, nil
}

// GetAsgForInstance returns ASG that owns the instance by matching Description
func (m *DatacrunchManager) GetAsgForInstance(ref *InstanceRef) (*Asg, error) {
	if ref == nil {
		return nil, errors.New("ref is required")
	}

	// find from cache first
	return m.asgs.FindASGForInstance(ref)
}

// DeleteInstances deletes instances by ID from an ASG
func (m *DatacrunchManager) DeleteInstances(instanceRefs []InstanceRef) error {
	if len(instanceRefs) == 0 {
		return nil
	}

	for _, ref := range instanceRefs {
		err := m.asgs.DeleteInstance(ref)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *DatacrunchManager) DeleteAsg(asg *Asg) error {
	return m.asgs.DeleteAsg(asg.AsgRef)
}

func verifyCloudConfigAndPatch(cfg *cloudConfig) *cloudConfig {

	if cfg.BillingConfig.Contract == "" {
		cfg.BillingConfig.Contract = string(instance.BillingContractPayAsYouGo)
	}
	if cfg.BillingConfig.Price == "" {
		cfg.BillingConfig.Price = string(instance.BillingPriceDynamic)
	}
	return cfg
}

// cleanupCreatedInstances attempts to clean up instances that were created but need to be removed due to errors
func (m *DatacrunchManager) cleanupCreatedInstances(instanceIDs []string) {
	for _, instanceID := range instanceIDs {
		err := m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
			Action: instance.InstanceActionDelete,
			ID:     instanceID,
		})
		if err != nil {
			klog.Errorf("Failed to cleanup instance %s: %v", instanceID, err)
		} else {
			klog.Infof("Cleaned up instance %s", instanceID)
		}
	}
}

func (m *DatacrunchManager) GetAvailableMachineTypes() ([]string, error) {
	instanceTypes, err := m.dcService.ListInstanceTypes()
	if err != nil {
		return nil, err
	}

	types := make([]string, len(instanceTypes))
	for _, it := range instanceTypes {
		types = append(types, it.InstanceType)
	}
	return types, nil
}

func (m *DatacrunchManager) GetAvailableGPUTypes() map[string]struct{} {
	instanceTypes, err := m.dcService.ListInstanceTypes()
	if err != nil {
		return nil
	}

	types := make(map[string]struct{}, len(instanceTypes))
	for _, it := range instanceTypes {
		// if it.InstanceType start with "CPU."
		if strings.HasPrefix(strings.ToUpper(it.InstanceType), "CPU.") {
			continue
		}
		types[it.InstanceType] = struct{}{}
	}

	return types
}

func (m *DatacrunchManager) getInstancesForAsg(ref AsgRef) ([]cloudprovider.Instance, error) {
	asgInstances, err := m.asgs.InstancesForAsg(ref)
	if err != nil {
		return nil, err
	}
	cloudInstances := make([]cloudprovider.Instance, 0, len(asgInstances))
	for _, asgIns := range asgInstances {
		switch asgIns.Status {
		case string(instance.InstanceStatusRunning):
			cloudInstances = append(cloudInstances, cloudprovider.Instance{
				Id: asgIns.ID,
				Status: &cloudprovider.InstanceStatus{
					State: cloudprovider.InstanceRunning,
				},
			})
		case string(instance.InstanceStatusNew):
		case string(instance.InstanceStatusOrdered):
		case string(instance.InstanceStatusProvisioning):
			cloudInstances = append(cloudInstances, cloudprovider.Instance{
				Id: asgIns.ID,
				Status: &cloudprovider.InstanceStatus{
					State: cloudprovider.InstanceCreating,
				},
			})
		case string(instance.InstanceStatusOffline):
		case string(instance.InstanceStatusDiscontinued):
		case string(instance.InstanceStatusNotFound):
		case string(instance.InstanceStatusUnknown):
		case string(instance.InstanceStatusDeleting):
			cloudInstances = append(cloudInstances, cloudprovider.Instance{Id: asgIns.ID, Status: &cloudprovider.InstanceStatus{State: cloudprovider.InstanceDeleting}})
		case string(instance.InstanceStatusError):
			cloudInstances = append(cloudInstances, cloudprovider.Instance{Id: asgIns.ID,
				Status: &cloudprovider.InstanceStatus{ErrorInfo: &cloudprovider.InstanceErrorInfo{
					ErrorClass:   cloudprovider.OtherErrorClass,
					ErrorCode:    "no-code-datacrunch",
					ErrorMessage: "error",
				}}})
		default:
			cloudInstances = append(cloudInstances, cloudprovider.Instance{Id: asgIns.ID})
		}
	}
	return cloudInstances, nil
}

// buildNodeFromTemplate builds a Kubernetes node from ASG template
// not used
func (m *DatacrunchManager) buildNodeFromTemplate(asg *Asg, template *asgTemplate) (*apiv1.Node, error) {
	klog.Infof("[DEBUG] buildNodeFromTemplate for ASG %s: CPU=%d, Memory=%d, GPU=%d",
		asg.Name, template.InstanceType.CPU, template.InstanceType.Memory, template.InstanceType.GPU)

	node := &apiv1.Node{}
	nodeName := fmt.Sprintf("asg-%s-%d", asg.Name, rand.Int63())
	klog.Infof("[DEBUG] Generated template node name: %s", nodeName)

	labels := map[string]string{
		"kubernetes.io/arch":               template.InstanceType.Arch,
		"kubernetes.io/os":                 "linux",
		"node.kubernetes.io/instance-type": template.InstanceType.InstanceType,
		"topology.kubernetes.io/location":  strings.Join(asg.AvailabilityLocations, ","),
		"datacrunch.io/hostname":           asg.Name,
		GPULabel:                           asg.instanceType,
		nodeGroupLabel:                     asg.Name,
	}

	node.ObjectMeta = metav1.ObjectMeta{
		Name:   nodeName,
		Labels: labels,
	}

	// Set node capacity and allocatable based on instance type
	// Memory is already in bytes from the wrapper conversion
	memoryBytes := template.InstanceType.Memory
	memoryGi := memoryBytes / (1024 * 1024 * 1024)
	klog.Infof("[DEBUG] Setting node capacity: CPU=%d cores, Memory=%dGi (%d bytes), Pods=%d",
		template.InstanceType.CPU, memoryGi, memoryBytes, defaultPodAmountsLimit)

	capacity := apiv1.ResourceList{
		apiv1.ResourcePods:   *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI),
		apiv1.ResourceCPU:    *resource.NewQuantity(template.InstanceType.CPU, resource.DecimalSI),
		apiv1.ResourceMemory: *resource.NewQuantity(memoryBytes, resource.BinarySI),
	}

	// Add GPU resources if available
	if template.InstanceType.GPU > 0 {
		klog.Infof("[DEBUG] Adding GPU resources: %d nvidia.com/gpu", template.InstanceType.GPU)
		capacity[apiv1.ResourceName("nvidia.com/gpu")] = *resource.NewQuantity(template.InstanceType.GPU, resource.DecimalSI)
	} else {
		klog.Infof("[DEBUG] No GPU resources for instance type %s", asg.instanceType)
	}

	node.Status = apiv1.NodeStatus{
		Capacity:    capacity,
		Allocatable: capacity, // Simplified - in reality should account for system overhead
		Conditions:  cloudprovider.BuildReadyConditions(),
	}

	// Most providers set Allocatable == Capacity for the template.
	node.Status.Allocatable = node.Status.Capacity

	// Add custom labels from template tags
	for key, value := range template.Tags {
		node.Labels[key] = value
	}
	// ---- Taints ----
	node.Spec.Taints = append([]apiv1.Taint(nil), m.cfg.Taints...)

	klog.Infof("[DEBUG] Template node created successfully for ASG %s: %s", asg.Name, nodeName)
	return node, nil
}

// getAsgTemplate returns template information for ASG
func (m *DatacrunchManager) getAsgTemplate(asgRef AsgRef) (*asgTemplate, error) {
	// get asg from cache
	asg, err := m.asgs.GetAsgByRef(asgRef)
	if err != nil {
		return nil, err
	}

	klog.Infof("[DEBUG] About to call GetInstanceTypeDetails for ASG %s, instanceType: %s", asg.Name, asg.instanceType)
	instanceDetails, err := m.dcService.GetInstanceTypeDetails(asg.instanceType)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to get instance type details for ASG %s, type: %s, %v", asg.Name, asg.instanceType, err)
		return nil, err
	}
	klog.Infof("[DEBUG] Successfully got instance type details for ASG %s", asg.Name)

	return &asgTemplate{
		InstanceType: instanceDetails,
		Location:     "",
		Tags:         make(map[string]string),
	}, nil
}
