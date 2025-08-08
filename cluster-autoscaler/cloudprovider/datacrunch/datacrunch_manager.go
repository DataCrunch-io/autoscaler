package datacrunch

import (
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
	klog "k8s.io/klog/v2"
)

const (
	datacrunchProviderIDPrefix = "datacrunch://"
	GPULabel                   = "datacrunch.io/deployment-name"
)

type DatacrunchManager struct {
	cfg         *cloudConfig
	sdkProvider *datacrunchSDKProvider
	dcService   *datacrunchWrapper
	asgs        *autoScalingGroups
}

// asgTemplate holds template information for creating nodes
type asgTemplate struct {
	InstanceType *InstanceType
	Region       string
	Tags         map[string]string
}

// InstanceType holds instance type information
type InstanceType struct {
	CPU    int64
	Memory int64
	GPU    int64
}

func createDatacrunchManager(cloudReader io.Reader) (*DatacrunchManager, error) {
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
	sdkProvider, err := createDatacrunchSDKProvider()
	if err != nil {
		return nil, err
	}

	// create the datacrunch wrapper
	dcService := &datacrunchWrapper{
		instance.New(sdkProvider.session),
		instancetypes.New(sdkProvider.session),
		startscripts.New(sdkProvider.session),
		newCustomInstanceAvailability(sdkProvider.session),
	}

	manager := &DatacrunchManager{
		cfg:         cfg,
		sdkProvider: sdkProvider,
		dcService:   dcService,
		asgs:        nil, // Will be set after creation
	}

	// Initialize ASG registry
	manager.asgs = newAutoScalingGroups(manager)

	return manager, nil
}

// Refresh updates manager state before each main loop
func (m *DatacrunchManager) Refresh() error {
	return nil
}

// allInstances returns all instances that belong to a given logical ASG (matched by Description)
func (m *DatacrunchManager) allInstances(asgName string) ([]*instance.ListInstancesResponse, error) {
	if m.dcService == nil || m.dcService.instanceI == nil {
		return nil, nil
	}
	instances, err := m.dcService.ListInstances()
	if err != nil {
		return nil, err
	}
	if asgName == "" {
		return instances, nil
	}
	filtered := make([]*instance.ListInstancesResponse, 0, len(instances))
	for _, inst := range instances {
		if inst != nil && inst.Description == asgName {
			filtered = append(filtered, inst)
		}
	}
	return filtered, nil
}

// GetAsgSize returns size for a given ASG by counting instances with matching Description
func (m *DatacrunchManager) GetAsgSize(asg *Asg) (int64, error) {
	if asg == nil {
		return 0, nil
	}
	instances, err := m.allInstances(asg.id)
	if err != nil {
		return -1, err
	}
	return int64(len(instances)), nil
}

// // SetAsgSize sets desired group size by creating or deleting instances.
// func (m *DatacrunchManager) SetAsgSize(asg *Asg, size int64) error {
// 	if asg == nil {
// 		return nil
// 	}
// 	if size < 0 {
// 		return errors.New("size must be non-negative")
// 	}

// 	// Get current size
// 	currentSize, err := m.GetAsgSize(asg)
// 	if err != nil {
// 		return fmt.Errorf("failed to get current ASG size: %v", err)
// 	}

// 	if currentSize == size {
// 		// Already at desired size
// 		return nil
// 	}

// 	if size > currentSize {
// 		// Scale up: create new instances
// 		return m.scaleUpAsg(asg, int(size-currentSize))
// 	} else {
// 		// Scale down: delete instances
// 		return m.scaleDownAsg(asg, int(currentSize-size))
// 	}
// }

// GetAsgNodes returns node provider IDs for instances in the ASG
func (m *DatacrunchManager) GetAsgNodes(asg *Asg) ([]string, error) {
	instances, err := m.allInstances(asg.id)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		// providerID format: datacrunch://<instance-id>/<hostname>
		providerID := datacrunchProviderIDPrefix + inst.ID
		if inst.Hostname != "" {
			providerID = providerID + "/" + inst.Hostname
		}
		out = append(out, providerID)
	}
	return out, nil
}

// RegisterAsg registers asg in DataCrunch Manager.
func (m *DatacrunchManager) RegisterAsg(asg *Asg) {
	m.asgs.Register(asg)
}

// GetAsgForInstance returns ASG that owns the instance by matching Description
func (m *DatacrunchManager) GetAsgForInstance(instanceId string) (*Asg, error) {
	return m.asgs.FindForInstance(instanceId)
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

	// Get instance type information
	instanceType, err := m.dcService.GetInstanceType(asg.instanceType)
	if err != nil {
		return nil, err
	}

	return &asgTemplate{
		InstanceType: instanceType,
		Region:       asg.locationCode,
		Tags:         make(map[string]string),
	}, nil
}

// buildNodeFromTemplate builds a Kubernetes node from ASG template
func (m *DatacrunchManager) buildNodeFromTemplate(asg *Asg, template *asgTemplate) (*apiv1.Node, error) {

	node := &apiv1.Node{}
	nodeName := fmt.Sprintf("%s-asg-%d", asg.id, rand.Int63())

	node.ObjectMeta = metav1.ObjectMeta{
		Name: nodeName,
		Labels: map[string]string{
			"kubernetes.io/arch":               "amd64",
			"kubernetes.io/os":                 "linux",
			"node.kubernetes.io/instance-type": asg.instanceType,
			"topology.kubernetes.io/zone":      asg.locationCode,
			"datacrunch.io/asg":                asg.id,
		},
	}

	// Set node capacity and allocatable based on instance type
	capacity := apiv1.ResourceList{
		apiv1.ResourcePods:   resource.MustParse("110"),
		apiv1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", template.InstanceType.CPU)),
		apiv1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", template.InstanceType.Memory/1024/1024/1024)),
	}

	// Add GPU resources if available
	if template.InstanceType.GPU > 0 {
		capacity["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", template.InstanceType.GPU))
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

	return node, nil
}

// // scaleUpAsg creates new instances for the ASG
// func (m *DatacrunchManager) scaleUpAsg(asg *Asg, count int) error {
// 	if count <= 0 {
// 		return nil
// 	}

// 	klog.V(2).Infof("Scaling up ASG %s by %d instances", asg.id, count)

// 	// Get node configuration for this ASG
// 	nodeConfig, err := m.getNodeConfigForAsg(asg)
// 	if err != nil {
// 		return fmt.Errorf("failed to get node config for ASG %s: %v", asg.id, err)
// 	}

// 	// Validate we don't exceed max size
// 	currentSize, err := m.GetAsgSize(asg)
// 	if err != nil {
// 		return fmt.Errorf("failed to get current ASG size: %v", err)
// 	}

// 	if int(currentSize)+count > asg.maxSize {
// 		return fmt.Errorf("scaling up by %d would exceed max size %d (current: %d)",
// 			count, asg.maxSize, currentSize)
// 	}

// 	// Create instances one by one
// 	createdInstances := make([]string, 0, count)
// 	for i := 0; i < count; i++ {
// 		instanceID, err := m.createInstanceForAsg(asg, nodeConfig, i)
// 		if err != nil {
// 			klog.Errorf("Failed to create instance %d for ASG %s: %v", i, asg.id, err)
// 			// Clean up any instances we've already created on failure
// 			m.cleanupCreatedInstances(createdInstances)
// 			return fmt.Errorf("failed to create instance %d: %v", i, err)
// 		}
// 		createdInstances = append(createdInstances, instanceID)
// 		klog.V(3).Infof("Successfully created instance %s for ASG %s", instanceID, asg.id)
// 	}

// 	klog.V(2).Infof("Successfully scaled up ASG %s by %d instances", asg.id, count)
// 	return nil
// }

// scaleDownAsg deletes instances from the ASG
// func (m *DatacrunchManager) scaleDownAsg(asg *Asg, count int) error {
// 	if count <= 0 {
// 		return nil
// 	}

// 	klog.V(2).Infof("Scaling down ASG %s by %d instances", asg.id, count)

// 	// Get current instances
// 	instances, err := m.allInstances(asg.id)
// 	if err != nil {
// 		return fmt.Errorf("failed to get instances for ASG %s: %v", asg.id, err)
// 	}

// 	if len(instances) < count {
// 		return fmt.Errorf("cannot delete %d instances, only %d available", count, len(instances))
// 	}

// 	// Validate we don't go below min size
// 	if len(instances)-count < asg.minSize {
// 		return fmt.Errorf("scaling down by %d would go below min size %d (current: %d)",
// 			count, asg.minSize, len(instances))
// 	}

// 	// Delete the requested number of instances
// 	deletedCount := 0
// 	for i := 0; i < count && i < len(instances); i++ {
// 		instanceID := instances[i].ID
// 		err := m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
// 			Action: instance.InstanceActionDelete,
// 			ID:     instanceID,
// 		})
// 		if err != nil {
// 			klog.Errorf("Failed to delete instance %s from ASG %s: %v", instanceID, asg.id, err)
// 			// Continue trying to delete other instances rather than failing completely
// 			continue
// 		}
// 		deletedCount++
// 		klog.V(3).Infof("Successfully deleted instance %s from ASG %s", instanceID, asg.id)
// 	}

// 	if deletedCount == 0 {
// 		return fmt.Errorf("failed to delete any instances from ASG %s", asg.id)
// 	}

// 	if deletedCount < count {
// 		klog.Warningf("Only deleted %d out of %d requested instances from ASG %s", deletedCount, count, asg.id)
// 	}

// 	klog.V(2).Infof("Successfully scaled down ASG %s by %d instances", asg.id, deletedCount)
// 	return nil
// }

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
func (m *DatacrunchManager) createInstanceForAsg(asg *Asg, nodeConfig *nodeConfig, instanceIndex int) (string, error) {
	// Generate unique hostname
	hostname := fmt.Sprintf("%s-%d-%d", asg.id, time.Now().Unix(), instanceIndex)

	// Create or get startup script ID
	startupScriptID, err := m.createOrGetStartupScript(asg, nodeConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create startup script: %v", err)
	}

	// Determine image to use based on instance type
	var image string
	if isGPUInstanceType(asg.instanceType) {
		image = m.cfg.Image.GPU
		if image == "" {
			return "", fmt.Errorf("no GPU image configured for instance type %s", asg.instanceType)
		}
	} else {
		image = m.cfg.Image.CPU
		if image == "" {
			return "", fmt.Errorf("no CPU image configured for instance type %s", asg.instanceType)
		}
	}

	// Create the instance input with all required fields
	input := &instance.CreateInstanceInput{
		InstanceType:    asg.instanceType,
		Image:           image,
		SSHKeyIDs:       nodeConfig.SSHKeyIDs, // Required
		StartupScriptID: startupScriptID,      // Required - now properly created
		Hostname:        hostname,
		Description:     asg.id, // Use ASG ID as description for grouping
		LocationCode:    asg.locationCode,
		IsSpot:          nodeConfig.IsSpot,
		Contract:        nodeConfig.Contract, // Required from billing config
		Pricing:         nodeConfig.Price,    // Required from billing config
	}

	// Add OS volume - always add since we set default size in validation
	input.OSVolume = &instance.OSVolume{
		Name: fmt.Sprintf("%s-os-volume", hostname),
		Size: nodeConfig.OSVolumeSize,
	}

	// Add additional volumes if specified
	if len(nodeConfig.Volumes) > 0 {
		volumes := make([]instance.Volume, len(nodeConfig.Volumes))
		for i, vol := range nodeConfig.Volumes {
			volumes[i] = instance.Volume{
				Name: vol.Name,
				Size: vol.Size,
				Type: vol.Type,
			}
		}
		input.Volumes = volumes
	}

	// Create the instance
	klog.V(3).Infof("Creating instance %s with type %s, image %s, contract %s, pricing %s",
		hostname, asg.instanceType, image, m.cfg.BillingConfig.Contract, m.cfg.BillingConfig.Price)

	instanceID, err := m.dcService.CreateInstance(input)
	if err != nil {
		return "", fmt.Errorf("failed to create instance: %v", err)
	}

	return instanceID, nil
}

// isGPUInstanceType determines if an instance type is GPU-based
func isGPUInstanceType(instanceType string) bool {
	// Common GPU instance type patterns
	gpuPatterns := []string{"L40S", "A40", "A6000", "V100", "T4", "RTX", "gpu", "GPU"}
	instanceTypeUpper := strings.ToUpper(instanceType)

	for _, pattern := range gpuPatterns {
		if strings.Contains(instanceTypeUpper, strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}

// createOrGetStartupScript creates a startup script or returns existing ID if already created
func (m *DatacrunchManager) createOrGetStartupScript(asg *Asg, nodeConfig *nodeConfig) (string, error) {
	scriptName := fmt.Sprintf("autoscaler-%s", asg.id)

	// Try to find existing script first
	scripts, err := m.dcService.ListStartScripts()
	if err != nil {
		return "", fmt.Errorf("failed to list startup scripts: %v", err)
	}

	for _, script := range scripts {
		if script.Name == scriptName {
			klog.V(3).Infof("Found existing startup script %s with ID %s", scriptName, script.ID)
			return script.ID, nil
		}
	}

	// Create new startup script
	klog.V(3).Infof("Creating new startup script %s", scriptName)
	scriptID, err := m.dcService.CreateStartScript(&startscripts.CreateStartScriptInput{
		Name:   scriptName,
		Script: nodeConfig.StartupScript, // This is base64 encoded
	})
	if err != nil {
		return "", fmt.Errorf("failed to create startup script: %v", err)
	}

	klog.V(3).Infof("Created startup script %s with ID %s", scriptName, scriptID)
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
			klog.V(3).Infof("Cleaned up instance %s", instanceID)
		}
	}
}

func (m *DatacrunchManager) instanceTypeAvailable(instanceType string, locationCode string) (bool, error) {
	return m.dcService.CheckInstanceAvailability(instanceType, locationCode)
}
