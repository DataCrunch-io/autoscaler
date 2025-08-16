package datacrunch

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	// GPULabel is the label added to nodes with GPU resource.
	// pending to check
	GPULabel = "datacrunch.io/type"
)

// DatacrunchCloudProvider implements CloudProvider interface for DataCrunch
type DatacrunchCloudProvider struct {
	manager         *DatacrunchManager
	resourceLimiter *cloudprovider.ResourceLimiter
}

// DatacrunchAsgSpec holds ASG specification
type DatacrunchAsgSpec struct {
	minSize      int
	maxSize      int
	instanceType string
	name         string
}

// newDatacrunchCloudProvider implement CloudProvider interface
func newDatacrunchCloudProvider(manager *DatacrunchManager, rl *cloudprovider.ResourceLimiter) (*DatacrunchCloudProvider, error) {
	return &DatacrunchCloudProvider{
		manager:         manager,
		resourceLimiter: rl,
	}, nil
}

// Name returns name of the cloud provider
func (d *DatacrunchCloudProvider) Name() string {
	return cloudprovider.DatacrunchProviderName
}

// NodeGroups returns all ASGs configured for this cloud provider
func (d *DatacrunchCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	d.manager.asgs.cacheMutex.Lock()
	defer d.manager.asgs.cacheMutex.Unlock()

	groups := make([]cloudprovider.NodeGroup, 0, len(d.manager.asgs.registeredAsgs))
	for _, asg := range d.manager.asgs.registeredAsgs {
		groups = append(groups, asg.config)
	}
	klog.V(4).Infof("[DEBUG] NodeGroups() returning %d node groups", len(groups))
	for i, group := range groups {
		klog.V(4).Infof("[DEBUG] NodeGroup %d: %s (min:%d, max:%d)", i+1, group.Id(), group.MinSize(), group.MaxSize())
	}
	return groups
}

// NodeGroupForNode returns the ASG for the given node
func (d *DatacrunchCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	klog.V(3).Infof("[DEBUG] NodeGroupForNode called for node: %s, providerID: %s", node.Name, node.Spec.ProviderID)
	location, hostname, err := toInstanceIDAndHostname(node.Spec.ProviderID)
	if err != nil {
		klog.V(4).Infof("[DEBUG] Node %s does not belong to DataCrunch (invalid providerID format): %v", node.Name, err)
		// Return nil, nil (not an error) when node doesn't belong to this cloud provider
		return nil, nil
	}
	// find from cache will be faster need consider cache
	// the instanceID can not be used due to create the nodegroup before create the instance?
	// @todo to consider first
	klog.V(3).Infof("[DEBUG] Found location: %s, hostname: %s for node %s", location, hostname, node.Name)

	// Use the registry to find the ASG for this instance
	asg, err := d.manager.GetAsgForInstanceByHostname(hostname)
	if err != nil {
		klog.V(3).Infof("[DEBUG] Error finding ASG for hostname %s: %v", hostname, err)
		return nil, err
	}
	if asg != nil {
		klog.V(3).Infof("[DEBUG] Found ASG %s for node %s (hostname: %s)", asg.Id(), node.Name, hostname)
		return asg, nil
	}
	klog.V(3).Infof("[DEBUG] No ASG found for node %s (hostname: %s, location: %s)", node.Name, hostname, location)

	klog.V(2).Infof("[DEBUG] No node group found for node: %s (providerID: %s)", node.Name, node.Spec.ProviderID)
	return nil, nil
}

// HasInstance returns whether the given node has a corresponding instance in this cloud provider
func (d *DatacrunchCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	return true, cloudprovider.ErrNotImplemented
}

// Pricing returns pricing model for this cloud provider or error if not available
func (d *DatacrunchCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider
func (d *DatacrunchCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	// @todo need connect to call wrapper.GetAvailableMachineTypes()
	// and the wrapper api will create a new list based on apis and with cache
	// so the list will be expired after 10 minutes
	return []string{}, nil
}

// NewNodeGroup builds a theoretical ASG based on the node definition provided
func (d *DatacrunchCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.)
func (d *DatacrunchCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return d.resourceLimiter, nil
}

// GPULabel returns the label added to nodes with GPU resource
func (d *DatacrunchCloudProvider) GPULabel() string {
	return ""
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports
func (d *DatacrunchCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	// @todo need connect to call wrapper.GetAvailableGPUTypes()
	// and the wrapper api will create a new list based on apis and with cache
	// so the list will be expired after 10 minutes
	return map[string]struct{}{
		"1L40S.20V": {},
		"A40.22V":   {},
		"A6000.24V": {},
	}
}

// GetNodeGpuConfig returns the label, type and resource name for the GPU added to node
func (d *DatacrunchCloudProvider) GetNodeGpuConfig(node *apiv1.Node) *cloudprovider.GpuConfig {
	// Check if node has GPU resources
	if gpuQuantity, exists := node.Status.Capacity["nvidia.com/gpu"]; exists && !gpuQuantity.IsZero() {
		// DataCrunch uses nvidia.com/gpu as the standard GPU resource name
		return &cloudprovider.GpuConfig{
			Label:        "accelerator",
			Type:         "nvidia-gpu",
			ResourceName: "nvidia.com/gpu",
		}
	}
	return nil
}

// Cleanup cleans up resources open by this provider
func (d *DatacrunchCloudProvider) Cleanup() error {
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state
func (d *DatacrunchCloudProvider) Refresh() error {
	klog.V(3).Info("[DEBUG] DataCrunch cloud provider refresh called - checking ASG states")

	// Log current state of all ASGs
	klog.V(3).Infof("[DEBUG] Refresh: Checking %d registered ASGs", len(d.manager.asgs.registeredAsgs))
	for i, asg := range d.manager.asgs.registeredAsgs {
		currentSize, err := d.manager.GetAsgSize(asg.config)
		if err != nil {
			klog.V(3).Infof("[DEBUG] Could not get size for ASG %d/%d %s: %v", i+1, len(d.manager.asgs.registeredAsgs), asg.config.id, err)
		} else {
			klog.V(3).Infof("[DEBUG] ASG %d/%d %s current state: size=%d, min=%d, max=%d, instanceType=%s",
				i+1, len(d.manager.asgs.registeredAsgs), asg.config.id, currentSize, asg.config.minSize, asg.config.maxSize, asg.config.instanceType)

			// Check if ASG is below minimum size and scale up if needed
			if currentSize < int64(asg.config.minSize) {
				needed := int64(asg.config.minSize) - currentSize
				klog.V(2).Infof("[DEBUG] ⚠️  ASG %s is BELOW minimum size! Current: %d, Min: %d, Need to create: %d instances",
					asg.config.id, currentSize, asg.config.minSize, needed)

				// Scale up to meet minimum size
				klog.V(2).Infof("[DEBUG] 🚀 Scaling ASG %s up to minimum size %d (adding %d instances)", asg.config.id, asg.config.minSize, needed)
				err = d.manager.SetAsgSize(asg.config, int64(asg.config.minSize))
				if err != nil {
					klog.Errorf("[DEBUG] ❌ Failed to scale ASG %s to minimum size: %v", asg.config.id, err)
				} else {
					klog.V(2).Infof("[DEBUG] ✅ Successfully initiated scale-up for ASG %s to minimum size %d", asg.config.id, asg.config.minSize)
				}
			} else if currentSize == int64(asg.config.minSize) {
				klog.V(3).Infof("[DEBUG] ✅ ASG %s is at minimum size: %d", asg.config.id, currentSize)
			} else {
				klog.V(3).Infof("[DEBUG] ASG %s is above minimum: current=%d, min=%d", asg.config.id, currentSize, asg.config.minSize)
			}

			// Also check if we have any nodes for this ASG
			nodes, err := asg.config.Nodes()
			if err != nil {
				klog.V(3).Infof("[DEBUG] Error getting nodes for ASG %s: %v", asg.config.id, err)
			} else {
				klog.V(3).Infof("[DEBUG] ASG %s has %d nodes in Kubernetes", asg.config.id, len(nodes))
			}
		}
	}

	return d.manager.Refresh()
}

// BuildDatacrunchCloudProvider builds the DataCrunch cloud provider.
func BuildDatacrunch(
	opts config.AutoscalingOptions,
	do cloudprovider.NodeGroupDiscoveryOptions,
	rl *cloudprovider.ResourceLimiter,
) cloudprovider.CloudProvider {

	var configFile io.ReadCloser
	if opts.CloudConfig != "" {
		var err error
		configFile, err = os.Open(opts.CloudConfig)
		if err != nil {
			klog.Fatalf("Couldn't open cloud provider configuration %s: %#v", opts.CloudConfig, err)
		}
		defer configFile.Close()
	}

	manager, err := createDatacrunchManager(configFile, createKubeClient(opts))
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch manager: %v", err)
	}

	provider, err := newDatacrunchCloudProvider(manager, rl)
	if err != nil {
		klog.Fatalf("Failed to create DataCrunch cloud provider: %v", err)
	}

	// add static ASGs
	if do.StaticDiscoverySpecified() {
		err := provider.addStaticASGs(do.NodeGroupSpecs, manager.cfg, createKubeClient(opts))
		if err != nil {
			klog.Fatalf("Failed to add static ASGs: %v", err)
		}
	}

	return provider
}

func (d *DatacrunchCloudProvider) addStaticASGs(asgSpecs []string, cfg *cloudConfig, kubeClient kubernetes.Interface) error {
	klog.V(2).Infof("[DEBUG] Adding %d static ASG specifications", len(asgSpecs))
	for i, spec := range asgSpecs {
		klog.V(2).Infof("[DEBUG] Processing ASG spec %d: %s", i+1, spec)
		asgSpec, err := d.parseAsgSpec(spec)
		if err != nil {
			klog.Errorf("Failed to parse ASG spec: %v", err)
			return err
		}

		// Initialize ASG wrapper for this static group
		klog.V(2).Infof("[DEBUG] Creating ASG: name=%s, min=%d, max=%d, type=%s",
			asgSpec.name, asgSpec.minSize, asgSpec.maxSize, asgSpec.instanceType)
		asg := &Asg{
			manager:               d.manager,
			kubeClient:            kubeClient,
			id:                    asgSpec.name,
			minSize:               asgSpec.minSize,
			maxSize:               asgSpec.maxSize,
			instanceType:          asgSpec.instanceType,
			AvailabilityLocations: cfg.AvailableLocations,
		}
		d.manager.RegisterAsg(asg)
		klog.V(2).Infof("[DEBUG] Successfully registered ASG: %s", asgSpec.name)

		// Check and enforce minimum size immediately after registration
		currentSize, err := d.manager.GetAsgSize(asg)
		if err != nil {
			klog.Errorf("[DEBUG] Error getting size for newly registered ASG %s: %v", asg.id, err)
		} else {
			klog.V(2).Infof("[DEBUG] Newly registered ASG %s current size: %d, minimum size: %d", asg.id, currentSize, asg.minSize)
			if currentSize < int64(asg.minSize) {
				needed := int64(asg.minSize) - currentSize
				klog.V(1).Infof("[DEBUG] 🚀 ASG %s is below minimum size at startup! Current: %d, Min: %d, scaling up by %d instances",
					asg.id, currentSize, asg.minSize, needed)
				err = d.manager.SetAsgSize(asg, int64(asg.minSize))
				if err != nil {
					klog.Errorf("[DEBUG] ❌ Failed to scale ASG %s to minimum size at startup: %v", asg.id, err)
				} else {
					klog.V(1).Infof("[DEBUG] ✅ Successfully initiated startup scale-up for ASG %s to minimum size %d", asg.id, asg.minSize)
				}
			} else {
				klog.V(2).Infof("[DEBUG] ✅ ASG %s is already at or above minimum size (%d >= %d)", asg.id, currentSize, asg.minSize)
			}
		}
	}
	return nil
}

// parse format: min:max:instance-type:asg-name
func (d *DatacrunchCloudProvider) parseAsgSpec(spec string) (*DatacrunchAsgSpec, error) {
	klog.V(3).Infof("[DEBUG] Parsing ASG spec: %s", spec)
	parts := strings.Split(spec, ":")
	if len(parts) != 4 {
		klog.Errorf("[DEBUG] Invalid ASG spec format: expected 4 parts, got %d - %v", len(parts), parts)
		return nil, fmt.Errorf("invalid ASG spec: %s", spec)
	}

	instanceType := parts[2]
	asgName := parts[3]

	minSize, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid min size: %s", parts[0])
	}

	maxSize, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid max size: %s", parts[1])
	}

	validAsgName := regexp.MustCompile(`^[a-z0-9A-Z]+[a-z0-9A-Z\-\.\_]*[a-z0-9A-Z]+$|^[a-z0-9A-Z]{1}$`)
	if !validAsgName.MatchString(asgName) {
		return nil, fmt.Errorf("invalid ASG name: %s", asgName)
	}

	klog.V(3).Infof("[DEBUG] Parsed ASG spec successfully: min=%d, max=%d, instanceType=%s, name=%s",
		minSize, maxSize, instanceType, asgName)
	return &DatacrunchAsgSpec{
		minSize:      minSize,
		maxSize:      maxSize,
		instanceType: instanceType,
		name:         asgName,
	}, nil
}

// toInstanceID parses the providerID and returns the instanceID
func toInstanceIDAndHostname(providerID string) (string, string, error) {
	// try to parse the providerID as datacrunch://location/hostname
	if !strings.HasPrefix(providerID, datacrunchProviderIDPrefix) {
		return "", "", fmt.Errorf("invalid providerID format: %s", providerID)
	}
	//
	_providerID := strings.TrimPrefix(providerID, datacrunchProviderIDPrefix)
	parts := strings.Split(_providerID, "/")
	if len(parts) < 2 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}

func getKubeConfig(opts config.AutoscalingOptions) *rest.Config {
	klog.V(1).Infof("Using kubeconfig file: %s", opts.KubeClientOpts.KubeConfigPath)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeClientOpts.KubeConfigPath)
	if err != nil {
		klog.Fatalf("Failed to build kubeConfig: %v", err)
	}

	return kubeConfig
}

func createKubeClient(opts config.AutoscalingOptions) kubernetes.Interface {
	return kubernetes.NewForConfigOrDie(getKubeConfig(opts))
}
