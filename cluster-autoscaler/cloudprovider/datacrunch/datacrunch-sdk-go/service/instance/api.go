package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

const (
	// ServiceName is the name of the service
	ServiceName = "instance"
)

// Volume represents a storage volume configuration
type Volume struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	Type string `json:"type,omitempty"` // NVMe, etc.
}

// OSVolume represents OS volume configuration
type OSVolume struct {
	Name string `json:"name"`
	Size int    `json:"size"`
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
	Coupon          string    `json:"coupon,omitempty"`
	Volumes         []Volume  `json:"volumes,omitempty"`
	ExistingVolumes []string  `json:"existing_volumes,omitempty"`
	Contract        string    `json:"contract"`
	Pricing         string    `json:"pricing"`
}

// Instance represents a compute instance
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
	Location        Location `json:"location"`
	PricePerHour    float64  `json:"price_per_hour"`
	IsSpot          bool     `json:"is_spot"`
	InstanceType    string   `json:"instance_type"`
	Image           string   `json:"image"`
	OSName          string   `json:"os_name"`
	StartupScriptID string   `json:"startup_script_id"`
	SSHKeyIDs       []string `json:"ssh_key_ids"`
	OSVolumeID      string   `json:"os_volume_id"`
	JupyterToken    string   `json:"jupyter_token"`
	Contract        string   `json:"contract"`
	Pricing         string   `json:"pricing"`
}

// CPU represents CPU configuration
type CPU struct {
	Description   string `json:"description"`
	NumberOfCores int    `json:"number_of_cores"`
}

// GPU represents GPU configuration
type GPU struct {
	Description  string `json:"description"`
	NumberOfGPUs int    `json:"number_of_gpus"`
}

// Memory represents memory configuration
type Memory struct {
	Description     string `json:"description"`
	SizeInGigabytes int    `json:"size_in_gigabytes"`
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

// ListInstances lists all instances
func (c *Instance) ListInstances(ctx context.Context) ([]*ListInstancesResponse, error) {
	op := &request.Operation{
		Name:       "ListInstances",
		HTTPMethod: "GET",
		HTTPPath:   "/instances",
	}

	req := c.NewRequest(op, nil, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return nil, dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return nil, req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return nil, dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Parse response
	var instances []*ListInstancesResponse
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	// Log the response
	log.Printf("Successfully retrieved %d instances", len(instances))
	return instances, nil
}

// CreateInstance creates a new compute instance
func (c *Instance) CreateInstance(ctx context.Context, input *CreateInstanceInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateInstance",
		HTTPMethod: "POST",
		HTTPPath:   "/instances",
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Set the request body
	body, err := json.Marshal(input)
	if err != nil {
		return "", dcerr.New("SerializationError", "failed to marshal request body", err)
	}
	req.SetBufferBody(body)

	// Set content type header
	req.HTTPRequest.Header.Set("Content-Type", "application/json")

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return "", req.Error
	}

	// Log the request URL and payload
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())
	log.Printf("Request payload: %s", string(body))
	log.Printf("Request headers: %v", req.HTTPRequest.Header)

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return "", dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return "", req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return "", dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	// Read the response body which should be the instance ID
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", dcerr.New("SerializationError", "failed to read response body", err)
	}

	instanceID := string(body)
	log.Printf("Successfully created instance: %s", instanceID)
	return instanceID, nil
}

// PerformInstanceAction performs an action on an instance
func (c *Instance) PerformInstanceAction(ctx context.Context, input *InstanceActionInput) error {
	op := &request.Operation{
		Name:       "PerformInstanceAction",
		HTTPMethod: "PUT",
		HTTPPath:   "/instances",
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Run the Build handlers
	req.Handlers.Build.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Log the request URL and payload
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}

	// Send the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req.HTTPRequest)
	if err != nil {
		return dcerr.New("RequestError", "failed to send request", err)
	}
	defer resp.Body.Close()

	// Set the response
	req.HTTPResponse = resp

	// Run the Complete handlers
	req.Handlers.Complete.Run(req)
	if req.Error != nil {
		return req.Error
	}

	// Check response status
	if resp.StatusCode != http.StatusAccepted {
		// Read and log the response body for error cases
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error response body: %s", string(body))
		return dcerr.NewRequestFailure(
			dcerr.New("RequestError", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil),
			resp.StatusCode,
			"",
		)
	}

	log.Printf("Successfully performed action %s on instance %s", input.Action, input.ID)
	return nil
}
