/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datacrunch

import (
	"os"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch"
)

type datacrunchSDKProvider struct {
	client *datacrunch.Client
}

func createDatacrunchSDKProvider(cfg *cloudConfig) (*datacrunchSDKProvider, error) {
	// Get credentials from environment variables
	clientID := os.Getenv("DATACRUNCH_CLIENT_ID")
	clientSecret := os.Getenv("DATACRUNCH_CLIENT_SECRET")

	// Create client with options
	client, err := datacrunch.NewClient(
		datacrunch.WithClientID(clientID),
		datacrunch.WithClientSecret(clientSecret),
		datacrunch.WithDebugLogging(cfg.Debug),
	)
	if err != nil {
		return nil, err
	}

	return &datacrunchSDKProvider{
		client: client,
	}, nil
}
