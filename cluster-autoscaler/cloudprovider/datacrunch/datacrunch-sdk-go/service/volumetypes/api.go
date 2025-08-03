package volumetypes

import (
	"context"
	"log"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
)

// Price represents the pricing details for a volume type
type Price struct {
	PricePerMonthPerGB float64 `json:"price_per_month_per_gb"`
	CPSPerGB           float64 `json:"cps_per_gb"`
	Currency           string  `json:"currency"`
}

// VolumeTypeResponse represents a volume type
type VolumeTypeResponse struct {
	Type                 string `json:"type"`
	Price                Price  `json:"price"`
	IsSharedFS           bool   `json:"is_shared_fs"`
	BurstBandwidth       int    `json:"burst_bandwidth"`
	ContinuousBandwidth  int    `json:"continuous_bandwidth"`
	InternalNetworkSpeed int    `json:"internal_network_speed"`
	IOPS                 string `json:"iops"`
}

// ListVolumeTypes lists all available volume types
func (c *VolumeTypes) ListVolumeTypes(ctx context.Context) ([]*VolumeTypeResponse, error) {
	op := &request.Operation{
		Name:       "ListVolumeTypes",
		HTTPMethod: "GET",
		HTTPPath:   "/volume-types",
	}

	var volumeTypes []*VolumeTypeResponse
	req := c.NewRequest(op, nil, &volumeTypes)
	req.SetContext(ctx)

	// Log the request URL
	log.Printf("Sending request to: %s", req.HTTPRequest.URL.String())

	// Use the client's Send method which handles all the request/response lifecycle
	if err := req.Send(); err != nil {
		return nil, err
	}

	return volumeTypes, nil
}
