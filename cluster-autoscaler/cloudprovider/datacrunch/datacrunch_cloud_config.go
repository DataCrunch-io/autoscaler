package datacrunch

import apiv1 "k8s.io/api/core/v1"

// default look for gpu or cpu,
// custom groupname can override the default
type cloudConfig struct {
	ImageList     map[string]string // list of images to use for the cluster
	NodeConfigs   map[string]*nodeConfig
	BillingConfig billingConfig
}

type billingConfig struct {
	Contract  string // LONG_TERM, PAY_AS_YOU_GO or SPOT
	PriceType string //DYNAMIC_PRICE or FIXED_PRICE
}

// Autoscaler Node Config
type nodeConfig struct {
	IsSpot        bool // default is false
	Image         string
	StartupScript string // base64 encoded
	SSHKeyIDs     []string
	OSVolumeSize  int // in GB
	Labels        map[string]string
	Volumes       []additionalVolume // optional
	Taints        []apiv1.Taint      // optional
}

type additionalVolume struct {
	Name string
	Size int    // in GB
	Type string // HDD or SSD
}

func (cfg *cloudConfig) isValid() bool {
	if cfg.ImageList == nil {
		return false
	}

	if cfg.NodeConfigs == nil {
		return false
	}

	for _, nodeConfig := range cfg.NodeConfigs {
		if nodeConfig.Image == "" {
			return false
		}
	}

	return true
}
