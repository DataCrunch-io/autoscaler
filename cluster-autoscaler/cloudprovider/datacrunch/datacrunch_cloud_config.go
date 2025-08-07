package datacrunch

import (
	apiv1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
)

// cloudConfig represents the global configuration for DataCrunch autoscaler
type cloudConfig struct {
	Image             imageConfig                `json:"image"`
	SSHKeyIDs         []string                   `json:"sshKeyIDs"`
	BillingConfig     billingConfig              `json:"billingConfig"`
	Labels            labelConfig                `json:"labels"`
	StartupScript     string                     `json:"startingScript"`     // base64 encoded, required
	AdditionalVolumes []additionalVolume         `json:"additionalVolumes"`  // optional
	Taints            []apiv1.Taint             `json:"taints"`            // optional
}

// imageConfig holds GPU and CPU specific images
type imageConfig struct {
	GPU string `json:"gpu"`
	CPU string `json:"cpu"`
}

// labelConfig holds GPU and CPU specific labels
type labelConfig struct {
	GPU map[string]string `json:"gpu"`
	CPU map[string]string `json:"cpu"`
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
	Labels        map[string]string  // optional override for labels
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

	// Validate billing config
	if cfg.BillingConfig.Contract == "" {
		klog.Errorf("BillingConfig.Contract is required")
		return false
	}
	if cfg.BillingConfig.Price == "" {
		klog.Errorf("BillingConfig.Price is required")
		return false
	}

	// Validate startup script is provided
	if cfg.StartupScript == "" {
		klog.Errorf("StartupScript is required")
		return false
	}

	// Validate labels config - at least one set should be provided
	if len(cfg.Labels.GPU) == 0 && len(cfg.Labels.CPU) == 0 {
		klog.Errorf("At least one label set (GPU or CPU) must be specified")
		return false
	}

	return true
}
