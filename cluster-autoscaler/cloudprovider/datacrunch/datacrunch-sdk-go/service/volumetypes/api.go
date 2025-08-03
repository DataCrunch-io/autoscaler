package volumetypes

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

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
	var volumeTypes []*VolumeTypeResponse
	if err := json.NewDecoder(resp.Body).Decode(&volumeTypes); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return volumeTypes, nil
}
