package instance

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/testutil"
)

func TestCreateInstance(t *testing.T) {
	tests := []struct {
		name           string
		input          *CreateInstanceInput
		serverResponse func(*testutil.TestServer)
		expectedError  string
	}{
		{
			name: "successful creation",
			input: &CreateInstanceInput{
				InstanceType: "v1.small",
				Image:        "ubuntu-20.04",
				SSHKeyIDs:    []string{"key-123"},
				Hostname:     "test-instance",
				LocationCode: "us-east-1",
				OSVolume: &OSVolume{
					Name: "root",
					Size: 50,
				},
				IsSpot:   false,
				Contract: "hourly",
				Pricing:  "standard",
			},
			serverResponse: func(ts *testutil.TestServer) {
				response := map[string]interface{}{
					"id":            "inst-new123",
					"name":          "test-instance",
					"status":        "creating",
					"instance_type": "v1.small",
					"location":      "us-east-1",
					"created_at":    time.Now().Format(time.RFC3339),
				}
				ts.SetJSONResponse(201, response)
			},
		},
		{
			name: "validation error",
			input: &CreateInstanceInput{
				// Missing required fields
				InstanceType: "",
				Image:        "",
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetErrorResponse(400, "Missing required fields: instance_type, image")
			},
			expectedError: "400",
		},
		{
			name: "unauthorized error",
			input: &CreateInstanceInput{
				InstanceType: "v1.small",
				Image:        "ubuntu-20.04",
				LocationCode: "us-east-1",
			},
			serverResponse: func(ts *testutil.TestServer) {
				ts.SetErrorResponse(401, "Invalid authentication credentials")
			},
			expectedError: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would be a real implementation test
			// For now, we'll test the input validation and structure

			// Validate required fields
			if tt.input.InstanceType == "" && tt.expectedError == "" {
				t.Error("Expected instance_type to be required")
			}
			if tt.input.Image == "" && tt.expectedError == "" {
				t.Error("Expected image to be required")
			}
			if tt.input.LocationCode == "" && tt.expectedError == "" {
				t.Error("Expected location_code to be required")
			}

			// Test JSON marshaling
			jsonData, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			// Validate JSON structure
			var unmarshaled map[string]interface{}
			err = json.Unmarshal(jsonData, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Check expected fields are present (when not empty)
			if tt.input.InstanceType != "" {
				if unmarshaled["instance_type"] != tt.input.InstanceType {
					t.Errorf("Expected instance_type %s in JSON, got %v", tt.input.InstanceType, unmarshaled["instance_type"])
				}
			}
			if tt.input.Image != "" {
				if unmarshaled["image"] != tt.input.Image {
					t.Errorf("Expected image %s in JSON, got %v", tt.input.Image, unmarshaled["image"])
				}
			}
		})
	}
}

func TestListInstancesResponse(t *testing.T) {
	// Test response unmarshaling
	responseJSON := `{
		"id": "inst-123456",
		"ip": "192.168.1.100",
		"status": "running",
		"instance_type": "v1.small",
		"location": {
			"code": "us-east-1",
			"name": "US East 1"
		},
		"created_at": "2023-01-01T12:00:00Z",
		"cpu": {
			"description": "2 vCPUs",
			"number_of_cores": 2
		},
		"gpu": {
			"description": "No GPU",
			"number_of_gpus": 0
		},
		"gpu_memory": {
			"description": "No GPU Memory",
			"size_in_gigabytes": 0
		},
		"memory": {
			"description": "4 GB RAM",
			"size_in_gigabytes": 4
		},
		"storage": {
			"description": "50 GB SSD"
		},
		"hostname": "test-instance",
		"price_per_hour": 0.05,
		"is_spot": false
	}`

	var response ListInstancesResponse
	err := json.Unmarshal([]byte(responseJSON), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Validate fields
	if response.ID != "inst-123456" {
		t.Errorf("Expected ID inst-123456, got %s", response.ID)
	}
	if response.IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", response.IP)
	}
	if response.Location.Code != "us-east-1" {
		t.Errorf("Expected location code us-east-1, got %s", response.Location.Code)
	}
	if response.CPU.NumberOfCores != 2 {
		t.Errorf("Expected 2 CPU cores, got %d", response.CPU.NumberOfCores)
	}
}

func TestVolume(t *testing.T) {
	tests := []struct {
		name     string
		volume   *Volume
		expected string
	}{
		{
			name: "volume with all fields",
			volume: &Volume{
				Name: "data-volume",
				Size: 100,
				Type: "NVMe",
			},
			expected: `{"name":"data-volume","size":100,"type":"NVMe"}`,
		},
		{
			name: "volume without type",
			volume: &Volume{
				Name: "basic-volume",
				Size: 50,
			},
			expected: `{"name":"basic-volume","size":50}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.volume)
			if err != nil {
				t.Fatalf("Failed to marshal volume: %v", err)
			}

			if string(jsonData) != tt.expected {
				t.Errorf("Expected JSON %s, got %s", tt.expected, string(jsonData))
			}
		})
	}
}

func TestOSVolume(t *testing.T) {
	osVolume := &OSVolume{
		Name: "root",
		Size: 20,
	}

	jsonData, err := json.Marshal(osVolume)
	if err != nil {
		t.Fatalf("Failed to marshal OS volume: %v", err)
	}

	expected := `{"name":"root","size":20}`
	if string(jsonData) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(jsonData))
	}
}

// Integration test that would test actual API calls
func TestInstanceService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test server
	ts := testutil.NewTestServer(t)
	defer ts.Close()

	// Mock successful response
	ts.SetJSONResponse(200, testutil.MockAPIResponses.InstanceList)

	// This would be where we'd test actual service method calls
	// For now, we'll test the server setup
	resp, err := http.Get(ts.URL + "/api/v1/instances")
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify request was captured
	if ts.GetRequestCount() != 1 {
		t.Errorf("Expected 1 request to be captured, got %d", ts.GetRequestCount())
	}

	lastReq := ts.GetLastRequest()
	if lastReq.URL.Path != "/api/v1/instances" {
		t.Errorf("Expected request path /api/v1/instances, got %s", lastReq.URL.Path)
	}
}

// Benchmark tests for performance
func BenchmarkCreateInstanceInput_Marshal(b *testing.B) {
	input := &CreateInstanceInput{
		InstanceType: "v1.small",
		Image:        "ubuntu-20.04",
		SSHKeyIDs:    []string{"key-123", "key-456"},
		Hostname:     "benchmark-instance",
		Description:  "Benchmark test instance",
		LocationCode: "us-east-1",
		OSVolume: &OSVolume{
			Name: "root",
			Size: 50,
		},
		IsSpot:   false,
		Contract: "hourly",
		Pricing:  "standard",
		Volumes: []Volume{
			{Name: "data", Size: 100, Type: "NVMe"},
			{Name: "logs", Size: 50},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(input)
		if err != nil {
			b.Fatalf("Failed to marshal input: %v", err)
		}
	}
}

func BenchmarkListInstancesResponse_Unmarshal(b *testing.B) {
	responseJSON := []byte(`{
		"id": "inst-123456",
		"ip": "192.168.1.100",
		"name": "benchmark-instance",
		"status": "running",
		"instance_type": "v1.small",
		"location": "us-east-1",
		"created_at": "2023-01-01T12:00:00Z"
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var response ListInstancesResponse
		err := json.Unmarshal(responseJSON, &response)
		if err != nil {
			b.Fatalf("Failed to unmarshal response: %v", err)
		}
	}
}
