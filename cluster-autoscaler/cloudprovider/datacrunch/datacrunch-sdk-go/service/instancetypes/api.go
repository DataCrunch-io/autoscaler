package instancetypes

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

// InstanceTypeResponse represents an instance type
type InstanceTypeResponse struct {
	BestFor         []string `json:"best_for"`
	CPU             CPU      `json:"cpu"`
	DeployWarning   string   `json:"deploy_warning"`
	Description     string   `json:"description"`
	GPU             GPU      `json:"gpu"`
	GPUMemory       Memory   `json:"gpu_memory"`
	ID              string   `json:"id"`
	InstanceType    string   `json:"instance_type"`
	Memory          Memory   `json:"memory"`
	Model           string   `json:"model"`
	Name            string   `json:"name"`
	P2P             string   `json:"p2p"`
	PricePerHour    string   `json:"price_per_hour"`
	SpotPrice       string   `json:"spot_price"`
	DynamicPrice    string   `json:"dynamic_price"`
	MaxDynamicPrice string   `json:"max_dynamic_price"`
	Storage         Storage  `json:"storage"`
	Currency        string   `json:"currency"`
	Manufacturer    string   `json:"manufacturer"`
	DisplayName     string   `json:"display_name"`
}

// PriceHistoryEntry represents a single price history entry
type PriceHistoryEntry struct {
	Date                string  `json:"date"`
	FixedPricePerHour   float64 `json:"fixed_price_per_hour"`
	DynamicPricePerHour float64 `json:"dynamic_price_per_hour"`
	Currency            string  `json:"currency"`
}

// PriceHistoryResponse represents the price history response
type PriceHistoryResponse struct {
	H100 []PriceHistoryEntry `json:"H100"`
}

// ListInstanceTypes lists all available instance types
func (c *InstanceTypes) ListInstanceTypes(ctx context.Context) ([]*InstanceTypeResponse, error) {
	op := &request.Operation{
		Name:       "ListInstanceTypes",
		HTTPMethod: "GET",
		HTTPPath:   "/instance-types",
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
	var instanceTypes []*InstanceTypeResponse
	if err := json.NewDecoder(resp.Body).Decode(&instanceTypes); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return instanceTypes, nil
}

// GetInstanceTypePriceHistory gets the price history for instance types
func (c *InstanceTypes) GetInstanceTypePriceHistory(ctx context.Context) (*PriceHistoryResponse, error) {
	op := &request.Operation{
		Name:       "GetInstanceTypePriceHistory",
		HTTPMethod: "GET",
		HTTPPath:   "/instance-types/price-history",
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
	var priceHistory PriceHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceHistory); err != nil {
		return nil, dcerr.New("SerializationError", "failed to decode response", err)
	}

	return &priceHistory, nil
}
