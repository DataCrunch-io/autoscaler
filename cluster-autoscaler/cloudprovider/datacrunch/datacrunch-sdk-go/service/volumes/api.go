package volumes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// Instance represents an instance attached to a volume
type Instance struct {
	ID                  string `json:"id"`
	AutoRentalExtension *bool  `json:"auto_rental_extension"`
	IP                  string `json:"ip"`
	InstanceType        string `json:"instance_type"`
	Status              string `json:"status"`
	OSVolumeID          string `json:"os_volume_id"`
	Hostname            string `json:"hostname"`
}

// LongTerm represents long-term contract details
type LongTerm struct {
	EndDate             string  `json:"end_date"`
	LongTermPeriod      string  `json:"long_term_period"`
	DiscountPercentage  float64 `json:"discount_percentage"`
	AutoRentalExtension bool    `json:"auto_rental_extension"`
	NextPeriodPrice     float64 `json:"next_period_price"`
	CurrentPeriodPrice  float64 `json:"current_period_price"`
}

// VolumeResponse represents a volume
type VolumeResponse struct {
	ID                       string     `json:"id"`
	InstanceID               string     `json:"instance_id"`
	Instances                []Instance `json:"instances"`
	Name                     string     `json:"name"`
	CreatedAt                string     `json:"created_at"`
	Status                   string     `json:"status"`
	Size                     int        `json:"size"`
	IsOSVolume               bool       `json:"is_os_volume"`
	Target                   string     `json:"target"`
	Type                     string     `json:"type"`
	Location                 string     `json:"location"`
	SSHKeyIDs                []string   `json:"ssh_key_ids"`
	PseudoPath               string     `json:"pseudo_path"`
	CreateDirectoryCommand   string     `json:"create_directory_command"`
	MountCommand             string     `json:"mount_command"`
	FilesystemToFstabCommand string     `json:"filesystem_to_fstab_command"`
	Contract                 string     `json:"contract"`
	BaseHourlyCost           float64    `json:"base_hourly_cost"`
	MonthlyPrice             float64    `json:"monthly_price"`
	Currency                 string     `json:"currency"`
	LongTerm                 *LongTerm  `json:"long_term"`
	DeletedAt                string     `json:"deleted_at,omitempty"`
}

// CreateVolumeInput represents input for creating a volume
type CreateVolumeInput struct {
	Type         string   `json:"type"`
	LocationCode string   `json:"location_code"`
	Size         int      `json:"size"`
	InstanceID   string   `json:"instance_id,omitempty"`
	InstanceIDs  []string `json:"instance_ids,omitempty"`
	Name         string   `json:"name"`
}

// VolumeActionInput represents input for performing an action on a volume
type VolumeActionInput struct {
	Action       string   `json:"action"`
	ID           string   `json:"id"`
	Size         int      `json:"size,omitempty"`
	InstanceID   string   `json:"instance_id,omitempty"`
	InstanceIDs  []string `json:"instance_ids,omitempty"`
	Name         string   `json:"name,omitempty"`
	Type         string   `json:"type,omitempty"`
	IsPermanent  bool     `json:"is_permanent,omitempty"`
	LocationCode string   `json:"location_code,omitempty"`
}

// ListVolumes lists all volumes
func (c *Volumes) ListVolumes(ctx context.Context) ([]*VolumeResponse, error) {
	op := &request.Operation{
		Name:       "ListVolumes",
		HTTPMethod: "GET",
		HTTPPath:   "/volumes",
	}

	var volumes []*VolumeResponse
	req := c.NewRequest(op, nil, &volumes)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return volumes, nil
}

// GetVolume gets a volume by ID
func (c *Volumes) GetVolume(ctx context.Context, id string) (*VolumeResponse, error) {
	op := &request.Operation{
		Name:       "GetVolume",
		HTTPMethod: "GET",
		HTTPPath:   fmt.Sprintf("/volumes/%s", id),
	}

	var volume VolumeResponse
	req := c.NewRequest(op, nil, &volume)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return &volume, nil
}

// CreateVolume creates a new volume
func (c *Volumes) CreateVolume(ctx context.Context, input *CreateVolumeInput) (string, error) {
	op := &request.Operation{
		Name:       "CreateVolume",
		HTTPMethod: "POST",
		HTTPPath:   "/volumes",
	}

	var volumeID string
	req := c.NewRequest(op, input, &volumeID)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return "", err
	}

	log.Printf("Successfully created volume: %s", volumeID)
	return volumeID, nil
}

// PerformVolumeAction performs an action on a volume
func (c *Volumes) PerformVolumeAction(ctx context.Context, input *VolumeActionInput) error {
	op := &request.Operation{
		Name:       "PerformVolumeAction",
		HTTPMethod: "PUT",
		HTTPPath:   "/volumes",
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return err
	}

	log.Printf("Successfully performed action %s on volume %s", input.Action, input.ID)
	return nil
}

// ListTrashVolumes lists all volumes in trash
func (c *Volumes) ListTrashVolumes(ctx context.Context) ([]*VolumeResponse, error) {
	op := &request.Operation{
		Name:       "ListTrashVolumes",
		HTTPMethod: "GET",
		HTTPPath:   "/volumes/trash",
	}

	var volumes []*VolumeResponse
	req := c.NewRequest(op, nil, &volumes)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return volumes, nil
}

// DeleteVolume deletes a volume by ID
func (c *Volumes) DeleteVolume(ctx context.Context, id string, isPermanent bool) error {
	op := &request.Operation{
		Name:       "DeleteVolume",
		HTTPMethod: "DELETE",
		HTTPPath:   fmt.Sprintf("/volumes/%s", id),
	}

	input := struct {
		IsPermanent bool `json:"is_permanent"`
	}{
		IsPermanent: isPermanent,
	}

	req := c.NewRequest(op, input, nil)
	req.SetContext(ctx)

	// Log the request URL and payload
	if body, err := json.Marshal(input); err == nil {
		log.Printf("Request payload: %s", string(body))
	}
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return err
	}

	log.Printf("Successfully deleted volume %s (permanent: %v)", id, isPermanent)
	return nil
}
