package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// TestClientConfig holds configuration for creating test clients
type TestClientConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
}

// NewTestClientConfig creates a standard test client configuration for the given mock server
func NewTestClientConfig(mockServer *MockServer) TestClientConfig {
	return TestClientConfig{
		BaseURL:      mockServer.URL(),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	}
}

// Mock types to avoid circular imports
type Instance struct {
	ID              string                 `json:"id"`
	InstanceType    string                 `json:"instance_type"`
	Image           string                 `json:"image"`
	PricePerHour    float64                `json:"price_per_hour"`
	Hostname        string                 `json:"hostname"`
	Description     string                 `json:"description"`
	IP              *string                `json:"ip"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	SSHKeyIDs       []string               `json:"ssh_key_ids"`
	CPU             map[string]interface{} `json:"cpu"`
	GPU             map[string]interface{} `json:"gpu"`
	Memory          map[string]interface{} `json:"memory"`
	Storage         map[string]interface{} `json:"storage"`
	OSVolumeID      *string                `json:"os_volume_id"`
	GPUMemory       map[string]interface{} `json:"gpu_memory"`
	Location        string                 `json:"location"`
	IsSpot          bool                   `json:"is_spot"`
	OSName          string                 `json:"os_name"`
	StartupScriptID *string                `json:"startup_script_id"`
	JupyterToken    *string                `json:"jupyter_token"`
	Contract        string                 `json:"contract"`
	Pricing         string                 `json:"pricing"`
}

type CreateInstanceRequest struct {
	InstanceType    string                 `json:"instance_type"`
	Image           string                 `json:"image"`
	Hostname        string                 `json:"hostname"`
	Description     string                 `json:"description"`
	SSHKeyIDs       []string               `json:"ssh_key_ids,omitempty"`
	Location        string                 `json:"location,omitempty"`
	StartupScriptID *string                `json:"startup_script_id,omitempty"`
	Volumes         []VolumeCreateRequest  `json:"volumes,omitempty"`
	ExistingVolumes []string               `json:"existing_volumes,omitempty"`
	OSVolume        *OSVolumeCreateRequest `json:"os_volume,omitempty"`
	IsSpot          bool                   `json:"is_spot,omitempty"`
	Coupon          *string                `json:"coupon,omitempty"`
}

type VolumeCreateRequest struct {
	Size int    `json:"size"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type OSVolumeCreateRequest struct {
	Size int    `json:"size"`
	Type string `json:"type"`
}

type InstanceActionRequest struct {
	IDList    any      `json:"id_list"`
	Action    string   `json:"action"`
	VolumeIDs []string `json:"volume_ids,omitempty"`
}

type Balance struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

type Location struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Available   bool   `json:"available"`
}

type Volume struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	InstanceID *string   `json:"instance_id"`
}

type StartupScript struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Script    string    `json:"script"`
	CreatedAt time.Time `json:"created_at"`
}

type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	Environment map[string]string `json:"environment,omitempty"`
}

const (
	StatusRunning = "RUNNING"
	StatusPending = "PENDING"
	LocationFIN01 = "FIN-01"
)

// MockServer provides a test HTTP server for mocking DataCrunch API responses
type MockServer struct {
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
	mu       sync.RWMutex
	sshKeys  map[string]SSHKey // Store created SSH keys
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	ms := &MockServer{
		handlers: make(map[string]http.HandlerFunc),
		sshKeys:  make(map[string]SSHKey),
	}

	ms.server = httptest.NewServer(http.HandlerFunc(ms.handleRequest))
	return ms
}

// URL returns the mock server URL
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// Close shuts down the mock server
func (ms *MockServer) Close() {
	ms.server.Close()
}

// SetHandler sets a custom handler for a specific path and method
func (ms *MockServer) SetHandler(method, path string, handler http.HandlerFunc) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	key := method + " " + path
	ms.handlers[key] = handler
}

// handleRequest routes requests to appropriate handlers
func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	key := r.Method + " " + r.URL.Path
	handler, exists := ms.handlers[key]
	ms.mu.RUnlock()

	if exists {
		handler(w, r)
		return
	}

	// Default handlers
	switch {
	case r.Method == "POST" && r.URL.Path == "/oauth2/token":
		ms.handleAuth(w, r)
	case r.Method == "GET" && r.URL.Path == "/instances":
		ms.handleGetInstances(w, r)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/instances/"):
		ms.handleGetInstance(w, r)
	case r.Method == "POST" && r.URL.Path == "/instances":
		ms.handleCreateInstance(w, r)
	case r.Method == "POST" && r.URL.Path == "/instances/action":
		ms.handleInstanceAction(w, r)
	case r.Method == "GET" && r.URL.Path == "/balance":
		ms.handleGetBalance(w, r)
	case r.Method == "GET" && r.URL.Path == "/sshkeys":
		ms.handleGetSSHKeys(w, r)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/sshkeys/"):
		ms.handleGetSSHKey(w, r)
	case r.Method == "POST" && r.URL.Path == "/sshkeys":
		ms.handleCreateSSHKey(w, r)
	case r.Method == "GET" && r.URL.Path == "/locations":
		ms.handleGetLocations(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (ms *MockServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var grantType, clientID, clientSecret string

	// Check content type and parse accordingly
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse JSON body
		var tokenReq struct {
			GrantType    string `json:"grant_type"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&tokenReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_json"})
			return
		}
		grantType = tokenReq.GrantType
		clientID = tokenReq.ClientID
		clientSecret = tokenReq.ClientSecret
	} else {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		grantType = r.FormValue("grant_type")
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if grantType == "" || clientID == "" || clientSecret == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request"})
		return
	}

	// Mock successful authentication
	response := map[string]interface{}{
		"access_token":  "mock_access_token_12345",
		"refresh_token": "mock_refresh_token_67890",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"scope":         "read write",
	}

	json.NewEncoder(w).Encode(response)
}

func (ms *MockServer) handleGetInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	instances := []Instance{
		{
			ID:           "inst_123",
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			PricePerHour: 0.50,
			Hostname:     "test-instance",
			Description:  "Test instance",
			Status:       StatusRunning,
			CreatedAt:    time.Now(),
			SSHKeyIDs:    []string{"key_123"},
			CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
			GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
			Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
			Storage:      map[string]interface{}{"description": "100GB SSD"},
			GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
			Location:     LocationFIN01,
			IsSpot:       false,
			OSName:       "test-instance-os",
			Contract:     "PAY_AS_YOU_GO",
			Pricing:      "FIXED_PRICE",
		},
	}

	json.NewEncoder(w).Encode(instances)
}

func (ms *MockServer) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract instance ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	instanceID := parts[2]

	instance := Instance{
		ID:           instanceID,
		InstanceType: "1V100.6V",
		Image:        "ubuntu-24.04-cuda-12.8-open-docker",
		PricePerHour: 0.50,
		Hostname:     "test-instance",
		Description:  "Test instance",
		Status:       StatusRunning,
		CreatedAt:    time.Now(),
		SSHKeyIDs:    []string{"key_123"},
		CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
		GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
		Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
		Storage:      map[string]interface{}{"description": "100GB SSD"},
		GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
		Location:     LocationFIN01,
		IsSpot:       false,
		OSName:       instanceID + "-os",
		Contract:     "PAY_AS_YOU_GO",
		Pricing:      "FIXED_PRICE",
	}

	json.NewEncoder(w).Encode(instance)
}

func (ms *MockServer) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	instance := Instance{
		ID:           "inst_new_123",
		InstanceType: req.InstanceType,
		Image:        req.Image,
		PricePerHour: 0.50,
		Hostname:     req.Hostname,
		Description:  req.Description,
		Status:       StatusPending,
		CreatedAt:    time.Now(),
		SSHKeyIDs:    req.SSHKeyIDs,
		CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
		GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
		Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
		Storage:      map[string]interface{}{"description": "100GB SSD"},
		GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
		Location:     req.Location,
		IsSpot:       false,
		OSName:       "inst_new_123-os",
		Contract:     "PAY_AS_YOU_GO",
		Pricing:      "FIXED_PRICE",
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(instance)
}

func (ms *MockServer) handleInstanceAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req InstanceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Mock successful action
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (ms *MockServer) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	balance := Balance{
		Amount:   100.50,
		Currency: "USD",
	}

	json.NewEncoder(w).Encode(balance)
}

func (ms *MockServer) handleGetSSHKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	keys := []SSHKey{
		{
			ID:          "key_123",
			Name:        "Test Key",
			PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
			Fingerprint: "SHA256:abc123...",
			CreatedAt:   time.Now(),
		},
	}

	json.NewEncoder(w).Encode(keys)
}

func (ms *MockServer) handleGetLocations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	locations := []Location{
		{
			Code:        LocationFIN01,
			Name:        "Finland 01",
			Country:     "Finland",
			CountryCode: "FI",
			Available:   true,
		},
	}

	json.NewEncoder(w).Encode(locations)
}

func (ms *MockServer) handleGetSSHKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract SSH key ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	keyID := parts[2]

	ms.mu.RLock()
	key, exists := ms.sshKeys[keyID]
	ms.mu.RUnlock()

	if !exists {
		// Return a default SSH key for unknown IDs
		// For key_new_123, return data that matches the SSH key creation test
		if keyID == "key_new_123" {
			key = SSHKey{
				ID:          keyID,
				Name:        "My New Key",
				PublicKey:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB...",
				Fingerprint: "SHA256:generated...",
				CreatedAt:   time.Now(),
			}
		} else {
			key = SSHKey{
				ID:          keyID,
				Name:        "Mock SSH Key",
				PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
				Fingerprint: "SHA256:mock...",
				CreatedAt:   time.Now(),
			}
		}
	}

	// Return as an array (to match the real API)
	json.NewEncoder(w).Encode([]SSHKey{key})
}

func (ms *MockServer) handleCreateSSHKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Parse the request to get SSH key data
	var req struct {
		Name      string `json:"name"`
		PublicKey string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Generate ID and store the key
	keyID := "key_new_123"
	key := SSHKey{
		ID:          keyID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Fingerprint: "SHA256:generated...",
		CreatedAt:   time.Now(),
	}

	ms.mu.Lock()
	ms.sshKeys[keyID] = key
	ms.mu.Unlock()

	// Return just the ID as plain text (to match the real API)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(keyID))
}

// Note: CreateTestClient is implemented in test files to avoid circular imports

// ErrorResponse creates a mock error response
func ErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": message,
		"status":  statusCode,
	})
}
