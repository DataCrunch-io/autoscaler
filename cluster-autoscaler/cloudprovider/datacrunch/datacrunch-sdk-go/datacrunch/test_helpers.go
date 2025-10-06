package datacrunch

import "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/testutil"

// NewTestClient creates a test client using the testutil configuration approach
// This is the standard way to create test clients for unit tests
//
// Example usage:
//
//	mockServer := testutil.NewMockServer()
//	defer mockServer.Close()
//	client := NewTestClient(mockServer)
func NewTestClient(mockServer *testutil.MockServer) *Client {
	config := testutil.NewTestClientConfig(mockServer)
	client, _ := NewClient(
		WithBaseURL(config.BaseURL),
		WithClientID(config.ClientID),
		WithClientSecret(config.ClientSecret),
	)
	return client
}
