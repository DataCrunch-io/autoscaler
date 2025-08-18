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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"
)

const (
	datacrunchProviderIDPrefix = "datacrunch://"
)

type DatacrunchManager struct {
	cfg         *cloudConfig
	sdkProvider *datacrunchSDKProvider
	dcService   *datacrunchWrapper
	kubeClient  kubernetes.Interface
	asgs        *autoScalingGroups
}

// asgTemplate holds template information for creating nodes
type asgTemplate struct {
	InstanceType *InstanceType
	Location     string
	Tags         map[string]string
}

// InstanceType holds instance type information
type InstanceType struct {
	CPU    int64
	Memory int64
	GPU    int64
}

func createDatacrunchManager(cloudReader io.Reader, kubeClient kubernetes.Interface) (*DatacrunchManager, error) {
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
		kubeClient:  kubeClient,
		asgs:        nil, // Will be set after creation
	}

	// Initialize ASG registry
	manager.asgs = newAutoScalingGroups(manager)

	return manager, nil
}

// Refresh updates manager state before each main loop
func (m *DatacrunchManager) Refresh() error {

	for _, asg := range m.asgs.registeredAsgs {
		currentSize, err := m.GetAsgSize(asg.config)
		if err != nil {
			klog.Warningf("[DEBUG] Error getting size for ASG %s: %v", asg.config.id, err)
			continue
		}
		klog.Infof("[DEBUG] ASG %s current size: %d", asg.config.id, currentSize)

		if currentSize < int64(asg.config.minSize) {
			klog.Infof("[DEBUG] ASG %s is below minimum size: %d", asg.config.id, currentSize)
			err = m.SetAsgSize(asg.config, int64(asg.config.minSize))
			if err != nil {
				klog.Warningf("[DEBUG] Error setting size for ASG %s: %v", asg.config.id, err)
			}
		} else if currentSize > int64(asg.config.maxSize) {
			klog.Infof("[DEBUG] ASG %s is above maximum size: %d", asg.config.id, currentSize)
			err = m.SetAsgSize(asg.config, int64(asg.config.maxSize))
			if err != nil {
				klog.Warningf("[DEBUG] Error setting size for ASG %s: %v", asg.config.id, err)
			}
		} else {
			klog.Infof("[DEBUG] ASG %s is at desired size: %d", asg.config.id, currentSize)
		}
	}

	return nil
}

// allInstances returns all instances that belong to a given logical ASG (matched by Description)
func (m *DatacrunchManager) allAsgRunningInstances(asgName string) ([]instance.ListInstancesResponse, error) {
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
	instances, err := m.allAsgRunningInstances(asg.id)
	if err != nil {
		klog.Warningf("[DEBUG] Error getting instances for ASG %s: %v", asg.id, err)
		return -1, err
	}

	return int64(len(instances)), nil
}

// SetAsgSize sets desired group size by creating or deleting instances.
func (m *DatacrunchManager) SetAsgSize(asg *Asg, size int64) error {
	if asg == nil {
		return nil
	}
	if size < 0 {
		return errors.New("size must be non-negative")
	}

	// Get current size
	currentSize, err := m.GetAsgSize(asg)
	if err != nil {
		return fmt.Errorf("failed to get current ASG size: %v", err)
	}

	if currentSize == size {
		// Already at desired size
		return nil
	}

	if size > currentSize {
		// Scale up: create new instances
		return m.scaleUpAsg(asg, int(size-currentSize))
	} else {
		// Scale down: delete instances
		return m.scaleDownAsg(asg, int(currentSize-size))
	}
}

// GetAsgNodes returns node provider IDs for instances in the ASG
func (m *DatacrunchManager) GetAsgNodes(asg *Asg) ([]string, error) {
	instances, err := m.allAsgRunningInstances(asg.id)
	if err != nil {
		klog.Warningf("[DEBUG] Error getting instances for ASG %s nodes: %v", asg.id, err)
		return nil, err
	}
	klog.Infof("[DEBUG] GetAsgNodes for %s: processing %d instances", asg.id, len(instances))
	out := make([]string, 0, len(instances))
	for _, inst := range instances {
		// providerID format: datacrunch://<instance-id>/<hostname>
		providerID := datacrunchProviderIDPrefix + inst.ID
		if inst.Hostname != "" {
			providerID = providerID + "/" + inst.Hostname
		}
		klog.Infof("[DEBUG] ASG %s node: instanceID=%s, hostname=%s, providerID=%s", asg.id, inst.ID, inst.Hostname, providerID)
		out = append(out, providerID)
	}
	klog.Infof("[DEBUG] GetAsgNodes for %s: returning %d provider IDs", asg.id, len(out))
	return out, nil
}

// RegisterAsg registers asg in DataCrunch Manager.
func (m *DatacrunchManager) RegisterAsg(asg *Asg) {
	m.asgs.Register(asg)
}

// GetAsgForInstance returns ASG that owns the instance by matching Description
func (m *DatacrunchManager) GetAsgForInstanceByHostname(hostname string) (*Asg, error) {
	return m.asgs.FindForInstance(hostname)
}

// DeleteInstances deletes instances by ID from an ASG
func (m *DatacrunchManager) DeleteInstances(instanceIds []string) error {
	if len(instanceIds) == 0 {
		return nil
	}
	for _, id := range instanceIds {
		if id == "" {
			continue
		}
		_ = m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
			Action: instance.InstanceActionDelete,
			ID:     id,
		})
	}
	return nil
}

// getAsgTemplate returns template information for ASG
func (m *DatacrunchManager) getAsgTemplate(asgId string) (*asgTemplate, error) {
	// For now, find ASG from registry by iterating (can be optimized later)
	m.asgs.cacheMutex.Lock()
	defer m.asgs.cacheMutex.Unlock()

	var asg *Asg
	for _, asgInfo := range m.asgs.registeredAsgs {
		if asgInfo.config.id == asgId {
			asg = asgInfo.config
			break
		}
	}

	if asg == nil {
		return nil, errors.New("ASG not found: " + asgId)
	}

	instanceDetails, err := m.dcService.GetInstanceTypeDetails(asg.instanceType)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to get instance type details for ASG %s, type: %s, %v", asg.id, asg.instanceType, err)
		return nil, err
	}

	return &asgTemplate{
		InstanceType: instanceDetails,
		Location:     "",
		Tags:         make(map[string]string),
	}, nil
}

// buildNodeFromTemplate builds a Kubernetes node from ASG template
func (m *DatacrunchManager) buildNodeFromTemplate(asg *Asg, template *asgTemplate) (*apiv1.Node, error) {
	klog.Infof("[DEBUG] buildNodeFromTemplate for ASG %s: CPU=%d, Memory=%d, GPU=%d",
		asg.id, template.InstanceType.CPU, template.InstanceType.Memory, template.InstanceType.GPU)

	node := &apiv1.Node{}
	nodeName := fmt.Sprintf("%s-asg-%d", asg.id, rand.Int63())
	klog.Infof("[DEBUG] Generated template node name: %s", nodeName)

	node.ObjectMeta = metav1.ObjectMeta{
		Name: nodeName,
		Labels: map[string]string{
			"kubernetes.io/arch":               "amd64",
			"kubernetes.io/os":                 "linux",
			"node.kubernetes.io/instance-type": asg.instanceType,
			// "topology.kubernetes.io/location":  asg.location,
			"datacrunch.io/asg": asg.id,
		},
	}

	// Set node capacity and allocatable based on instance type
	memoryGi := template.InstanceType.Memory / 1024 / 1024 / 1024
	klog.Infof("[DEBUG] Setting node capacity: CPU=%d, Memory=%dGi, Pods=110", template.InstanceType.CPU, memoryGi)
	capacity := apiv1.ResourceList{
		apiv1.ResourcePods:   resource.MustParse("110"),
		apiv1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", template.InstanceType.CPU)),
		apiv1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", memoryGi)),
	}

	// Add GPU resources if available
	if template.InstanceType.GPU > 0 {
		klog.Infof("[DEBUG] Adding GPU resources: %d nvidia.com/gpu", template.InstanceType.GPU)
		capacity["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", template.InstanceType.GPU))
	} else {
		klog.Infof("[DEBUG] No GPU resources for instance type %s", asg.instanceType)
	}

	node.Status = apiv1.NodeStatus{
		Capacity:    capacity,
		Allocatable: capacity, // Simplified - in reality should account for system overhead
		Conditions:  cloudprovider.BuildReadyConditions(),
	}

	// Add custom labels from template tags
	for key, value := range template.Tags {
		node.Labels[key] = value
	}

	klog.Infof("[DEBUG] Template node created successfully for ASG %s: %s", asg.id, nodeName)
	return node, nil
}

// scaleUpAsg creates new instances for the ASG
func (m *DatacrunchManager) scaleUpAsg(asg *Asg, count int) error {
	if count <= 0 {
		return nil
	}

	klog.Infof("Scaling up ASG %s by %d instances", asg.id, count)

	// Get node configuration for this ASG
	nodeConfig, err := m.getNodeConfigForAsg(asg)
	if err != nil {
		return fmt.Errorf("failed to get node config for ASG %s: %v", asg.id, err)
	}

	// Validate we don't exceed max size
	currentSize, err := m.GetAsgSize(asg)
	if err != nil {
		return fmt.Errorf("failed to get current ASG size: %v", err)
	}

	if int(currentSize)+count > asg.maxSize {
		return fmt.Errorf("scaling up by %d would exceed max size %d (current: %d)",
			count, asg.maxSize, currentSize)
	}

	// Create instances one by one
	createdInstances := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// check if instance type is available in any of the locations
		location, err := m.instanceTypeAvailableLocation(asg.instanceType, asg.AvailabilityLocations)
		if err != nil {
			return fmt.Errorf("failed to check if instance type %s is available: %v", asg.instanceType, err)
		}
		instanceID, _, err := m.createInstanceForAsg(asg, nodeConfig, location)
		if err != nil {
			klog.Errorf("Failed to create instance %d for ASG %s: %v", i, asg.id, err)
			// Clean up any instances we've already created on failure
			m.cleanupCreatedInstances(createdInstances)
			return fmt.Errorf("failed to create instance %d: %v", i, err)
		}
		createdInstances = append(createdInstances, instanceID)
		klog.Infof("Successfully created instance %s for ASG %s", instanceID, asg.id)
	}

	klog.Infof("Successfully scaled up ASG %s by %d instances", asg.id, count)
	return nil
}

// scaleDownAsg deletes instances from the ASG
func (m *DatacrunchManager) scaleDownAsg(asg *Asg, count int) error {
	if count <= 0 {
		return nil
	}

	klog.Infof("Scaling down ASG %s by %d instances", asg.id, count)

	// Get current instances
	instances, err := m.dcService.GetAllInstancesByDescription(asg.id)
	if err != nil {
		return fmt.Errorf("failed to get instances for ASG %s: %v", asg.id, err)
	}

	if len(instances) < count {
		return fmt.Errorf("cannot delete %d instances, only %d available", count, len(instances))
	}

	// Validate we don't go below min size
	if len(instances)-count < asg.minSize {
		return fmt.Errorf("scaling down by %d would go below min size %d (current: %d)",
			count, asg.minSize, len(instances))
	}

	// Delete the requested number of instances
	deletedCount := 0
	for i := 0; i < count && i < len(instances); i++ {
		instanceID := instances[i].ID
		err := m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
			Action: instance.InstanceActionDelete,
			ID:     instanceID,
		})
		if err != nil {
			klog.Errorf("Failed to delete instance %s from ASG %s: %v", instanceID, asg.id, err)
			// Continue trying to delete other instances rather than failing completely
			continue
		}
		deletedCount++
		klog.Infof("Successfully deleted instance %s from ASG %s", instanceID, asg.id)
	}

	if deletedCount == 0 {
		return fmt.Errorf("failed to delete any instances from ASG %s", asg.id)
	}

	if deletedCount < count {
		klog.Warningf("Only deleted %d out of %d requested instances from ASG %s", deletedCount, count, asg.id)
	}

	klog.Infof("Successfully scaled down ASG %s by %d instances", asg.id, deletedCount)
	return nil
}

// getNodeConfigForAsg retrieves the node configuration for an ASG using global config
func (m *DatacrunchManager) getNodeConfigForAsg(asg *Asg) (*nodeConfig, error) {
	// Create a nodeConfig from global cloudConfig for this ASG
	nodeConfig := &nodeConfig{
		IsSpot:        false, // Default to non-spot
		Image:         "",    // Will be resolved based on instance type
		StartupScript: m.cfg.StartupScript,
		SSHKeyIDs:     m.cfg.SSHKeyIDs,
		OSVolumeSize:  100, // Default 100GB
		Labels:        make(map[string]string),
		Volumes:       make([]additionalVolume, len(m.cfg.AdditionalVolumes)),
		Taints:        make([]apiv1.Taint, len(m.cfg.Taints)),
		Contract:      m.cfg.BillingConfig.Contract,
		Price:         m.cfg.BillingConfig.Price,
	}

	// Copy additional volumes
	for i, vol := range m.cfg.AdditionalVolumes {
		nodeConfig.Volumes[i] = vol
	}

	// Copy taints
	for i, taint := range m.cfg.Taints {
		nodeConfig.Taints[i] = taint
	}

	// Determine labels based on instance type
	if isGPUInstanceType(asg.instanceType) {
		nodeConfig.Labels = m.cfg.Labels.GPU
	} else {
		nodeConfig.Labels = m.cfg.Labels.CPU
	}

	return nodeConfig, nil
}

// createInstanceForAsg creates a single instance for the ASG
func (m *DatacrunchManager) createInstanceForAsg(asg *Asg, nodeConfig *nodeConfig, location string) (string, string, error) {
	// Generate unique hostname
	// asgname + location + timestamp
	hostname := strings.ReplaceAll(fmt.Sprintf("%s-%s-%d", asg.id, location, time.Now().Unix()), ".", "-")
	klog.Infof("[DEBUG] createInstanceForAsg: ASG=%s, instanceType=%s, location=%s, hostname=%s",
		asg.id, asg.instanceType, location, hostname)

	// Create or get startup script ID
	providerID := fmt.Sprintf("datacrunch://%s/%s", location, asg.id)
	klog.Infof("[DEBUG] Creating startup script for ASG %s", asg.id)
	startupScriptID, err := m.createOrGetStartupScript(asg, nodeConfig, providerID)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to create startup script for ASG %s: %v", asg.id, err)
		return "", hostname, fmt.Errorf("failed to create startup script: %v", err)
	}
	klog.Infof("[DEBUG] Startup script created with ID: '%s'", startupScriptID)

	// Check if startup script ID is empty
	if startupScriptID == "" {
		klog.Errorf("[DEBUG] ❌ Startup script creation returned empty ID - cannot proceed with instance creation")
		return "", hostname, fmt.Errorf("startup script creation returned empty ID")
	}

	// Determine image to use based on instance type
	var image string
	isGPU := isGPUInstanceType(asg.instanceType)
	klog.Infof("[DEBUG] Instance type %s detected as GPU: %t", asg.instanceType, isGPU)
	if isGPU {
		image = m.cfg.Image.GPU
		if image == "" {
			klog.Errorf("[DEBUG] No GPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no GPU image configured for instance type %s", asg.instanceType)
		}
		klog.Infof("[DEBUG] Using GPU image: %s", image)
	} else {
		image = m.cfg.Image.CPU
		if image == "" {
			klog.Errorf("[DEBUG] No CPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no CPU image configured for instance type %s", asg.instanceType)
		}
		klog.Infof("[DEBUG] Using CPU image: %s", image)
	}

	// Create the instance input with all required fields
	input := instance.CreateInstanceInput{
		InstanceType:    asg.instanceType,
		Image:           image,
		SSHKeyIDs:       nodeConfig.SSHKeyIDs, // Required
		StartupScriptID: startupScriptID,      // Required - now properly created
		Hostname:        hostname,
		Description:     asg.id, // Use ASG ID as description for grouping
		LocationCode:    location,
		IsSpot:          nodeConfig.IsSpot,
		Contract:        nodeConfig.Contract, // Required from billing config
		Pricing:         nodeConfig.Price,    // Required from billing config
	}

	// Add OS volume - always add since we set default size in validation
	input.OSVolume = &instance.OSVolume{
		Name: fmt.Sprintf("%s-os-volume", hostname),
		Size: int64(nodeConfig.OSVolumeSize),
	}

	// Add additional volumes if specified
	if len(nodeConfig.Volumes) > 0 {
		volumes := make([]instance.Volume, len(nodeConfig.Volumes))
		for i, vol := range nodeConfig.Volumes {
			volumes[i] = instance.Volume{
				Name: vol.Name,
				Size: int64(vol.Size),
				Type: vol.Type,
			}
		}
		input.Volumes = volumes
	}

	// Create the instance
	klog.Infof("[DEBUG] Creating DataCrunch instance: hostname=%s, type=%s, image=%s, location=%s, contract=%s, pricing=%s",
		hostname, asg.instanceType, image, location, m.cfg.BillingConfig.Contract, m.cfg.BillingConfig.Price)

	// Debug: Log full request body
	if requestBody, err := json.MarshalIndent(input, "", "  "); err == nil {
		klog.Infof("[DEBUG] CreateInstance request body:\n%s", string(requestBody))
	}

	instanceID, err := m.dcService.CreateInstance(&input)
	if err != nil {
		klog.Errorf("[DEBUG] DataCrunch API call failed for instance %s: %v", hostname, err)
		return "", hostname, fmt.Errorf("failed to create instance: %v", err)
	}

	klog.Infof("[DEBUG] DataCrunch instance created successfully: ID=%s, hostname=%s", instanceID, hostname)
	return instanceID, hostname, nil
}

// isGPUInstanceType determines if an instance type is GPU-based
func isGPUInstanceType(instanceType string) bool {
	// Common GPU instance type patterns
	// instanceType start with "CPU." will be CPU others will be GPU
	return !strings.HasPrefix(strings.ToUpper(instanceType), "CPU.")
}

// createOrGetStartupScript creates a startup script or returns existing ID if already created
func (m *DatacrunchManager) createOrGetStartupScript(asg *Asg, nodeConfig *nodeConfig, providerID string) (string, error) {
	scriptName := fmt.Sprintf("autoscaler-%s", asg.id)
	// Create new startup script
	klog.Infof("Creating new startup script %s", scriptName)
	// decode the base64 and stringify to text utf8 encoded  new line "\n"
	_base64Script, err := base64.StdEncoding.DecodeString(nodeConfig.StartupScript)
	if err != nil {
		return "", fmt.Errorf("failed to decode startup script: %v", err)
	}

	// patch the script
	startupScriptEnv := m.cfg.StartupScriptEnv
	startupScriptEnv["PROVIDER_ID"] = providerID
	_patchedBase64Script := patchScript(_base64Script, startupScriptEnv)

	// stringify the script
	_scriptsUtf8 := string(_patchedBase64Script)

	input := &startscripts.CreateStartScriptInput{
		Name:   scriptName,
		Script: _scriptsUtf8,
	}
	if inputJSON, err := json.MarshalIndent(input, "", "  "); err == nil {
		klog.Infof("[DEBUG] CreateStartScript request body:\n%s", string(inputJSON))
	}

	scriptID, err := m.dcService.CreateStartScript(input)
	if err != nil {
		klog.Errorf("[DEBUG] CreateStartScript API call failed: %v", err)
		return "", fmt.Errorf("failed to create startup script: %v", err)
	}

	klog.Infof("[DEBUG] CreateStartScript response - scriptID: '%s'", scriptID)

	klog.Infof("Created startup script %s with ID %s", scriptName, scriptID)
	return scriptID, nil
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

func (m *DatacrunchManager) instanceTypeAvailableLocation(instanceType string, locations []string) (string, error) {
	klog.Infof("[DEBUG] Checking instance availability: instanceType=%s, locations=%s", instanceType, locations)
	location, err := m.dcService.GetInstanceAvailabilityLocation(instanceType, locations)
	if err != nil {
		klog.Errorf("[DEBUG] Error checking availability for %s in %s: %v", instanceType, locations, err)
		return "", err
	}
	klog.Infof("[DEBUG] Instance type %s is available in location %s", instanceType, location)
	return location, nil
}

func (m *DatacrunchManager) getAvailableMachineTypes() ([]string, error) {
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
