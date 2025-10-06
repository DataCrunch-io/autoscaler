package datacrunch

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/testutil"
)

// Test Balance Service
func TestBalanceService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get balance", func(t *testing.T) {
		ctx := context.Background()
		balance, err := client.Balance.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if balance == nil {
			t.Fatal("expected balance, got nil")
		}

		if balance.Amount != 100.50 {
			t.Errorf("expected amount 100.50, got %f", balance.Amount)
		}

		if balance.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", balance.Currency)
		}
	})
}

// Test SSH Keys Service
func TestSSHKeyService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all SSH keys", func(t *testing.T) {
		ctx := context.Background()
		keys, err := client.SSHKeys.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 SSH key, got %d", len(keys))
		}

		key := keys[0]
		if key.ID != "key_123" {
			t.Errorf("expected key ID 'key_123', got '%s'", key.ID)
		}

		if key.Name != "Test Key" {
			t.Errorf("expected key name 'Test Key', got '%s'", key.Name)
		}

		if key.PublicKey != "ssh-rsa AAAAB3NzaC1yc2E..." {
			t.Errorf("expected public key to be set correctly")
		}
	})
}

func TestSSHKeyService_GetByID(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for specific SSH key
	mockServer.SetHandler("GET", "/sshkeys/key_123", func(w http.ResponseWriter, r *http.Request) {
		key := testutil.SSHKey{
			ID:          "key_123",
			Name:        "Specific Test Key",
			PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
			Fingerprint: "SHA256:abc123...",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testutil.SSHKey{key})
	})

	t.Run("get SSH key by ID", func(t *testing.T) {
		ctx := context.Background()
		key, err := client.SSHKeys.GetByID(ctx, "key_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if key == nil {
			t.Fatal("expected SSH key, got nil")
		}

		if key.ID != "key_123" {
			t.Errorf("expected key ID 'key_123', got '%s'", key.ID)
		}

		if key.Name != "Specific Test Key" {
			t.Errorf("expected key name 'Specific Test Key', got '%s'", key.Name)
		}
	})
}

func TestSSHKeyService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for SSH key creation
	mockServer.SetHandler("POST", "/sshkeys", func(w http.ResponseWriter, r *http.Request) {
		var req CreateSSHKeyRequest
		json.NewDecoder(r.Body).Decode(&req)

		key := testutil.SSHKey{
			ID:          "key_new_123",
			Name:        req.Name,
			PublicKey:   req.PublicKey,
			Fingerprint: "SHA256:generated...",
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(key.ID))
	})

	t.Run("create SSH key", func(t *testing.T) {
		req := CreateSSHKeyRequest{
			Name:      "My New Key",
			PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB...",
		}

		ctx := context.Background()
		key, err := client.SSHKeys.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if key == nil {
			t.Fatal("expected SSH key, got nil")
		}

		if key.Name != req.Name {
			t.Errorf("expected key name '%s', got '%s'", req.Name, key.Name)
		}

		if key.PublicKey != req.PublicKey {
			t.Errorf("expected public key to match request")
		}
	})
}

func TestSSHKeyService_Delete(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for SSH key deletion
	mockServer.SetHandler("DELETE", "/sshkeys/key_123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	t.Run("delete SSH key", func(t *testing.T) {
		ctx := context.Background()
		err := client.SSHKeys.Delete(ctx, "key_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Test Locations Service
func TestLocationService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all locations", func(t *testing.T) {
		ctx := context.Background()
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(locations) != 1 {
			t.Errorf("expected 1 location, got %d", len(locations))
		}

		location := locations[0]
		if location.Code != LocationFIN01 {
			t.Errorf("expected location code '%s', got '%s'", LocationFIN01, location.Code)
		}

		if location.Name != "Finland 01" {
			t.Errorf("expected location name 'Finland 01', got '%s'", location.Name)
		}

		if location.Country != "Finland" {
			t.Errorf("expected country 'Finland', got '%s'", location.Country)
		}

		if !location.Available {
			t.Error("expected location to be available")
		}
	})
}

// Test Volumes Service
func TestVolumeService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for volumes
	mockServer.SetHandler("GET", "/volumes", func(w http.ResponseWriter, r *http.Request) {
		volumes := []testutil.Volume{
			{
				ID:     "vol_123",
				Name:   "Test Volume",
				Size:   100,
				Type:   "SSD",
				Status: "available",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(volumes)
	})

	t.Run("get all volumes", func(t *testing.T) {
		ctx := context.Background()
		volumes, err := client.Volumes.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(volumes) != 1 {
			t.Errorf("expected 1 volume, got %d", len(volumes))
		}

		volume := volumes[0]
		if volume.ID != "vol_123" {
			t.Errorf("expected volume ID 'vol_123', got '%s'", volume.ID)
		}

		if volume.Size != 100 {
			t.Errorf("expected volume size 100, got %d", volume.Size)
		}
	})
}

func TestVolumeService_GetByID(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for specific volume
	mockServer.SetHandler("GET", "/volumes/vol_123", func(w http.ResponseWriter, r *http.Request) {
		volume := testutil.Volume{
			ID:     "vol_123",
			Name:   "Specific Volume",
			Size:   200,
			Type:   "SSD",
			Status: "in-use",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(volume)
	})

	t.Run("get volume by ID", func(t *testing.T) {
		ctx := context.Background()
		volume, err := client.Volumes.GetByID(ctx, "vol_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if volume == nil {
			t.Fatal("expected volume, got nil")
		}

		if volume.ID != "vol_123" {
			t.Errorf("expected volume ID 'vol_123', got '%s'", volume.ID)
		}

		if volume.Size != 200 {
			t.Errorf("expected volume size 200, got %d", volume.Size)
		}
	})
}

// Test Startup Scripts Service
func TestStartupScriptService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for startup scripts
	mockServer.SetHandler("GET", "/startup-scripts", func(w http.ResponseWriter, r *http.Request) {
		scripts := []testutil.StartupScript{
			{
				ID:     "script_123",
				Name:   "Test Script",
				Script: "#!/bin/bash\necho 'Hello World'",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(scripts)
	})

	t.Run("get all startup scripts", func(t *testing.T) {
		ctx := context.Background()
		scripts, err := client.StartupScripts.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(scripts) != 1 {
			t.Errorf("expected 1 startup script, got %d", len(scripts))
		}

		script := scripts[0]
		if script.ID != "script_123" {
			t.Errorf("expected script ID 'script_123', got '%s'", script.ID)
		}

		if script.Name != "Test Script" {
			t.Errorf("expected script name 'Test Script', got '%s'", script.Name)
		}
	})
}

func TestStartupScriptService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for startup script creation
	mockServer.SetHandler("POST", "/startup-scripts", func(w http.ResponseWriter, r *http.Request) {
		var req CreateStartupScriptRequest
		json.NewDecoder(r.Body).Decode(&req)

		script := testutil.StartupScript{
			ID:     "script_new_123",
			Name:   req.Name,
			Script: req.Script,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(script)
	})

	t.Run("create startup script", func(t *testing.T) {
		req := CreateStartupScriptRequest{
			Name:   "Setup Script",
			Script: "#!/bin/bash\nnpm install",
		}

		ctx := context.Background()
		script, err := client.StartupScripts.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if script == nil {
			t.Fatal("expected startup script, got nil")
		}

		if script.Name != req.Name {
			t.Errorf("expected script name '%s', got '%s'", req.Name, script.Name)
		}

		if script.Script != req.Script {
			t.Errorf("expected script content to match request")
		}
	})
}

// Test Containers Service
func TestContainerService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for containers
	mockServer.SetHandler("GET", "/containers", func(w http.ResponseWriter, r *http.Request) {
		containers := []testutil.Container{
			{
				ID:     "container_123",
				Name:   "Test Container",
				Image:  "nginx:latest",
				Status: "running",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(containers)
	})

	t.Run("get all containers", func(t *testing.T) {
		ctx := context.Background()
		containers, err := client.Containers.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(containers) != 1 {
			t.Errorf("expected 1 container, got %d", len(containers))
		}

		container := containers[0]
		if container.ID != "container_123" {
			t.Errorf("expected container ID 'container_123', got '%s'", container.ID)
		}

		if container.Name != "Test Container" {
			t.Errorf("expected container name 'Test Container', got '%s'", container.Name)
		}
	})
}

func TestContainerService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock response for container creation
	mockServer.SetHandler("POST", "/containers", func(w http.ResponseWriter, r *http.Request) {
		var req CreateContainerRequest
		json.NewDecoder(r.Body).Decode(&req)

		container := testutil.Container{
			ID:          "container_new_123",
			Name:        req.Name,
			Image:       req.Image,
			Status:      "creating",
			Environment: req.Environment,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(container)
	})

	t.Run("create container", func(t *testing.T) {
		req := CreateContainerRequest{
			Name:  "my-container",
			Image: "python:3.9",
			Environment: map[string]string{
				"API_KEY": "secret",
				"DEBUG":   "true",
			},
		}

		ctx := context.Background()
		container, err := client.Containers.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if container == nil {
			t.Fatal("expected container, got nil")
		}

		if container.Name != req.Name {
			t.Errorf("expected container name '%s', got '%s'", req.Name, container.Name)
		}

		if container.Image != req.Image {
			t.Errorf("expected container image '%s', got '%s'", req.Image, container.Image)
		}

		if len(container.Environment) != 2 {
			t.Errorf("expected 2 environment variables, got %d", len(container.Environment))
		}
	})
}
