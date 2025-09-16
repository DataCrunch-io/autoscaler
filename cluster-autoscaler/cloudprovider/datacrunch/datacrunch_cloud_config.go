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
	apiv1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
)

// cloudConfig represents the global configuration for DataCrunch autoscaler
type cloudConfig struct {
	Image              imageConfig        `json:"image"`
	SSHKeyIDs          []string           `json:"sshKeyIDs"`
	BillingConfig      billingConfig      `json:"billingConfig"`
	Labels             []string           `json:"labels"`
	Debug              bool               `json:"debug"`
	AvailableLocations []string           `json:"availableLocations"`
	StartupScript      string             `json:"startupScript"`     // base64 encoded, required
	StartupScriptEnv   map[string]string  `json:"startupScriptEnv"`  // all key will be capitalized and the value will be string
	AdditionalVolumes  []additionalVolume `json:"additionalVolumes"` // optional
	Taints             []apiv1.Taint      `json:"taints"`            // optional
}

// imageConfig holds GPU and CPU specific images
type imageConfig struct {
	GPU string `json:"gpu"`
	CPU string `json:"cpu"`
}

// billingConfig holds billing-related configuration
type billingConfig struct {
	Price    string `json:"price"`    // DYNAMIC_PRICE or FIXED_PRICE
	Contract string `json:"contract"` // LONG_TERM, PAY_AS_YOU_GO, or SPOT
}

// nodeConfig represents per-ASG configuration (if needed for overrides)
type nodeConfig struct {
	IsSpot        bool               // default is false
	Image         string             // optional override for specific image
	StartupScript string             // optional override for startup script
	SSHKeyIDs     []string           // optional override for SSH keys
	OSVolumeSize  int                // in GB, default is 100GB
	Labels        []string           // optional override for labels
	Volumes       []additionalVolume // optional additional volumes
	Taints        []apiv1.Taint      // optional taints
	Contract      string             // optional override for contract
	Price         string             // optional override for price
}

type additionalVolume struct {
	Name string
	Size int    // in GB
	Type string // HDD, NVMe; default is NVMe
}

func (cfg *cloudConfig) isValid() bool {
	// Validate image config
	if cfg.Image.GPU == "" && cfg.Image.CPU == "" {
		klog.Errorf("At least one image (GPU or CPU) must be specified")
		return false
	}

	// Validate SSH keys are provided
	if len(cfg.SSHKeyIDs) == 0 {
		klog.Errorf("SSHKeyIDs is required")
		return false
	}

	// Validate startup script is provided
	if cfg.StartupScript == "" {
		klog.Errorf("StartupScript is required")
		return false
	}

	// Validate available locations
	if len(cfg.AvailableLocations) == 0 {
		klog.Errorf("AvailableLocations is required")
		return false
	}

	return true
}
