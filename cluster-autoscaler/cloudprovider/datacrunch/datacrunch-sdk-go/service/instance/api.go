package instance

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol/restjson"
)

const (
	// ServiceName is the name of the service
	ServiceName = "instance"
)

// Volume represents a storage volume configuration
type Volume struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type,omitempty"` // NVMe, etc.
}

// OSVolume represents OS volume configuration
type OSVolume struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// CreateInstanceInput represents the input for creating a new instance
type CreateInstanceInput struct {
	InstanceType    string    `json:"instance_type"`
	Image           string    `json:"image"`
	SSHKeyIDs       []string  `json:"ssh_key_ids"`
	StartupScriptID string    `json:"startup_script_id,omitempty"`
	Hostname        string    `json:"hostname,omitempty"`
	Description     string    `json:"description,omitempty"`
	LocationCode    string    `json:"location_code"`
	OSVolume        *OSVolume `json:"os_volume,omitempty"`
	IsSpot          bool      `json:"is_spot"`
	Volumes         []Volume  `json:"volumes,omitempty"`
	ExistingVolumes []string  `json:"existing_volumes,omitempty"`
	Contract        string    `json:"contract"`
	Pricing         string    `json:"pricing"`
}

// ListInstancesResponse represents a compute instance
type ListInstancesResponse struct {
	ID              string   `json:"id"`
	IP              string   `json:"ip"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"created_at"`
	CPU             CPU      `json:"cpu"`
	GPU             GPU      `json:"gpu"`
	GPUMemory       Memory   `json:"gpu_memory"`
	Memory          Memory   `json:"memory"`
	Storage         Storage  `json:"storage"`
	Hostname        string   `json:"hostname"`
	Description     string   `json:"description"`
	Location        string   `json:"location"`
	PricePerHour    float64  `json:"price_per_hour"`
	IsSpot          bool     `json:"is_spot"`
	InstanceType    string   `json:"instance_type"`
	Image           string   `json:"image"`
	OSName          string   `json:"os_name"`
	StartupScriptID *string  `json:"startup_script_id"` // Changed to pointer for null values
	SSHKeyIDs       []string `json:"ssh_key_ids"`
	OSVolumeID      string   `json:"os_volume_id"`
	JupyterToken    *string  `json:"jupyter_token"` // Changed to pointer for null values
	Contract        string   `json:"contract"`
	Pricing         string   `json:"pricing"`
}

// CPU represents CPU configuration
type CPU struct {
	Description   string `json:"description"`
	NumberOfCores int64  `json:"number_of_cores"`
}

// GPU represents GPU configuration
type GPU struct {
	Description  string `json:"description"`
	NumberOfGPUs int64  `json:"number_of_gpus"`
}

// Memory represents memory configuration
type Memory struct {
	Description     string `json:"description"`
	SizeInGigabytes int64  `json:"size_in_gigabytes"`
}

// Storage represents storage configuration
type Storage struct {
	Description string `json:"description"`
}

// Location represents instance location
type Location struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
}

// InstanceActionType represents the type of action to perform on an instance
type InstanceActionType string

const (
	InstanceActionBoot          InstanceActionType = "boot"
	InstanceActionStart         InstanceActionType = "start"
	InstanceActionShutdown      InstanceActionType = "shutdown"
	InstanceActionDelete        InstanceActionType = "delete"
	InstanceActionDiscontinue   InstanceActionType = "discontinue"
	InstanceActionHibernate     InstanceActionType = "hibernate"
	InstanceActionConfigureSpot InstanceActionType = "configure_spot"
	InstanceActionForceShutdown InstanceActionType = "force_shutdown"
)

// InstanceActionInput represents the input for performing an action on an instance
type InstanceActionInput struct {
	Action    InstanceActionType `json:"action"`
	ID        string             `json:"id"`
	VolumeIDs []string           `json:"volume_ids,omitempty"`
}

// InstanceStatus represents the status of an instance
type InstanceStatus string

const (
	InstanceStatusRunning      InstanceStatus = "running"
	InstanceStatusProvisioning InstanceStatus = "provisioning"
	InstanceStatusOffline      InstanceStatus = "offline"
	InstanceStatusDiscontinued InstanceStatus = "discontinued"
	InstanceStatusUnknown      InstanceStatus = "unknown"
	InstanceStatusOrdered      InstanceStatus = "ordered"
	InstanceStatusNotFound     InstanceStatus = "notfound"
	InstanceStatusNew          InstanceStatus = "new"
	InstanceStatusError        InstanceStatus = "error"
	InstanceStatusDeleting     InstanceStatus = "deleting"
	InstanceStatusValidating   InstanceStatus = "validating"
)

type BillingContract string

const (
	BillingContractPayAsYouGo BillingContract = "PAY_AS_YOU_GO"
	BillingContractLongTerm   BillingContract = "LONG_TERM"
	BillingContractSpot       BillingContract = "SPOT"
)

type BillingPrice string

const (
	BillingPriceDynamic BillingPrice = "DYNAMIC_PRICE"
	BillingPriceFixed   BillingPrice = "FIXED_PRICE"
)

// ListInstancesInput represents the input for listing instances
type ListInstancesInput struct {
	Status string `location:"querystring" locationName:"status"`
}

// ListInstances lists all instances
func (c *Instance) ListInstances(input *ListInstancesInput) ([]*ListInstancesResponse, error) {
	op := &request.Operation{
		Name:       "ListInstances",
		HTTPMethod: "GET",
		HTTPPath:   "/instances",
	}

	var instances []*ListInstancesResponse
	req := c.newRequest(op, input, &instances)

	return instances, req.Send()
}

// CreateInstance creates a new compute instance
func (c *Instance) CreateInstance(input *CreateInstanceInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateInstance",
		HTTPMethod: "POST",
		HTTPPath:   "/instances",
	}

	var instanceID string
	req := c.newRequest(op, input, &instanceID)

	// Replace the JSON unmarshaler with string unmarshaler for the instance ID response
	// The default error handler will still run first
	req.Handlers.Unmarshal.RemoveByName("datacrunchsdk.restjson.Unmarshal")
	req.Handlers.Unmarshal.PushBackNamed(restjson.StringUnmarshalHandler)

	err := req.Send()
	if err != nil {
		return "", err
	}

	return instanceID, nil
}

// PerformInstanceAction performs an action on an instance
func (c *Instance) PerformInstanceAction(input *InstanceActionInput) error {
	op := &request.Operation{
		Name:       "PerformInstanceAction",
		HTTPMethod: "PUT",
		HTTPPath:   "/instances",
	}

	req := c.newRequest(op, input, nil)

	return req.Send()
}
