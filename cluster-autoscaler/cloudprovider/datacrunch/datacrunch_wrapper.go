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
	"context"
	"fmt"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
	klog "k8s.io/klog/v2"
)

// DatacrunchWrapper provides high-level operations for DataCrunch instances
type datacrunchWrapper struct {
	client *datacrunch.Client
	ctx    context.Context
}

// GetInstanceAvailabilityLocation finds an available location for the given instance type
func (w *datacrunchWrapper) GetInstanceAvailabilityLocation(instanceType string, locations []string) (string, error) {
	if len(locations) == 0 {
		return "", fmt.Errorf("locations is empty")
	}

	// Convert all locations to upper case
	newLocations := make([]string, len(locations))
	for i, location := range locations {
		newLocations[i] = strings.ToUpper(location)
	}

	// Check each location for availability
	for _, location := range newLocations {
		available, err := w.client.Instances.IsAvailable(w.ctx, instanceType, false, location)
		if err != nil {
			klog.V(4).Infof("Error checking availability for %s in %s: %v", instanceType, location, err)
			continue
		}
		if available {
			klog.V(4).Infof("Instance type %s is AVAILABLE in location %s", instanceType, location)
			return location, nil
		}
	}

	klog.Warningf("Instance type %s not available in any of the locations %v", instanceType, newLocations)
	return "", nil
}

// GetInstanceTypeDetails retrieves details for a specific instance type
func (w *datacrunchWrapper) GetInstanceTypeDetails(instanceType string) (*InstanceResource, error) {
	if instanceType == "" {
		return nil, fmt.Errorf("instance type is empty")
	}

	// Get all locations to find instance type details
	locations, err := w.client.Locations.Get(w.ctx)
	if err != nil {
		klog.Errorf("Error fetching locations: %v", err)
		return nil, err
	}

	// Try to get instance details from availability data
	// Note: The official SDK doesn't have a direct ListInstanceTypes method
	// We'll need to infer from availability or use a different approach
	for _, loc := range locations {
		availabilities, err := w.client.Instances.GetAvailabilities(w.ctx, nil, loc.Code)
		if err != nil {
			continue
		}

		for _, avail := range availabilities {
			if avail.InstanceType == instanceType {
				// For now, return basic info - we may need to enhance this
				// based on instance type naming conventions
				return parseInstanceType(instanceType), nil
			}
		}
	}

	klog.Errorf("Instance type %s not found in API response", instanceType)
	return nil, fmt.Errorf("instance type %s not found", instanceType)
}

// parseInstanceType extracts resource information from instance type name
// Example: "1V100.6V" means 1 GPU, 6 vCPUs
func parseInstanceType(instanceType string) *InstanceResource {
	// This is a simplified parser - adjust based on actual naming conventions
	return &InstanceResource{
		InstanceType: instanceType,
		Arch:         "amd64",
		CPU:          6,                       // Default, should be parsed from name
		Memory:       32 * 1024 * 1024 * 1024, // Default 32GB
		GPU:          1,                       // Default, should be parsed from name
	}
}

// GetInstanceByHostname retrieves an instance by its hostname
func (w *datacrunchWrapper) GetInstanceByHostname(hostname string) (*datacrunch.Instance, error) {
	// Get all instances
	instances, err := w.client.Instances.Get(w.ctx, "")
	if err != nil {
		return nil, err
	}

	// Define active statuses that should be considered
	activeStatuses := map[string]bool{
		"NEW":                    true,
		"ORDERED":                true,
		"PROVISIONING":           true,
		datacrunch.StatusRunning: true,
	}

	for _, inst := range instances {
		if inst.Hostname == hostname && activeStatuses[inst.Status] {
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("instance with hostname %s not found", hostname)
}

// GetAllInstancesByDescription retrieves all instances matching a description
func (w *datacrunchWrapper) GetAllInstancesByDescription(description string) ([]datacrunch.Instance, error) {
	// Get all instances
	instances, err := w.client.Instances.Get(w.ctx, "")
	if err != nil {
		klog.Errorf("ListInstances API call failed: %v", err)
		return nil, err
	}

	// Define active statuses
	activeStatuses := map[string]bool{
		"NEW":                    true,
		"ORDERED":                true,
		"PROVISIONING":           true,
		datacrunch.StatusRunning: true,
	}

	filteredInstances := make([]datacrunch.Instance, 0)
	for _, inst := range instances {
		if inst.Description == description && activeStatuses[inst.Status] {
			filteredInstances = append(filteredInstances, inst)
		}
	}

	klog.V(5).Infof("GetAllInstancesByDescription returning %d filtered instances for description '%s'",
		len(filteredInstances), description)
	return filteredInstances, nil
}

// GetAllInstancesByAsgName gets all instances that belong to an ASG
func (w *datacrunchWrapper) GetAllInstancesByAsgName(asgName string) ([]datacrunch.Instance, error) {
	// Get all instances
	instances, err := w.client.Instances.Get(w.ctx, "")
	if err != nil {
		klog.Errorf("ListInstances API call failed: %v", err)
		return nil, err
	}

	klog.V(5).Infof("GetAllInstancesByAsgName found %d total instances from API", len(instances))

	// Log details of all instances for debugging
	for i, inst := range instances {
		klog.V(6).Infof("Instance %d: hostname='%s', description='%s', status='%s'",
			i+1, inst.Hostname, inst.Description, inst.Status)
	}

	// Define active statuses
	activeStatuses := map[string]bool{
		"NEW":                    true,
		"ORDERED":                true,
		"PROVISIONING":           true,
		datacrunch.StatusRunning: true,
	}

	filteredInstances := make([]datacrunch.Instance, 0)
	for _, inst := range instances {
		// Only process instances in active states
		if !activeStatuses[inst.Status] {
			klog.V(6).Infof("Skipping instance '%s' with inactive status: %s", inst.Hostname, inst.Status)
			continue
		}

		// Extract ASG name from hostname
		extractedAsgName, err := extractAsgNameFromHostname(inst.Hostname)
		if err != nil {
			klog.V(5).Infof("Failed to extract ASG from hostname '%s': %v, trying description fallback", inst.Hostname, err)
			// Fall back to description check
			if inst.Description == asgName {
				klog.V(6).Infof("Instance '%s' matched by description: %s", inst.Hostname, inst.Description)
				filteredInstances = append(filteredInstances, inst)
			} else {
				klog.V(6).Infof("Instance '%s' description '%s' does not match ASG '%s'", inst.Hostname, inst.Description, asgName)
			}
			continue
		}

		// Match by extracted ASG name
		if extractedAsgName == asgName {
			klog.V(6).Infof("Instance '%s' matched by extracted ASG name: %s", inst.Hostname, extractedAsgName)
			filteredInstances = append(filteredInstances, inst)
		} else {
			klog.V(6).Infof("Instance '%s' extracted ASG '%s' does not match target '%s'", inst.Hostname, extractedAsgName, asgName)
		}
	}

	klog.V(5).Infof("GetAllInstancesByAsgName returning %d filtered instances for ASG '%s'",
		len(filteredInstances), asgName)

	// Log the final filtered instances
	for i, inst := range filteredInstances {
		klog.V(6).Infof("Filtered instance %d: hostname='%s', description='%s', status='%s'",
			i+1, inst.Hostname, inst.Description, inst.Status)
	}

	return filteredInstances, nil
}

// ListInstances retrieves instances with optional status filter
func (w *datacrunchWrapper) ListInstances(status string) ([]datacrunch.Instance, error) {
	return w.client.Instances.Get(w.ctx, status)
}

// CreateInstance creates a new instance
func (w *datacrunchWrapper) CreateInstance(req *datacrunch.CreateInstanceRequest) (*datacrunch.Instance, error) {
	return w.client.Instances.Create(w.ctx, *req)
}

// PerformInstanceAction performs an action on an instance
func (w *datacrunchWrapper) PerformInstanceAction(instanceID, action string) error {
	return w.client.Instances.Action(w.ctx, instanceID, action, nil)
}

// DeleteInstance deletes an instance
func (w *datacrunchWrapper) DeleteInstance(instanceID string) error {
	return w.client.Instances.Delete(w.ctx, instanceID, nil)
}

// CreateStartScript creates a startup script
func (w *datacrunchWrapper) CreateStartScript(name, script string) (*datacrunch.StartupScript, error) {
	req := datacrunch.CreateStartupScriptRequest{
		Name:   name,
		Script: script,
	}
	return w.client.StartupScripts.Create(w.ctx, req)
}

// DeleteStartScript deletes a startup script
func (w *datacrunchWrapper) DeleteStartScript(id string) error {
	return w.client.StartupScripts.Delete(w.ctx, id)
}

// ListStartScripts retrieves all startup scripts
func (w *datacrunchWrapper) ListStartScripts() ([]datacrunch.StartupScript, error) {
	return w.client.StartupScripts.Get(w.ctx)
}

// ListInstanceTypes retrieves all available instance types from availability data
func (w *datacrunchWrapper) ListInstanceTypes() ([]string, error) {
	availabilities, err := w.client.Instances.GetAvailabilities(w.ctx, nil, "")
	if err != nil {
		return nil, err
	}

	// Use a map to deduplicate instance types
	typeMap := make(map[string]bool)
	for _, avail := range availabilities {
		typeMap[avail.InstanceType] = true
	}

	// Convert map to slice
	types := make([]string, 0, len(typeMap))
	for instanceType := range typeMap {
		types = append(types, instanceType)
	}

	return types, nil
}
