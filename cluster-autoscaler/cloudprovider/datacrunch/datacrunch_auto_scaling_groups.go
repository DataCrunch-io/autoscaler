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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	klog "k8s.io/klog/v2"
)

type autoScalingGroups struct {
	registeredAsgs           []*asgInformation
	instanceToAsg            map[string]*Asg
	cacheMutex               sync.Mutex
	instancesNotInManagedAsg map[string]struct{}
	manager                  *DatacrunchManager
}

type asgInformation struct {
	config *Asg
}

func newAutoScalingGroups(manager *DatacrunchManager) *autoScalingGroups {
	registry := &autoScalingGroups{
		registeredAsgs:           make([]*asgInformation, 0),
		manager:                  manager,
		instanceToAsg:            make(map[string]*Asg),
		instancesNotInManagedAsg: make(map[string]struct{}),
	}

	go wait.Forever(func() {
		registry.cacheMutex.Lock()
		defer registry.cacheMutex.Unlock()
		if err := registry.regenerateCache(); err != nil {
			klog.Errorf("failed to regenerate ASG cache: %v", err)
		}
	}, time.Hour)

	return registry
}

// Register registers asg in DataCrunch Manager.
func (m *autoScalingGroups) Register(asg *Asg) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	klog.Infof("[DEBUG] Registering ASG in registry: %s (type=%s, min=%d, max=%d)",
		asg.id, asg.instanceType, asg.minSize, asg.maxSize)
	m.registeredAsgs = append(m.registeredAsgs, &asgInformation{
		config: asg,
	})
	klog.Infof("[DEBUG] Total ASGs in registry: %d", len(m.registeredAsgs))
}

// FindForInstance returns Asg of the given Instance
func (m *autoScalingGroups) FindForInstance(hostname string) (*Asg, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	klog.Infof("[DEBUG] Looking for ASG for instance hostname: %s", hostname)
	if config, found := m.instanceToAsg[hostname]; found {
		klog.Infof("[DEBUG] Found ASG %s for hostname %s in cache", config.id, hostname)
		return config, nil
	}
	if _, found := m.instancesNotInManagedAsg[hostname]; found {
		// The instance is already known to not belong to any configured ASG
		// Skip regenerateCache so that we won't unnecessarily call APIs
		return nil, nil
	}
	if err := m.regenerateCache(); err != nil {
		return nil, err
	}
	if config, found := m.instanceToAsg[hostname]; found {
		return config, nil
	}
	// instance does not belong to any configured ASG
	m.instancesNotInManagedAsg[hostname] = struct{}{}
	return nil, nil
}

func (m *autoScalingGroups) regenerateCache() error {
	newCache := make(map[string]*Asg)

	for _, asg := range m.registeredAsgs {
		instances, err := m.manager.allAsgRunningInstances(asg.config.id)
		if err != nil {
			return err
		}
		for _, instance := range instances {
			if instance.Hostname != "" {
				// Map by hostname, not instance ID, since lookups are by hostname
				newCache[instance.Hostname] = asg.config
				klog.Infof("[DEBUG] Cached mapping: hostname %s -> ASG %s", instance.Hostname, asg.config.id)
			}
		}
	}

	m.instanceToAsg = newCache
	return nil
}
