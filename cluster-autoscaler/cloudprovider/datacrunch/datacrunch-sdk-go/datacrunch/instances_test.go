package datacrunch

import (
	"context"
	"net/http"
	"testing"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/testutil"
)

func TestInstanceService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all instances", func(t *testing.T) {
		ctx := context.Background()
		instances, err := client.Instances.Get(ctx, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}

		instance := instances[0]
		if instance.ID != "inst_123" {
			t.Errorf("expected instance ID 'inst_123', got '%s'", instance.ID)
		}

		if instance.InstanceType != "1V100.6V" {
			t.Errorf("expected instance type '1V100.6V', got '%s'", instance.InstanceType)
		}

		if instance.Status != StatusRunning {
			t.Errorf("expected status '%s', got '%s'", StatusRunning, instance.Status)
		}
	})

	t.Run("get instances with status filter", func(t *testing.T) {
		ctx := context.Background()
		instances, err := client.Instances.Get(ctx, StatusRunning)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}
	})
}

func TestInstanceService_GetByID(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get instance by ID", func(t *testing.T) {
		ctx := context.Background()
		instanceID := "test_instance_123"
		instance, err := client.Instances.GetByID(ctx, instanceID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.ID != instanceID {
			t.Errorf("expected instance ID '%s', got '%s'", instanceID, instance.ID)
		}

		if instance.InstanceType != "1V100.6V" {
			t.Errorf("expected instance type '1V100.6V', got '%s'", instance.InstanceType)
		}
	})
}

func TestInstanceService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("create instance with minimal config", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			Hostname:     "test-instance",
			Description:  "Test instance",
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.ID != "inst_new_123" {
			t.Errorf("expected instance ID 'inst_new_123', got '%s'", instance.ID)
		}

		if instance.InstanceType != req.InstanceType {
			t.Errorf("expected instance type '%s', got '%s'", req.InstanceType, instance.InstanceType)
		}

		if instance.Hostname != req.Hostname {
			t.Errorf("expected hostname '%s', got '%s'", req.Hostname, instance.Hostname)
		}

		// Should set default location
		if instance.Location != LocationFIN01 {
			t.Errorf("expected location '%s', got '%s'", LocationFIN01, instance.Location)
		}
	})

	t.Run("create instance with full config", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType:    "8V100.48V",
			Image:           "custom-image",
			Hostname:        "ml-server",
			Description:     "ML training server",
			SSHKeyIDs:       []string{"key_123", "key_456"},
			Location:        "US-01",
			StartupScriptID: stringPtr("script_123"),
			Volumes: []VolumeCreateRequest{
				{Size: 500, Type: "SSD", Name: "data"},
			},
			ExistingVolumes: []string{"vol_123"},
			OSVolume:        &OSVolumeCreateRequest{Size: 100, Type: "SSD"},
			IsSpot:          true,
			Coupon:          stringPtr("DISCOUNT20"),
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.Location != "US-01" {
			t.Errorf("expected location 'US-01', got '%s'", instance.Location)
		}

		if len(instance.SSHKeyIDs) != 2 {
			t.Errorf("expected 2 SSH keys, got %d", len(instance.SSHKeyIDs))
		}
	})
}

func TestInstanceService_Action(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("action on single instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Action(ctx, "inst_123", ActionShutdown, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("action on multiple instances", func(t *testing.T) {
		ctx := context.Background()
		instanceIDs := []string{"inst_123", "inst_456"}
		err := client.Instances.Action(ctx, instanceIDs, ActionDelete, []string{"vol_123"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestInstanceService_ConvenienceMethods(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("delete instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Delete(ctx, "inst_123", []string{"vol_123"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("shutdown instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Shutdown(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("hibernate instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Hibernate(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestInstanceService_IsAvailable(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock to return availability
	mockServer.SetHandler("GET", "/instance-availability/1V100.6V", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`true`))
	})

	t.Run("check instance availability", func(t *testing.T) {
		ctx := context.Background()
		available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !available {
			t.Error("expected instance type to be available")
		}
	})

	t.Run("check spot instance availability", func(t *testing.T) {
		ctx := context.Background()
		available, err := client.Instances.IsAvailable(ctx, "1V100.6V", true, "FIN-01")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !available {
			t.Error("expected spot instance type to be available")
		}
	})
}

func TestInstanceService_GetAvailabilities(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	// Set up mock to return availabilities
	mockServer.SetHandler("GET", "/instance-availability", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"instance_type": "1V100.6V",
				"location": "FIN-01",
				"available": true,
				"is_spot": false
			},
			{
				"instance_type": "8V100.48V",
				"location": "FIN-01",
				"available": false,
				"is_spot": true
			}
		]`))
	})

	t.Run("get all availabilities", func(t *testing.T) {
		ctx := context.Background()
		availabilities, err := client.Instances.GetAvailabilities(ctx, nil, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(availabilities) != 2 {
			t.Errorf("expected 2 availabilities, got %d", len(availabilities))
		}

		if availabilities[0].InstanceType != "1V100.6V" {
			t.Errorf("expected first instance type '1V100.6V', got '%s'", availabilities[0].InstanceType)
		}

		if availabilities[0].Available != true {
			t.Error("expected first instance to be available")
		}

		if availabilities[1].Available != false {
			t.Error("expected second instance to be unavailable")
		}
	})

	t.Run("get spot availabilities", func(t *testing.T) {
		ctx := context.Background()
		isSpot := true
		availabilities, err := client.Instances.GetAvailabilities(ctx, &isSpot, "FIN-01")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(availabilities) != 2 {
			t.Errorf("expected 2 availabilities, got %d", len(availabilities))
		}
	})
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}
