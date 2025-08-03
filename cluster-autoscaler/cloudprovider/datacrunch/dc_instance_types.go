package datacrunch

// DataCrunch instance type definitions
// These constants define the available instance types in DataCrunch

const (
	// GPU Instance Types
	InstanceType1L40S20V  = "1L40S.20V"  // 1x L40S GPU, 20 vCPUs
	InstanceType4L40S48V  = "4L40S.48V"  // 4x L40S GPU, 48 vCPUs
	InstanceType1H10020V  = "1H100.20V"  // 1x H100 GPU, 20 vCPUs
	InstanceType8H100240V = "8H100.240V" // 8x H100 GPU, 240 vCPUs
	
	// CPU Instance Types
	InstanceTypeCPU4V16G  = "CPU.4V.16G"  // 4 vCPUs, 16GB RAM
	InstanceTypeCPU8V32G  = "CPU.8V.32G"  // 8 vCPUs, 32GB RAM
	InstanceTypeCPU16V64G = "CPU.16V.64G" // 16 vCPUs, 64GB RAM
)

// InstanceTypeInfo provides metadata about instance types
type InstanceTypeInfo struct {
	ID          string
	Name        string
	CPUs        int
	MemoryGB    int
	GPUCount    int
	GPUType     string
	IsGPU       bool
	StorageGB   int
	Description string
}

// GetInstanceTypeInfo returns information about a specific instance type
func GetInstanceTypeInfo(instanceType string) *InstanceTypeInfo {
	instanceTypes := map[string]*InstanceTypeInfo{
		InstanceType1L40S20V: {
			ID:          InstanceType1L40S20V,
			Name:        "1x L40S GPU Instance",
			CPUs:        20,
			MemoryGB:    80,
			GPUCount:    1,
			GPUType:     "L40S",
			IsGPU:       true,
			StorageGB:   1000,
			Description: "Single L40S GPU instance for inference and training",
		},
		InstanceType4L40S48V: {
			ID:          InstanceType4L40S48V,
			Name:        "4x L40S GPU Instance",
			CPUs:        48,
			MemoryGB:    192,
			GPUCount:    4,
			GPUType:     "L40S",
			IsGPU:       true,
			StorageGB:   2000,
			Description: "Quad L40S GPU instance for large model training",
		},
		InstanceType1H10020V: {
			ID:          InstanceType1H10020V,
			Name:        "1x H100 GPU Instance",
			CPUs:        20,
			MemoryGB:    80,
			GPUCount:    1,
			GPUType:     "H100",
			IsGPU:       true,
			StorageGB:   1000,
			Description: "Single H100 GPU instance for high-performance training",
		},
		InstanceType8H100240V: {
			ID:          InstanceType8H100240V,
			Name:        "8x H100 GPU Instance",
			CPUs:        240,
			MemoryGB:    1920,
			GPUCount:    8,
			GPUType:     "H100",
			IsGPU:       true,
			StorageGB:   8000,
			Description: "Octa H100 GPU instance for massive model training",
		},
		InstanceTypeCPU4V16G: {
			ID:          InstanceTypeCPU4V16G,
			Name:        "CPU 4 vCPU 16GB",
			CPUs:        4,
			MemoryGB:    16,
			GPUCount:    0,
			GPUType:     "",
			IsGPU:       false,
			StorageGB:   100,
			Description: "Small CPU instance for general workloads",
		},
		InstanceTypeCPU8V32G: {
			ID:          InstanceTypeCPU8V32G,
			Name:        "CPU 8 vCPU 32GB",
			CPUs:        8,
			MemoryGB:    32,
			GPUCount:    0,
			GPUType:     "",
			IsGPU:       false,
			StorageGB:   200,
			Description: "Medium CPU instance for compute workloads",
		},
		InstanceTypeCPU16V64G: {
			ID:          InstanceTypeCPU16V64G,
			Name:        "CPU 16 vCPU 64GB",
			CPUs:        16,
			MemoryGB:    64,
			GPUCount:    0,
			GPUType:     "",
			IsGPU:       false,
			StorageGB:   400,
			Description: "Large CPU instance for intensive compute workloads",
		},
	}

	return instanceTypes[instanceType]
}

// GetAllInstanceTypes returns information about all available instance types
func GetAllInstanceTypes() map[string]*InstanceTypeInfo {
	result := make(map[string]*InstanceTypeInfo)
	
	instanceTypes := []string{
		InstanceType1L40S20V,
		InstanceType4L40S48V,
		InstanceType1H10020V,
		InstanceType8H100240V,
		InstanceTypeCPU4V16G,
		InstanceTypeCPU8V32G,
		InstanceTypeCPU16V64G,
	}
	
	for _, instanceType := range instanceTypes {
		if info := GetInstanceTypeInfo(instanceType); info != nil {
			result[instanceType] = info
		}
	}
	
	return result
}

// IsGPUInstanceType returns true if the instance type has GPUs
func IsGPUInstanceType(instanceType string) bool {
	info := GetInstanceTypeInfo(instanceType)
	return info != nil && info.IsGPU
}

// GetGPUCount returns the number of GPUs for an instance type
func GetGPUCount(instanceType string) int {
	info := GetInstanceTypeInfo(instanceType)
	if info != nil {
		return info.GPUCount
	}
	return 0
}