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

	m.registeredAsgs = append(m.registeredAsgs, &asgInformation{
		config: asg,
	})
}

// FindForInstance returns Asg of the given Instance
func (m *autoScalingGroups) FindForInstance(instanceId string) (*Asg, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	if config, found := m.instanceToAsg[instanceId]; found {
		return config, nil
	}
	if _, found := m.instancesNotInManagedAsg[instanceId]; found {
		// The instance is already known to not belong to any configured ASG
		// Skip regenerateCache so that we won't unnecessarily call APIs
		return nil, nil
	}
	if err := m.regenerateCache(); err != nil {
		return nil, err
	}
	if config, found := m.instanceToAsg[instanceId]; found {
		return config, nil
	}
	// instance does not belong to any configured ASG
	m.instancesNotInManagedAsg[instanceId] = struct{}{}
	return nil, nil
}

func (m *autoScalingGroups) regenerateCache() error {
	newCache := make(map[string]*Asg)

	for _, asg := range m.registeredAsgs {
		instances, err := m.manager.allInstances(asg.config.id)
		if err != nil {
			return err
		}
		for _, instance := range instances {
			if instance != nil {
				newCache[instance.ID] = asg.config
			}
		}
	}

	m.instanceToAsg = newCache
	return nil
}