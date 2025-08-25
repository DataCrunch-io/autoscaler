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
	"reflect"
	"strings"
	"sync"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/startscripts"
	klog "k8s.io/klog/v2"
)

const (
	// ASG_SEPARATOR_MAGIC_NUMBER is used as a separator in hostnames to reliably extract ASG names
	// Format: {asg-name}-{ASG_SEPARATOR_MAGIC_NUMBER}-{location}-{timestamp}
	ASG_SEPARATOR_MAGIC_NUMBER = "77"
)

var (
	// ASG_SEPARATOR is the full separator pattern used in hostnames
	ASG_SEPARATOR = fmt.Sprintf("-%s-", ASG_SEPARATOR_MAGIC_NUMBER)
)

type autoScalingGroups struct {
	registeredAsgs    map[AsgRef]*Asg
	asgToInstances    map[AsgRef][]InstanceRef
	instanceToAsg     map[InstanceRef]*Asg
	asgNodeGroupSpecs map[AsgRef]string
	cfg               *cloudConfig
	dcService         *datacrunchWrapper

	cacheMutex sync.Mutex
}

type asgInformation struct {
	config *Asg
}

func newAutoScalingGroups(dcService *datacrunchWrapper, nodeGroupSpecs []string, cfg *cloudConfig) (*autoScalingGroups, error) {
	registry := &autoScalingGroups{
		registeredAsgs:    make(map[AsgRef]*Asg),
		asgToInstances:    make(map[AsgRef][]InstanceRef),
		instanceToAsg:     make(map[InstanceRef]*Asg),
		asgNodeGroupSpecs: make(map[AsgRef]string),
		cfg:               cfg,
		dcService:         dcService,
	}

	if err := registry.parseASGNodeGroupSpecs(nodeGroupSpecs); err != nil {
		return nil, err
	}

	return registry, nil
}

func (m *autoScalingGroups) getAsgs() []*Asg {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	asgs := make([]*Asg, 0, len(m.registeredAsgs))
	for _, asg := range m.registeredAsgs {
		asgs = append(asgs, asg)
	}
	return asgs
}

func (m *autoScalingGroups) GetAsgByRef(ref AsgRef) (*Asg, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	asg, exists := m.registeredAsgs[ref]
	if !exists {
		return nil, fmt.Errorf("ASG not found for ref: %s", ref.Name)
	}
	return asg, nil
}

// Register registers asg in DataCrunch Manager.
func (m *autoScalingGroups) Register(asg *Asg) *Asg {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if existing, asgExists := m.registeredAsgs[asg.AsgRef]; asgExists {
		if reflect.DeepEqual(existing, asg) {
			return existing
		}

		// check the node group spec for the asg
		if _, asgExists := m.asgNodeGroupSpecs[asg.AsgRef]; !asgExists {
			existing.minSize = asg.minSize
			existing.maxSize = asg.maxSize
			existing.instanceType = asg.instanceType
		}

		existing.AvailabilityLocations = asg.AvailabilityLocations
		return existing
	}

	m.registeredAsgs[asg.AsgRef] = asg
	return asg
}

func (m *autoScalingGroups) Unregister(asg *Asg) {
	if _, asgExists := m.registeredAsgs[asg.AsgRef]; asgExists {
		delete(m.registeredAsgs, asg.AsgRef)
	}
}

// FindASGForInstance returns Asg of the given Instance
func (m *autoScalingGroups) FindASGForInstance(ref *InstanceRef) (*Asg, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// First try exact match
	if asg, asgExists := m.instanceToAsg[*ref]; asgExists {
		return asg, nil
	}

	// If exact match fails, try finding by hostname
	// This handles cases where InstanceRef might have different ProviderID values
	for cachedRef, asg := range m.instanceToAsg {
		if cachedRef.Hostname == ref.Hostname {
			klog.Infof("[DEBUG] Found ASG %s for hostname %s via hostname lookup", asg.Name, ref.Hostname)
			return asg, nil
		}
	}

	return nil, fmt.Errorf("ASG not found for hostname: %s", ref.Hostname)
}

func (m *autoScalingGroups) regenerate() error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	klog.Infof("[DEBUG] regenerate() called with %d asgNodeGroupSpecs", len(m.asgNodeGroupSpecs))
	for ref, spec := range m.asgNodeGroupSpecs {
		klog.Infof("[DEBUG] ASG spec in regenerate: %s -> %s", ref.Name, spec)
	}

	newInstanceToAsg := make(map[InstanceRef]*Asg)
	newAsgToInstances := make(map[AsgRef][]InstanceRef)
	newRegisteredAsgs := make(map[AsgRef]*Asg)

	for _, spec := range m.asgNodeGroupSpecs {
		klog.Infof("[DEBUG] Processing spec in regenerate: %s", spec)
		asg, err := m.buildASGFromSpec(spec)
		if err != nil {
			klog.Errorf("failed to build ASG from spec: %v", err)
			continue
		}
		klog.Infof("[DEBUG] Built ASG %s from spec in regenerate", asg.Name)
		newRegisteredAsgs[asg.AsgRef] = asg
		// get all instances for the asg using hostname-based matching (more reliable)
		klog.Infof("[DEBUG] About to call GetAllInstancesByAsgName for ASG %s", asg.Name)
		deployedInstances, err := m.dcService.GetAllInstancesByAsgName(asg.Name)
		if err != nil {
			klog.Errorf("failed to get instances for ASG %s: %v", asg.Name, err)
			continue
		}
		klog.Infof("[DEBUG] Found %d instances for ASG %s", len(deployedInstances), asg.Name)
		for _, deployedInstance := range deployedInstances {
			// Create consistent InstanceRef with both hostname and provider ID
			providerID := datacrunchProviderIDPrefix + deployedInstance.Location + "/" + deployedInstance.Hostname
			instanceRef := InstanceRef{
				Hostname:   deployedInstance.Hostname,
				ProviderID: providerID,
			}
			newInstanceToAsg[instanceRef] = asg
			newAsgToInstances[asg.AsgRef] = append(newAsgToInstances[asg.AsgRef], instanceRef)
		}
		// update current size
		asg.curSize = len(deployedInstances)
		klog.Infof("[DEBUG] Set ASG %s curSize to %d based on discovered instances", asg.Name, asg.curSize)
	}

	// Unregister no longer existing Node Groups specs
	for _, asg := range m.registeredAsgs {
		if _, asgExists := newRegisteredAsgs[asg.AsgRef]; !asgExists {
			if _, asgExists := m.asgNodeGroupSpecs[asg.AsgRef]; !asgExists {
				klog.Infof("[DEBUG] Unregistering ASG %s", asg.Name)
				m.Unregister(asg)
			}
		}
	}

	m.registeredAsgs = newRegisteredAsgs
	m.instanceToAsg = newInstanceToAsg
	m.asgToInstances = newAsgToInstances

	klog.Infof("[DEBUG] regenerate() finished with %d registered ASGs", len(m.registeredAsgs))
	for ref := range m.registeredAsgs {
		klog.Infof("[DEBUG] Registered ASG after regenerate: %s", ref.Name)
	}

	return nil
}

func (m *autoScalingGroups) buildASGFromSpec(spec string) (*Asg, error) {
	asgSpec, err := parseAsgSpec(spec)
	if err != nil {
		return nil, err
	}

	asg := &Asg{
		AsgRef:                AsgRef{Name: asgSpec.name},
		minSize:               asgSpec.minSize,
		maxSize:               asgSpec.maxSize,
		instanceType:          asgSpec.instanceType,
		AvailabilityLocations: m.cfg.AvailableLocations,
	}
	return asg, nil
}

func (m *autoScalingGroups) parseASGNodeGroupSpecs(specs []string) error {
	klog.Infof("[DEBUG] parseASGNodeGroupSpecs called with %d specs", len(specs))
	for _, spec := range specs {
		klog.Infof("[DEBUG] Processing spec: %s", spec)
		asg, err := m.buildASGFromSpec(spec)
		if err != nil {
			return err
		}
		klog.Infof("[DEBUG] Registering ASG %s", asg.Name)
		registeredAsg := m.Register(asg)
		klog.Infof("[DEBUG] ASG %s registered successfully", registeredAsg.Name)

		// Store the spec for this ASG
		m.asgNodeGroupSpecs[asg.AsgRef] = spec
		klog.Infof("[DEBUG] Stored spec for ASG %s: %s", asg.Name, spec)
	}
	klog.Infof("[DEBUG] parseASGNodeGroupSpecs finished, total asgNodeGroupSpecs: %d", len(m.asgNodeGroupSpecs))
	return nil
}

func (m *autoScalingGroups) scaleUpAsg(asg *Asg, delta int) error {
	klog.Infof("increase ASG:%s with %d nodes", asg.Name, delta)
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size := asg.curSize
	if int(size)+delta > asg.maxSize {
		return fmt.Errorf("size increase is too large - desired:%d max:%d", int(size)+delta, asg.maxSize)
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// check if required resources are available
	location, err := m.dcService.GetInstanceAvailabilityLocation(asg.instanceType, asg.AvailabilityLocations)
	if err != nil {
		return fmt.Errorf("failed to check if instance type %s is available: %v", asg.instanceType, err)
	}
	if location == "" {
		return fmt.Errorf("instance type %s is not available in any of the locations", asg.instanceType)
	}
	klog.Infof("instance type %s is available in location %s", asg.instanceType, location)

	// prepare node config for each server
	nodeConfig, err := m.getNodeConfigForAsg(asg)
	if err != nil {
		return fmt.Errorf("failed to get node config for ASG %s: %v", asg.Name, err)
	}

	// datcrunch doesnt support group server creation, we need to create each server manually
	// we need concurrent like create server and if error then need reduce actual delta
	waitGroup := sync.WaitGroup{}
	errsCh := make(chan error, delta)
	for i := 0; i < delta; i++ {
		waitGroup.Add(1)
		go func(index int, location string) {
			defer waitGroup.Done()
			klog.Infof("[DEBUG] Creating instance %d/%d for ASG %s", index+1, delta, asg.Name)
			_, hostname, err := m.createInstanceForAsg(asg, nodeConfig, location)
			if err != nil {
				klog.Errorf("[DEBUG] Failed to create instance %d for ASG %s: %v", index+1, asg.Name, err)
				errsCh <- err
			} else {
				// update cache with consistent InstanceRef including provider ID
				providerID := datacrunchProviderIDPrefix + location + "/" + hostname
				instanceRef := InstanceRef{
					Hostname:   hostname,
					ProviderID: providerID,
				}
				m.instanceToAsg[instanceRef] = asg
				m.asgToInstances[asg.AsgRef] = append(m.asgToInstances[asg.AsgRef], instanceRef)
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

	// Update curSize to reflect successful creations
	// Note: curSize will be refreshed from API in next regenerate() call,
	// but we update it here for immediate consistency
	successfulCreations := delta - len(errs)
	asg.curSize += successfulCreations
	klog.Infof("[DEBUG] Updated ASG %s curSize to %d after creating %d instances",
		asg.Name, asg.curSize, successfulCreations)

	return nil
}

// getNodeConfigForAsg retrieves the node configuration for an ASG using global config
func (m *autoScalingGroups) getNodeConfigForAsg(asg *Asg) (*nodeConfig, error) {
	// Create a nodeConfig from global cloudConfig for this ASG
	isGPU := isGPUInstanceType(asg.instanceType)
	var image string
	if isGPU {
		image = m.cfg.Image.GPU
	} else {
		image = m.cfg.Image.CPU
	}

	nodeConfig := &nodeConfig{
		IsSpot:        m.cfg.BillingConfig.Contract == string(instance.BillingContractSpot), // Default to non-spot
		Image:         image,
		StartupScript: m.cfg.StartupScript,
		SSHKeyIDs:     m.cfg.SSHKeyIDs,
		OSVolumeSize:  100, // Default 100GB
		Labels:        m.cfg.Labels,
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

	return nodeConfig, nil
}

// createInstanceForAsg creates a single instance for the ASG
func (m *autoScalingGroups) createInstanceForAsg(asg *Asg, nodeConfig *nodeConfig, location string) (string, string, error) {
	// Generate unique hostname with magic separator
	// Format: {asg-name}-{magic-number}-{location}-{timestamp}
	hostname := strings.ReplaceAll(fmt.Sprintf("%s%s%s-%d", asg.Name, ASG_SEPARATOR, location, time.Now().Unix()), ".", "-")
	// Create or get startup script ID
	providerID := fmt.Sprintf("datacrunch://%s/%s", location, hostname)
	startupScriptID, err := m.createOrGetStartupScript(asg, nodeConfig, providerID)
	if err != nil {
		klog.Errorf("[DEBUG] Failed to create startup script for ASG %s: %v", asg.Name, err)
		return "", hostname, err
	}

	// Check if startup script ID is empty
	if startupScriptID == "" {
		klog.Errorf("[DEBUG] Startup script creation returned empty ID - cannot proceed with instance creation")
		return "", hostname, errors.New("startup script creation returned empty ID")
	}

	defer func() {
		// clean up the starup script if the id is valid
		if startupScriptID != "" {
			m.dcService.DeleteStartScript(startupScriptID)
		}
	}()

	// Determine image to use based on instance type
	var image string
	isGPU := isGPUInstanceType(asg.instanceType)
	if isGPU {
		image = m.cfg.Image.GPU
		if image == "" {
			klog.Errorf("[DEBUG] No GPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no GPU image configured for instance type %s", asg.instanceType)
		}
	} else {
		image = m.cfg.Image.CPU
		if image == "" {
			klog.Errorf("[DEBUG] No CPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no CPU image configured for instance type %s", asg.instanceType)
		}
	}

	// Create the instance input with all required fields
	input := instance.CreateInstanceInput{
		InstanceType:    asg.instanceType,
		Image:           image,
		SSHKeyIDs:       nodeConfig.SSHKeyIDs, // Required
		StartupScriptID: startupScriptID,      // Required - now properly created
		Hostname:        hostname,
		Description:     asg.Name, // Use ASG ID as description for grouping
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

	// TODO: Delete before official release
	// Debug: Log full request body
	if requestBody, err := json.MarshalIndent(input, "", "  "); err == nil {
		klog.Infof("[DEBUG] CreateInstance request body:\n%s", string(requestBody))
	}

	instanceID, err := m.dcService.CreateInstance(&input)
	if err != nil {
		klog.Errorf("[DEBUG] DataCrunch API call failed for instance %s: %v", hostname, err)
		return "", hostname, fmt.Errorf("failed to create instance: %v", err)
	}

	return instanceID, hostname, nil
}

// createOrGetStartupScript creates a startup script or returns existing ID if already created
func (m *autoScalingGroups) createOrGetStartupScript(asg *Asg, nodeConfig *nodeConfig, providerID string) (string, error) {
	scriptName := fmt.Sprintf("as-%s", asg.Name)
	// decode the base64 and stringify to text utf8 encoded  new line "\n"
	_base64Script, err := base64.StdEncoding.DecodeString(nodeConfig.StartupScript)
	if err != nil {
		return "", fmt.Errorf("failed to decode startup script: %v", err)
	}

	// patch the script
	startupScriptEnv := m.cfg.StartupScriptEnv
	startupScriptEnv["PROVIDER_ID"] = providerID
	labels := convertConfigLabelsToK8sLabels(nodeConfig.Labels, asg)
	startupScriptEnv["LABELS"] = labels

	// patch the script
	_patchedBase64Script := patchScript(_base64Script, startupScriptEnv)
	// stringify the script
	_scriptsUtf8 := string(_patchedBase64Script)

	input := &startscripts.CreateStartScriptInput{
		Name:   scriptName,
		Script: _scriptsUtf8,
	}

	scriptID, err := m.dcService.CreateStartScript(input)
	if err != nil {
		klog.Errorf("[DEBUG] CreateStartScript API call failed: %v", err)
		return "", fmt.Errorf("failed to create startup script: %v", err)
	}

	return scriptID, nil
}

func (m *autoScalingGroups) scaleDownAsg(asg *Asg, count int) error {
	if count <= 0 {
		return nil
	}

	klog.Infof("Scaling down ASG %s by %d instances", asg.Name, count)

	// Get current instances using hostname-based matching
	instances, err := m.dcService.GetAllInstancesByAsgName(asg.Name)
	if err != nil {
		return fmt.Errorf("failed to get instances for ASG %s: %v", asg.Name, err)
	}

	if len(instances) < count {
		return fmt.Errorf("cannot delete %d instances, only %d available", count, len(instances))
	}

	// Validate we don't go below min size
	if len(instances)-count < asg.minSize {
		return fmt.Errorf("scaling down by %d would go below min size %d (current: %d)",
			count, asg.minSize, len(instances))
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	wg := sync.WaitGroup{}
	errsCh := make(chan error, count)

	// Collect instances to delete and their IDs
	instancesToDelete := make([]struct {
		ID       string
		Hostname string
	}, 0, count)

	for i := 0; i < count && i < len(instances); i++ {
		instancesToDelete = append(instancesToDelete, struct {
			ID       string
			Hostname string
		}{
			ID:       instances[i].ID,
			Hostname: instances[i].Hostname,
		})
	}

	// Delete instances concurrently
	for _, inst := range instancesToDelete {
		wg.Add(1)
		go func(instanceID, hostname string) {
			defer wg.Done()
			err := m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
				Action: instance.InstanceActionDelete,
				ID:     instanceID,
			})
			if err != nil {
				klog.Errorf("Failed to delete instance %s: %v", instanceID, err)
				errsCh <- err
			} else {
				klog.Infof("Successfully deleted instance %s from ASG %s", instanceID, asg.Name)
			}
		}(inst.ID, inst.Hostname)
	}
	wg.Wait()
	close(errsCh)

	// Check for errors
	errs := make([]error, 0, count)
	for err := range errsCh {
		errs = append(errs, err)
	}

	// Calculate successful deletions
	successfulDeletions := len(instancesToDelete) - len(errs)

	if len(errs) > 0 && successfulDeletions == 0 {
		return fmt.Errorf("failed to delete any instances: %w", errors.Join(errs...))
	}

	// Update cache and curSize only for successful deletions
	for i := 0; i < successfulDeletions; i++ {
		m.deleteInstanceRef(InstanceRef{Hostname: instancesToDelete[i].Hostname})
		asg.curSize--
	}

	deletedCount := successfulDeletions
	klog.Infof("Successfully deleted %d instances from ASG %s", deletedCount, asg.Name)

	if deletedCount == 0 {
		return fmt.Errorf("failed to delete any instances from ASG %s", asg.Name)
	}

	if deletedCount < count {
		klog.Warningf("Only deleted %d out of %d requested instances from ASG %s", deletedCount, count, asg.Name)
	}

	return nil
}

func (m *autoScalingGroups) deleteInstanceRef(ref InstanceRef) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// Find the exact InstanceRef in cache by hostname (since InstanceRef might be partially filled)
	var foundRef *InstanceRef
	var foundAsg *Asg

	for cachedRef, asg := range m.instanceToAsg {
		if cachedRef.Hostname == ref.Hostname {
			foundRef = &cachedRef
			foundAsg = asg
			break
		}
	}

	if foundRef != nil && foundAsg != nil {
		// Remove from instanceToAsg using the exact cached ref
		delete(m.instanceToAsg, *foundRef)

		// Remove from asgToInstances
		_instanceRefs := m.asgToInstances[foundAsg.AsgRef]
		for i, instanceRef := range _instanceRefs {
			if instanceRef.Hostname == ref.Hostname {
				m.asgToInstances[foundAsg.AsgRef] = append(_instanceRefs[:i], _instanceRefs[i+1:]...)
				break
			}
		}
	}
}

func (m *autoScalingGroups) InstanceRefsForAsg(ref AsgRef) ([]InstanceRef, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	return m.asgToInstances[ref], nil
}

func (m *autoScalingGroups) InstancesForAsg(ref AsgRef) ([]*instance.ListInstancesResponse, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	var instanceRefs []InstanceRef
	var found bool
	if instanceRefs, found = m.asgToInstances[ref]; !found {
		klog.Infof("[DEBUG] No instances found in cache for ASG %s, returning empty list", ref.Name)
		return []*instance.ListInstancesResponse{}, nil
	}

	instances := make([]*instance.ListInstancesResponse, 0, len(instanceRefs))
	for _, instanceRef := range instanceRefs {
		instance, err := m.dcService.GetInstanceByHostname(instanceRef.Hostname)
		if err != nil {
			klog.Errorf("[DEBUG] Failed to get instance %s for ASG %s: %v", instanceRef.Hostname, ref.Name, err)
			continue
		}
		instances = append(instances, &instance)
	}

	klog.Infof("[DEBUG] InstancesForAsg %s: returning %d instances", ref.Name, len(instances))
	return instances, nil
}

func (m *autoScalingGroups) DeleteAsg(ref AsgRef) error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	// delete all instances for the asg
	asgInstances, err := m.InstancesForAsg(ref)
	if err != nil {
		return err
	}

	//delete asg from cache
	defer func() {
		delete(m.registeredAsgs, ref)
		delete(m.asgToInstances, ref)
		delete(m.asgNodeGroupSpecs, ref)
	}()

	waitGroup := sync.WaitGroup{}
	errsCh := make(chan error, len(asgInstances))
	for _, asgInstance := range asgInstances {
		waitGroup.Add(1)
		go func(instanceID string, insRef InstanceRef) {
			defer waitGroup.Done()
			// update cache
			delete(m.instanceToAsg, insRef)
			//delete instance
			err := m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
				Action: instance.InstanceActionDelete,
				ID:     instanceID,
			})
			if err != nil {
				errsCh <- err
			}
		}(asgInstance.ID, InstanceRef{Hostname: asgInstance.Hostname})
	}
	waitGroup.Wait()
	close(errsCh)

	errs := make([]error, 0, len(asgInstances))
	for err := range errsCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete all instances: %w", errors.Join(errs...))
	}

	return nil
}

func (m *autoScalingGroups) DeleteInstance(ref InstanceRef) error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	asg, found := m.instanceToAsg[ref]
	if !found {
		return fmt.Errorf("instance %s not found in any ASG", ref.Hostname)
	}

	// Get actual instance details to get the correct ID
	inst, err := m.dcService.GetInstanceByHostname(ref.Hostname)
	if err != nil {
		return fmt.Errorf("failed to get instance details for %s: %v", ref.Hostname, err)
	}

	// Delete instance via API using correct instance ID
	err = m.dcService.PerformInstanceAction(&instance.InstanceActionInput{
		Action: instance.InstanceActionDelete,
		ID:     inst.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete instance %s: %v", inst.ID, err)
	}

	// Update cache only after successful deletion
	m.deleteInstanceRef(ref)

	// Update ASG curSize
	asg.curSize--
	klog.Infof("[DEBUG] Deleted instance %s from ASG %s, curSize now %d",
		ref.Hostname, asg.Name, asg.curSize)

	return nil
}
