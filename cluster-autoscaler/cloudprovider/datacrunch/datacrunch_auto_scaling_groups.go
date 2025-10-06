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
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
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
			klog.V(4).Infof("Found ASG %s for hostname %s via hostname lookup", asg.Name, ref.Hostname)
			return asg, nil
		}
	}

	return nil, fmt.Errorf("ASG not found for hostname: %s", ref.Hostname)
}

func (m *autoScalingGroups) regenerate() error {
	// Copy specs under lock to avoid holding the lock during API calls
	m.cacheMutex.Lock()
	specs := make([]string, 0, len(m.asgNodeGroupSpecs))
	for _, spec := range m.asgNodeGroupSpecs {
		specs = append(specs, spec)
	}
	m.cacheMutex.Unlock()

	klog.V(4).Infof("regenerate() called with %d asgNodeGroupSpecs", len(specs))

	newInstanceToAsg := make(map[InstanceRef]*Asg)
	newAsgToInstances := make(map[AsgRef][]InstanceRef)
	newRegisteredAsgs := make(map[AsgRef]*Asg)

	for _, spec := range specs {
		klog.V(5).Infof("Processing spec in regenerate: %s", spec)
		asg, err := m.buildASGFromSpec(spec)
		if err != nil {
			klog.Errorf("failed to build ASG from spec: %v", err)
			continue
		}
		klog.V(5).Infof("Built ASG %s from spec in regenerate", asg.Name)
		newRegisteredAsgs[asg.AsgRef] = asg

		// get all instances for the asg using hostname-based matching (more reliable)
		klog.V(5).Infof("About to call GetAllInstancesByAsgName for ASG %s", asg.Name)
		deployedInstances, err := m.dcService.GetAllInstancesByAsgName(asg.Name)
		if err != nil {
			klog.Errorf("failed to get instances for ASG %s: %v", asg.Name, err)
			continue
		}
		klog.V(4).Infof("Found %d instances for ASG %s", len(deployedInstances), asg.Name)
		for _, deployedInstance := range deployedInstances {
			providerID := datacrunchProviderIDPrefix + deployedInstance.Location + "/" + deployedInstance.Hostname
			instanceRef := InstanceRef{Hostname: deployedInstance.Hostname, ProviderID: providerID}
			newInstanceToAsg[instanceRef] = asg
			newAsgToInstances[asg.AsgRef] = append(newAsgToInstances[asg.AsgRef], instanceRef)
		}
		asg.curSize = len(deployedInstances)
		klog.V(4).Infof("Set ASG %s curSize to %d based on discovered instances", asg.Name, asg.curSize)
	}

	// Swap new state under lock
	m.cacheMutex.Lock()
	m.registeredAsgs = newRegisteredAsgs
	m.instanceToAsg = newInstanceToAsg
	m.asgToInstances = newAsgToInstances
	m.cacheMutex.Unlock()

	klog.V(4).Infof("regenerate() finished with %d registered ASGs", len(newRegisteredAsgs))
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
	klog.V(5).Infof("parseASGNodeGroupSpecs called with %d specs", len(specs))
	for _, spec := range specs {
		klog.V(5).Infof("Processing spec: %s", spec)
		asg, err := m.buildASGFromSpec(spec)
		if err != nil {
			return err
		}
		klog.V(4).Infof("Registering ASG %s", asg.Name)
		registeredAsg := m.Register(asg)
		klog.V(5).Infof("ASG %s registered successfully", registeredAsg.Name)

		// Store the spec for this ASG
		m.asgNodeGroupSpecs[asg.AsgRef] = spec
		klog.V(5).Infof("Stored spec for ASG %s: %s", asg.Name, spec)
	}
	klog.V(4).Infof("parseASGNodeGroupSpecs finished, total asgNodeGroupSpecs: %d", len(m.asgNodeGroupSpecs))
	return nil
}

func (m *autoScalingGroups) scaleUpAsg(asg *Asg, delta int) error {
	klog.V(4).Infof("Increasing ASG %s by %d nodes", asg.Name, delta)
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	// Validate size constraints under lock
	m.cacheMutex.Lock()
	size := asg.curSize
	max := asg.maxSize
	m.cacheMutex.Unlock()
	if int(size)+delta > max {
		return fmt.Errorf("size increase is too large - desired:%d max:%d", int(size)+delta, max)
	}

	// check if required resources are available (no lock held during API calls)
	location, err := m.dcService.GetInstanceAvailabilityLocation(asg.instanceType, asg.AvailabilityLocations)
	if err != nil {
		return fmt.Errorf("failed to check if instance type %s is available: %v", asg.instanceType, err)
	}
	if location == "" {
		return fmt.Errorf("instance type %s is not available in any of the locations", asg.instanceType)
	}
	klog.V(4).Infof("instance type %s is available in location %s", asg.instanceType, location)

	// prepare node config for each server
	nodeConfig, err := m.getNodeConfigForAsg(asg)
	if err != nil {
		return fmt.Errorf("failed to get node config for ASG %s: %v", asg.Name, err)
	}

	// Create instances concurrently, collect results, and update cache afterwards
	var wg sync.WaitGroup
	errsCh := make(chan error, delta)
	createdRefs := make(chan InstanceRef, delta)
	for i := 0; i < delta; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			klog.V(5).Infof("Creating instance %d/%d for ASG %s", index+1, delta, asg.Name)
			_, hostname, err := m.createInstanceForAsg(asg, nodeConfig, location)
			if err != nil {
				klog.Errorf("Failed to create instance %d for ASG %s: %v", index+1, asg.Name, err)
				errsCh <- err
				return
			}
			providerID := datacrunchProviderIDPrefix + location + "/" + hostname
			createdRefs <- InstanceRef{Hostname: hostname, ProviderID: providerID}
		}(i)
	}
	wg.Wait()
	close(errsCh)
	close(createdRefs)

	// Collect results
	errs := make([]error, 0, delta)
	refs := make([]InstanceRef, 0, delta)
	for err := range errsCh {
		errs = append(errs, err)
	}
	for ref := range createdRefs {
		refs = append(refs, ref)
	}

	// Update cache and curSize for successful creations
	m.cacheMutex.Lock()
	for _, ref := range refs {
		m.instanceToAsg[ref] = asg
		m.asgToInstances[asg.AsgRef] = append(m.asgToInstances[asg.AsgRef], ref)
	}
	asg.curSize += len(refs)
	m.cacheMutex.Unlock()
	klog.V(4).Infof("Updated ASG %s curSize to %d after creating %d instances", asg.Name, asg.curSize, len(refs))

	if len(errs) > 0 {
		return fmt.Errorf("failed to create all servers: %w", errors.Join(errs...))
	}
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

	// check spot from config
	isSpot := false
	if m.cfg.BillingConfig.Contract == "SPOT" {
		isSpot = true
	}

	nodeConfig := &nodeConfig{
		IsSpot:        isSpot,
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
	copy(nodeConfig.Volumes, m.cfg.AdditionalVolumes)

	// Copy taints
	copy(nodeConfig.Taints, m.cfg.Taints)

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
		klog.Errorf("Failed to create startup script for ASG %s: %v", asg.Name, err)
		return "", hostname, err
	}

	// Check if startup script ID is empty
	if startupScriptID == "" {
		klog.Errorf("Startup script creation returned empty ID - cannot proceed with instance creation")
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
			klog.Errorf("No GPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no GPU image configured for instance type %s", asg.instanceType)
		}
	} else {
		image = m.cfg.Image.CPU
		if image == "" {
			klog.Errorf("No CPU image configured for instance type %s", asg.instanceType)
			return "", hostname, fmt.Errorf("no CPU image configured for instance type %s", asg.instanceType)
		}
	}

	// Create the instance input with all required fields
	input := datacrunch.CreateInstanceRequest{
		InstanceType:    asg.instanceType,
		Image:           image,
		SSHKeyIDs:       nodeConfig.SSHKeyIDs, // Required
		StartupScriptID: &startupScriptID,     // Required - now properly created
		Hostname:        hostname,
		Description:     asg.Name, // Use ASG ID as description for grouping
		Location:        location,
		IsSpot:          nodeConfig.IsSpot,
	}

	// Add OS volume - always add since we set default size in validation
	input.OSVolume = &datacrunch.OSVolumeCreateRequest{
		Name: fmt.Sprintf("%s-os-volume", hostname),
		Size: nodeConfig.OSVolumeSize,
		Type: "SSD", // Default type
	}

	// Add additional volumes if specified
	if len(nodeConfig.Volumes) > 0 {
		volumes := make([]datacrunch.VolumeCreateRequest, len(nodeConfig.Volumes))
		for i, vol := range nodeConfig.Volumes {
			volumes[i] = datacrunch.VolumeCreateRequest{
				Name: vol.Name,
				Size: vol.Size,
				Type: vol.Type,
			}
		}
		input.Volumes = volumes
	}

	// High-verbosity request body logging for troubleshooting; gated at V(7)
	if requestBody, err := json.MarshalIndent(input, "", "  "); err == nil {
		klog.V(7).Infof("CreateInstance request body:\n%s", string(requestBody))
	}

	instance, err := m.dcService.CreateInstance(&input)
	if err != nil {
		klog.Errorf("DataCrunch API call failed for instance %s: %v", hostname, err)
		return "", hostname, fmt.Errorf("failed to create instance: %v", err)
	}

	return instance.ID, hostname, nil
}

// createOrGetStartupScript creates a startup script or returns existing ID if already created
func (m *autoScalingGroups) createOrGetStartupScript(asg *Asg, nodeConfig *nodeConfig, providerID string) (string, error) {
	scriptName := fmt.Sprintf("as-%s", asg.Name)
	// decode the base64 and stringify to text utf8 encoded  new line "\n"
	_base64Script, err := base64.StdEncoding.DecodeString(nodeConfig.StartupScript)
	if err != nil {
		return "", fmt.Errorf("failed to decode startup script: %v", err)
	}

	// Prepare a fresh env map to avoid mutating shared config and for concurrency safety
	startupScriptEnv := make(map[string]string, len(m.cfg.StartupScriptEnv)+2)
	for k, v := range m.cfg.StartupScriptEnv {
		startupScriptEnv[strings.ToUpper(k)] = v
	}
	startupScriptEnv["PROVIDER_ID"] = providerID
	labels := convertConfigLabelsToK8sLabels(nodeConfig.Labels, asg)
	startupScriptEnv["LABELS"] = labels

	// Patch the script with env values
	_patchedBase64Script := patchScript(_base64Script, startupScriptEnv)
	// stringify the script
	_scriptsUtf8 := string(_patchedBase64Script)

	script, err := m.dcService.CreateStartScript(scriptName, _scriptsUtf8)
	if err != nil {
		klog.Errorf("CreateStartScript API call failed: %v", err)
		return "", fmt.Errorf("failed to create startup script: %v", err)
	}

	return script.ID, nil
}

func (m *autoScalingGroups) scaleDownAsg(asg *Asg, count int) error {
	if count <= 0 {
		return nil
	}

	klog.V(4).Infof("Scaling down ASG %s by %d instances", asg.Name, count)

	// Get current instances using hostname-based matching (no lock held during API calls)
	instances, err := m.dcService.GetAllInstancesByAsgName(asg.Name)
	if err != nil {
		return fmt.Errorf("failed to get instances for ASG %s: %v", asg.Name, err)
	}

	if len(instances) < count {
		return fmt.Errorf("cannot delete %d instances, only %d available", count, len(instances))
	}

	// Validate we don't go below min size
	if len(instances)-count < asg.minSize {
		return fmt.Errorf("scaling down by %d would go below min size %d (current: %d)", count, asg.minSize, len(instances))
	}

	var wg sync.WaitGroup
	errsCh := make(chan error, count)
	deletedHostnames := make(chan string, count)

	// Collect instances to delete and their IDs
	instancesToDelete := make([]struct {
		ID       string
		Hostname string
	}, 0, count)
	for i := 0; i < count && i < len(instances); i++ {
		instancesToDelete = append(instancesToDelete, struct {
			ID       string
			Hostname string
		}{ID: instances[i].ID, Hostname: instances[i].Hostname})
	}

	// Delete instances concurrently
	for _, inst := range instancesToDelete {
		wg.Add(1)
		go func(instanceID, hostname string) {
			defer wg.Done()
			err := m.dcService.PerformInstanceAction(instanceID, datacrunch.ActionDelete)
			if err != nil {
				klog.Errorf("Failed to delete instance %s: %v", instanceID, err)
				errsCh <- err
				return
			}
			klog.V(4).Infof("Successfully deleted instance %s from ASG %s", instanceID, asg.Name)
			deletedHostnames <- hostname
		}(inst.ID, inst.Hostname)
	}
	wg.Wait()
	close(errsCh)
	close(deletedHostnames)

	// Gather results
	errs := make([]error, 0, count)
	successHostnames := make([]string, 0, count)
	for err := range errsCh {
		errs = append(errs, err)
	}
	for hn := range deletedHostnames {
		successHostnames = append(successHostnames, hn)
	}

	if len(errs) > 0 && len(successHostnames) == 0 {
		return fmt.Errorf("failed to delete any instances: %w", errors.Join(errs...))
	}

	// Update cache and curSize for successful deletions under lock
	m.cacheMutex.Lock()
	for _, hostname := range successHostnames {
		// Find exact cached ref and owning ASG
		var cachedRef *InstanceRef
		for ref := range m.instanceToAsg {
			if ref.Hostname == hostname {
				r := ref // copy
				cachedRef = &r
				break
			}
		}
		if cachedRef != nil {
			asgRef := m.instanceToAsg[*cachedRef].AsgRef
			delete(m.instanceToAsg, *cachedRef)
			refs := m.asgToInstances[asgRef]
			for i, r := range refs {
				if r.Hostname == hostname {
					m.asgToInstances[asgRef] = append(refs[:i], refs[i+1:]...)
					break
				}
			}
			asg.curSize--
		}
	}
	m.cacheMutex.Unlock()

	deletedCount := len(successHostnames)
	klog.V(4).Infof("Successfully deleted %d instances from ASG %s", deletedCount, asg.Name)

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

func (m *autoScalingGroups) InstancesForAsg(ref AsgRef) ([]datacrunch.Instance, error) {
	// Copy refs under lock, do API calls without holding the lock
	m.cacheMutex.Lock()
	refs0, found := m.asgToInstances[ref]
	refs := append([]InstanceRef(nil), refs0...)
	m.cacheMutex.Unlock()
	if !found {
		klog.V(5).Infof("No instances found in cache for ASG %s, returning empty list", ref.Name)
		return []datacrunch.Instance{}, nil
	}

	instances := make([]datacrunch.Instance, 0, len(refs))
	for _, r := range refs {
		inst, err := m.dcService.GetInstanceByHostname(r.Hostname)
		if err != nil {
			klog.Errorf("Failed to get instance %s for ASG %s: %v", r.Hostname, ref.Name, err)
			continue
		}
		instances = append(instances, *inst)
	}

	klog.V(5).Infof("InstancesForAsg %s: returning %d instances", ref.Name, len(instances))
	return instances, nil
}

func (m *autoScalingGroups) DeleteAsg(ref AsgRef) error {
	// Copy instance refs under lock to avoid deadlocks
	m.cacheMutex.Lock()
	instanceRefs := append([]InstanceRef(nil), m.asgToInstances[ref]...)
	asg := m.registeredAsgs[ref]
	m.cacheMutex.Unlock()

	// Delete all instances concurrently (no lock held during API calls)
	var wg sync.WaitGroup
	errsCh := make(chan error, len(instanceRefs))
	for _, insRef := range instanceRefs {
		wg.Add(1)
		go func(hostname string) {
			defer wg.Done()
			inst, err := m.dcService.GetInstanceByHostname(hostname)
			if err != nil {
				errsCh <- err
				return
			}
			err = m.dcService.PerformInstanceAction(inst.ID, datacrunch.ActionDelete)
			if err != nil {
				errsCh <- err
			}
		}(insRef.Hostname)
	}
	wg.Wait()
	close(errsCh)

	var errs []error
	for err := range errsCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete all instances: %w", errors.Join(errs...))
	}

	// Clean up caches under lock
	m.cacheMutex.Lock()
	for _, insRef := range instanceRefs {
		// remove from instanceToAsg by hostname
		var keyToDelete *InstanceRef
		for k := range m.instanceToAsg {
			if k.Hostname == insRef.Hostname {
				kk := k
				keyToDelete = &kk
				break
			}
		}
		if keyToDelete != nil {
			delete(m.instanceToAsg, *keyToDelete)
		}
	}
	delete(m.registeredAsgs, ref)
	delete(m.asgToInstances, ref)
	delete(m.asgNodeGroupSpecs, ref)
	if asg != nil {
		asg.curSize = 0
	}
	m.cacheMutex.Unlock()
	return nil
}

func (m *autoScalingGroups) DeleteInstance(ref InstanceRef) error {
	// Find ASG mapping under lock (try exact and then by hostname)
	m.cacheMutex.Lock()
	asg, found := m.instanceToAsg[ref]
	if !found {
		for k, v := range m.instanceToAsg {
			if k.Hostname == ref.Hostname {
				ref = k
				asg = v
				found = true
				break
			}
		}
	}
	m.cacheMutex.Unlock()
	if !found {
		return fmt.Errorf("instance %s not found in any ASG", ref.Hostname)
	}

	// Get actual instance details to get the correct ID (no lock held)
	inst, err := m.dcService.GetInstanceByHostname(ref.Hostname)
	if err != nil {
		return fmt.Errorf("failed to get instance details for %s: %v", ref.Hostname, err)
	}

	// Delete instance via API using correct instance ID
	if err := m.dcService.PerformInstanceAction(inst.ID, datacrunch.ActionDelete); err != nil {
		return fmt.Errorf("failed to delete instance %s: %v", inst.ID, err)
	}

	// Update cache and ASG size under lock
	m.cacheMutex.Lock()
	// Remove from instanceToAsg
	var keyToDelete *InstanceRef
	for k := range m.instanceToAsg {
		if k.Hostname == ref.Hostname {
			kk := k
			keyToDelete = &kk
			break
		}
	}
	if keyToDelete != nil {
		delete(m.instanceToAsg, *keyToDelete)
	}
	// Remove from asgToInstances
	refs := m.asgToInstances[asg.AsgRef]
	for i, r := range refs {
		if r.Hostname == ref.Hostname {
			m.asgToInstances[asg.AsgRef] = append(refs[:i], refs[i+1:]...)
			break
		}
	}
	asg.curSize--
	m.cacheMutex.Unlock()

	klog.V(4).Infof("Deleted instance %s from ASG %s, curSize now %d", ref.Hostname, asg.Name, asg.curSize)
	return nil
}
