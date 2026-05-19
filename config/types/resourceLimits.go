package config

type ResourceLimits struct {
	CPUCores   int `yaml:"cpu_cores" json:"cpu_cores"`
	MemoryMB   int `yaml:"memory_mb" json:"memory_mb"`
	HeapSizeMB int `yaml:"heap_size_mb" json:"heap_size_mb"`
}
