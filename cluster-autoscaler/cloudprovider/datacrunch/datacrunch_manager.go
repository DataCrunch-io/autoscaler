package datacrunch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/klog/v2"
)

// DatacrunchManager handles DataCrunch API operations
type DatacrunchManager struct {
	client         *datacrunch.Client
	clusterConfig  *ClusterConfig
	resourceLimiter *cloudprovider.ResourceLimiter
}

// ClusterConfig represents the cluster configuration for DataCrunch
type ClusterConfig struct {
	ProjectId    string                      `json:"project_id"`
	GlobalConfig *GlobalConfig               `json:"global_config,omitempty"`
	NodeConfigs  map[string]*NodeConfig      `json:"node_configs"`
}

// GlobalConfig contains global cluster settings
type GlobalConfig struct {
	APIServerEndpoint    string `json:"api_server_endpoint"`
	BootstrapTokenId     string `json:"bootstrap_token_id"`
	BootstrapTokenSecret string `json:"bootstrap_token_secret"`
	CACertHash           string `json:"ca_cert_hash"`
	KubernetesVersion    string `json:"kubernetes_version"`
	PodCIDR              string `json:"pod_cidr"`
	ServiceCIDR          string `json:"service_cidr"`
}

// NodeConfig represents configuration for a node group
type NodeConfig struct {
	ImageType            string            `json:"image_type"`
	DiskSizeGB           int               `json:"disk_size_gb"`
	InstanceOption       string            `json:"instance_option"`
	PricingOption        string            `json:"pricing_option"`
	SSHKeyIDs            []string          `json:"ssh_key_ids"`
	StartupScriptBase64  string            `json:"startup_script_base64,omitempty"`
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Taints               []NodeTaint       `json:"taints,omitempty"`
	
	// Deprecated fields - kept for backward compatibility
	BootstrapTemplate    string            `json:"bootstrap_template,omitempty"`
}

// NodeTaint represents a Kubernetes node taint
type NodeTaint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// NewDatacrunchManager creates a new DataCrunch manager
func NewDatacrunchManager(configFile string) (*DatacrunchManager, error) {
	klog.V(2).Infof("Creating DataCrunch manager with config file: %s", configFile)
	
	// Read configuration file
	config, err := loadClusterConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster config: %v", err)
	}

	// Create API client using the SDK
	client, err := newDatacrunchSDKClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create DataCrunch client: %v", err)
	}

	// Create resource limiter
	resourceLimiter := &cloudprovider.ResourceLimiter{
		MinLimits: map[string]int64{
			cloudprovider.ResourceNameCores:   1,
			cloudprovider.ResourceNameMemory:  1 * 1024 * 1024 * 1024, // 1GB
		},
		MaxLimits: map[string]int64{
			cloudprovider.ResourceNameCores:   1000,
			cloudprovider.ResourceNameMemory:  1024 * 1024 * 1024 * 1024, // 1TB
		},
	}

	manager := &DatacrunchManager{
		client:         client,
		clusterConfig:  config,
		resourceLimiter: resourceLimiter,
	}

	klog.V(2).Infof("DataCrunch manager created successfully with %d node groups", len(config.NodeConfigs))
	return manager, nil
}

// loadClusterConfig loads cluster configuration from file
func loadClusterConfig(configFile string) (*ClusterConfig, error) {
	klog.V(4).Infof("Loading cluster config from: %s", configFile)
	
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configFile, err)
	}

	var config ClusterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	// Validate configuration
	if config.ProjectId == "" {
		return nil, fmt.Errorf("project_id is required in cluster config")
	}

	if len(config.NodeConfigs) == 0 {
		return nil, fmt.Errorf("at least one node group must be configured")
	}

	klog.V(4).Infof("Loaded cluster config: project_id=%s, node_groups=%d", 
		config.ProjectId, len(config.NodeConfigs))
	
	return &config, nil
}

// newDatacrunchSDKClient creates a new DataCrunch SDK client
func newDatacrunchSDKClient() (*datacrunch.Client, error) {
	clientID := os.Getenv("DATACRUNCH_CLIENT_ID")
	clientSecret := os.Getenv("DATACRUNCH_CLIENT_SECRET")
	apiURL := os.Getenv("DATACRUNCH_API_URL")
	
	if clientID == "" {
		return nil, fmt.Errorf("DATACRUNCH_CLIENT_ID environment variable is required")
	}
	
	if clientSecret == "" {
		return nil, fmt.Errorf("DATACRUNCH_CLIENT_SECRET environment variable is required")
	}
	
	if apiURL == "" {
		apiURL = "https://api.datacrunch.io/v1" // Default to production
	}

	config := &datacrunch.Config{
		BaseURL:      apiURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30 * time.Second,
	}

	client := datacrunch.New(config)
	klog.V(4).Infof("Created DataCrunch SDK client for API: %s", apiURL)
	return client, nil
}

// GetResourceLimiter returns the resource limiter for this manager
func (dm *DatacrunchManager) GetResourceLimiter() *cloudprovider.ResourceLimiter {
	return dm.resourceLimiter
}

// GetClusterConfig returns the cluster configuration
func (dm *DatacrunchManager) GetClusterConfig() *ClusterConfig {
	return dm.clusterConfig
}

// GetNodeConfig returns the node configuration for a specific node group
func (dm *DatacrunchManager) GetNodeConfig(nodeGroupName string) (*NodeConfig, error) {
	config, exists := dm.clusterConfig.NodeConfigs[nodeGroupName]
	if !exists {
		return nil, fmt.Errorf("node group %s not found in configuration", nodeGroupName)
	}
	return config, nil
}

// createInstance creates a new instance with the specified configuration
func (dm *DatacrunchManager) createInstance(nodeConfig *NodeConfig, nodeGroup, instanceType, location string) (*DatacrunchInstance, error) {
	klog.V(2).Infof("Creating instance for node group %s: type=%s, location=%s", 
		nodeGroup, instanceType, location)

	// Generate unique instance ID and hostname
	timestamp := time.Now().Unix()
	hostname := fmt.Sprintf("%s-%d", nodeGroup, timestamp)

	// Create instance input using the SDK
	input := &instance.CreateInstanceInput{
		ServerType:  instanceType,
		Location:    location,
		Image:       nodeConfig.ImageType,
		SSHKeys:     nodeConfig.SSHKeyIDs,
		Hostname:    hostname,
		Description: fmt.Sprintf("Cluster autoscaler instance for %s", nodeGroup),
		UserData:    "", // Will be set by wrapper with environment variables
	}

	// Make API call to create instance using the SDK
	sdkInstance, err := dm.client.Instance.CreateInstance(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %v", err)
	}

	klog.V(2).Infof("Successfully created instance %s (%s) for node group %s", 
		sdkInstance.ID, sdkInstance.Hostname, nodeGroup)
	
	return sdkInstance, nil
}

// DatacrunchInstance is an alias for the SDK instance type
type DatacrunchInstance = instance.Instance