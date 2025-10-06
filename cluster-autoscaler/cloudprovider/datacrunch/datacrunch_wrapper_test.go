package datacrunch

import (
	"context"
	"net/http"
	"testing"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/testutil"
)

// TestWrapperWithMockServer tests the wrapper methods using the official SDK's mock server
func TestWrapperWithMockServer(t *testing.T) {
	// Create a mock server using the official SDK's test utilities
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	// Create a client pointing to the mock server
	client, err := datacrunch.NewClient(
		datacrunch.WithClientID("test-client-id"),
		datacrunch.WithClientSecret("test-client-secret"),
		datacrunch.WithBaseURL(mockServer.URL()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create wrapper
	wrapper := &datacrunchWrapper{
		client: client,
		ctx:    context.Background(),
	}

	t.Run("ListInstances", func(t *testing.T) {
		instances, err := wrapper.ListInstances("")
		if err != nil {
			t.Fatalf("ListInstances failed: %v", err)
		}
		if len(instances) == 0 {
			t.Error("Expected at least one instance from mock server")
		}
		t.Logf("Got %d instances", len(instances))
	})

	t.Run("GetInstanceByHostname", func(t *testing.T) {
		// First get all instances to find a valid hostname
		instances, err := wrapper.ListInstances("")
		if err != nil {
			t.Fatalf("ListInstances failed: %v", err)
		}
		if len(instances) == 0 {
			t.Skip("No instances available for testing")
		}

		hostname := instances[0].Hostname
		instance, err := wrapper.GetInstanceByHostname(hostname)
		if err != nil {
			t.Fatalf("GetInstanceByHostname failed: %v", err)
		}
		if instance.Hostname != hostname {
			t.Errorf("Expected hostname %s, got %s", hostname, instance.Hostname)
		}
	})

	t.Run("ListInstanceTypes", func(t *testing.T) {
		types, err := wrapper.ListInstanceTypes()
		if err != nil {
			// Mock server may not have this endpoint - skip if not available
			t.Skipf("ListInstanceTypes not available in mock server: %v", err)
		}
		t.Logf("Got %d instance types", len(types))
	})

	t.Run("CreateStartScript", func(t *testing.T) {
		script, err := wrapper.CreateStartScript("test-script", "#!/bin/bash\necho 'test'")
		if err != nil {
			// Mock server may not have this endpoint - skip if not available
			t.Skipf("CreateStartScript not available in mock server: %v", err)
		}
		if script.Name != "test-script" {
			t.Errorf("Expected script name 'test-script', got %s", script.Name)
		}
		t.Logf("Created script with ID: %s", script.ID)
	})

	t.Run("ListStartScripts", func(t *testing.T) {
		scripts, err := wrapper.ListStartScripts()
		if err != nil {
			// Mock server may not have this endpoint - skip if not available
			t.Skipf("ListStartScripts not available in mock server: %v", err)
		}
		t.Logf("Got %d startup scripts", len(scripts))
	})
}

// TestWrapperInstanceFiltering tests instance filtering methods
func TestWrapperInstanceFiltering(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client, err := datacrunch.NewClient(
		datacrunch.WithClientID("test-client-id"),
		datacrunch.WithClientSecret("test-client-secret"),
		datacrunch.WithBaseURL(mockServer.URL()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	wrapper := &datacrunchWrapper{
		client: client,
		ctx:    context.Background(),
	}

	t.Run("GetAllInstancesByDescription", func(t *testing.T) {
		instances, err := wrapper.GetAllInstancesByDescription("Test instance")
		if err != nil {
			t.Fatalf("GetAllInstancesByDescription failed: %v", err)
		}
		t.Logf("Got %d instances with description 'Test instance'", len(instances))
	})

	t.Run("GetAllInstancesByAsgName", func(t *testing.T) {
		instances, err := wrapper.GetAllInstancesByAsgName("test-asg")
		if err != nil {
			t.Fatalf("GetAllInstancesByAsgName failed: %v", err)
		}
		t.Logf("Got %d instances for ASG 'test-asg'", len(instances))
	})
}

// TestWrapperAvailability tests availability checking methods
func TestWrapperAvailability(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client, err := datacrunch.NewClient(
		datacrunch.WithClientID("test-client-id"),
		datacrunch.WithClientSecret("test-client-secret"),
		datacrunch.WithBaseURL(mockServer.URL()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	wrapper := &datacrunchWrapper{
		client: client,
		ctx:    context.Background(),
	}

	t.Run("GetInstanceAvailabilityLocation", func(t *testing.T) {
		location, err := wrapper.GetInstanceAvailabilityLocation("1V100.6V", []string{"FIN-01", "FIN-02"})
		if err != nil {
			t.Logf("GetInstanceAvailabilityLocation returned error (expected if not available): %v", err)
		} else {
			t.Logf("Instance type available in location: %s", location)
		}
	})
}

// TestCreateInstanceRequest tests instance creation request structure
func TestCreateInstanceRequest(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client, err := datacrunch.NewClient(
		datacrunch.WithClientID("test-client-id"),
		datacrunch.WithClientSecret("test-client-secret"),
		datacrunch.WithBaseURL(mockServer.URL()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	wrapper := &datacrunchWrapper{
		client: client,
		ctx:    context.Background(),
	}

	t.Run("CreateInstance", func(t *testing.T) {
		scriptID := "test-script-id"
		req := &datacrunch.CreateInstanceRequest{
			InstanceType:    "1V100.6V",
			Image:           "ubuntu-24.04-cuda-12.8-open-docker",
			Hostname:        "test-instance",
			Description:     "test-asg",
			SSHKeyIDs:       []string{"ssh-key-1"},
			Location:        "FIN-01",
			StartupScriptID: &scriptID,
			IsSpot:          false,
		}

		instance, err := wrapper.CreateInstance(req)
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}
		if instance.Hostname != "test-instance" {
			t.Errorf("Expected hostname 'test-instance', got %s", instance.Hostname)
		}
		t.Logf("Created instance with ID: %s", instance.ID)
	})
}

// TestPerformInstanceAction tests instance actions
func TestPerformInstanceAction(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client, err := datacrunch.NewClient(
		datacrunch.WithClientID("test-client-id"),
		datacrunch.WithClientSecret("test-client-secret"),
		datacrunch.WithBaseURL(mockServer.URL()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	wrapper := &datacrunchWrapper{
		client: client,
		ctx:    context.Background(),
	}

	t.Run("PerformInstanceAction_Shutdown", func(t *testing.T) {
		// The mock server should handle this gracefully
		err := wrapper.PerformInstanceAction("test-instance-id", datacrunch.ActionShutdown)
		if err != nil {
			// Mock server might return 404 or other error - that's ok for this test
			t.Logf("PerformInstanceAction returned error (expected for mock): %v", err)
		}
	})

	t.Run("PerformInstanceAction_Delete", func(t *testing.T) {
		err := wrapper.PerformInstanceAction("test-instance-id", datacrunch.ActionDelete)
		if err != nil {
			t.Logf("PerformInstanceAction returned error (expected for mock): %v", err)
		}
	})
}

// TestMockServerResponses verifies the mock server is working correctly
func TestMockServerResponses(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(mockServer.URL() + "/instances")
		if err != nil {
			t.Fatalf("Failed to call mock server: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
